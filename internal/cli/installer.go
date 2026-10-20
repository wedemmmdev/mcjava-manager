package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

const maxDependencyDepth = 24

type projectKey struct {
	provider  modsource.Provider
	projectID string
}

type installOutcome struct {
	Entry                manifest.Entry
	Path                 string
	Dependency           bool
	Skipped              bool
	PreviousVersionID    string
	PreviousVersionLabel string
}

type installPlan struct {
	entry      manifest.Entry
	file       modsource.File
	directory  string
	safeName   string
	targetPath string
	overwrite  bool
	skipped    bool
	existing   *manifest.Entry
	oldEntries []manifest.Entry
	dependency bool
}

type contentInstaller struct {
	client                     modsource.Client
	downloader                 *download.Client
	store                      *manifest.Store
	inst                       *instance.Instance
	filter                     modsource.VersionFilter
	channelExplicit            bool
	overwrite                  bool
	allowDependencyReplacement bool
	resolveDependencies        bool
	errOut                     io.Writer
	active                     map[projectKey]string
}

func newContentInstaller(
	client modsource.Client,
	downloader *download.Client,
	store *manifest.Store,
	inst *instance.Instance,
	filter modsource.VersionFilter,
	channelExplicit bool,
	overwrite bool,
	allowDependencyReplacement bool,
	resolveDependencies bool,
	errOut io.Writer,
) *contentInstaller {
	return &contentInstaller{
		client:                     client,
		downloader:                 downloader,
		store:                      store,
		inst:                       inst,
		filter:                     filter,
		channelExplicit:            channelExplicit,
		overwrite:                  overwrite,
		allowDependencyReplacement: allowDependencyReplacement,
		resolveDependencies:        resolveDependencies,
		errOut:                     errOut,
		active:                     make(map[projectKey]string),
	}
}

func (i *contentInstaller) InstallProject(ctx context.Context, projectRef, versionID string) ([]installOutcome, error) {
	project, err := i.client.GetProject(ctx, projectRef)
	if err != nil {
		return nil, fmt.Errorf("falha ao obter o projeto %q: %w", projectRef, err)
	}
	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return nil, fmt.Errorf("o provedor não retornou o ID do projeto %q", projectRef)
	}

	if _, err := installDirectoryName(project.Type); err != nil {
		return nil, fmt.Errorf("projeto %q: %w", projectDisplayName(project), err)
	}

	version, err := selectInstallVersion(
		ctx,
		i.client,
		project,
		&installOptions{versionID: versionID},
		i.filter,
		i.channelExplicit,
	)
	if err != nil {
		return nil, err
	}

	return i.installResolved(ctx, project, version, false, 0)
}

func (i *contentInstaller) installResolved(
	ctx context.Context,
	project modsource.Project,
	version modsource.Version,
	dependency bool,
	depth int,
) ([]installOutcome, error) {
	if depth > maxDependencyDepth {
		return nil, fmt.Errorf(
			"a árvore de dependências de %q excedeu o limite de %d níveis",
			projectDisplayName(project),
			maxDependencyDepth,
		)
	}

	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return nil, errors.New("o provedor não retornou o ID de um projeto necessário")
	}
	version.ID = strings.TrimSpace(version.ID)
	if version.ID == "" {
		return nil, fmt.Errorf("o provedor não retornou o ID da versão de %q", projectDisplayName(project))
	}
	version.ProjectID = strings.TrimSpace(version.ProjectID)
	if version.ProjectID == "" {
		version.ProjectID = project.ID
	}
	if version.ProjectID != project.ID {
		return nil, fmt.Errorf(
			"a versão %q não pertence ao projeto %q",
			versionDisplayLabel(version),
			projectDisplayName(project),
		)
	}

	provider := i.client.Provider()
	key := projectKey{provider: provider, projectID: project.ID}
	if activeVersion, ok := i.active[key]; ok {
		if activeVersion == version.ID {
			// A required dependency cycle is harmless: the project is already
			// being installed higher in the tree.
			return nil, nil
		}

		return nil, fmt.Errorf(
			"conflito de dependência: %q já está sendo instalado na versão %q, mas a versão %q foi exigida",
			projectDisplayName(project),
			activeVersion,
			versionDisplayLabel(version),
		)
	}
	i.active[key] = version.ID
	defer delete(i.active, key)

	plan, err := i.planInstall(project, version, dependency)
	if err != nil {
		return nil, err
	}

	var outcomes []installOutcome
	if i.resolveDependencies {
		for _, dependencyRef := range version.Dependencies {
			if dependencyRef.Kind != modsource.DependencyRequired {
				continue
			}

			depProject, depVersion, err := i.resolveDependency(ctx, dependencyRef, project, version)
			if err != nil {
				return outcomes, fmt.Errorf(
					"falha ao resolver a dependência obrigatória de %q (%s): %w",
					projectDisplayName(project),
					versionDisplayLabel(version),
					err,
				)
			}

			depOutcomes, err := i.installResolved(ctx, depProject, depVersion, true, depth+1)
			outcomes = append(outcomes, depOutcomes...)
			if err != nil {
				return outcomes, fmt.Errorf(
					"falha ao instalar a dependência obrigatória %q de %q: %w",
					projectDisplayName(depProject),
					projectDisplayName(project),
					err,
				)
			}
		}
	}

	if plan.skipped {
		outcomes = append(outcomes, outcomeFromPlan(plan, true))
		return outcomes, nil
	}

	outcome, err := i.downloadAndRecord(ctx, plan)
	if outcome.Entry.FilePath != "" && outcome.Path != "" {
		outcomes = append(outcomes, outcome)
	}

	return outcomes, err
}

func (i *contentInstaller) planInstall(
	project modsource.Project,
	version modsource.Version,
	dependency bool,
) (installPlan, error) {
	directoryName, err := installDirectoryName(project.Type)
	if err != nil {
		return installPlan{}, fmt.Errorf("projeto %q: %w", projectDisplayName(project), err)
	}

	file, err := version.PrimaryFile()
	if err != nil {
		return installPlan{}, fmt.Errorf(
			"a versão %q de %q não possui um arquivo instalável: %w",
			versionDisplayLabel(version),
			projectDisplayName(project),
			err,
		)
	}
	safeName, err := download.SafeFilename(file.Name)
	if err != nil {
		return installPlan{}, fmt.Errorf(
			"o provedor retornou um nome de arquivo inválido para %q: %w",
			projectDisplayName(project),
			err,
		)
	}

	relativePath, err := manifest.RelativePath(project.Type, safeName)
	if err != nil {
		return installPlan{}, err
	}
	targetDirectory := filepath.Join(i.inst.Path, directoryName)
	targetPath := filepath.Join(targetDirectory, safeName)

	provider := i.client.Provider()
	existingEntries := i.store.FindByProject(provider, project.ID)
	if len(existingEntries) > 1 {
		return installPlan{}, fmt.Errorf(
			"o manifesto contém mais de uma versão instalada de %q; remova as entradas duplicadas antes de continuar",
			projectDisplayName(project),
		)
	}

	var existing *manifest.Entry
	if len(existingEntries) == 1 {
		copy := existingEntries[0]
		existing = &copy
	}
	if existing != nil && existing.VersionID != version.ID && dependency && !i.allowDependencyReplacement {
		return installPlan{}, fmt.Errorf(
			"a dependência %q já está instalada na versão %q, mas a versão %q é necessária; remova ou atualize a dependência antes de continuar",
			projectDisplayName(project),
			existing.VersionLabel(),
			versionDisplayLabel(version),
		)
	}

	if other, ok := i.store.FindByPath(relativePath); ok &&
		(other.Provider != provider || other.ProjectID != project.ID) {
		return installPlan{}, fmt.Errorf(
			"o arquivo %q já está registrado para %q",
			relativePath,
			other.ProjectLabel(),
		)
	}

	overwrite := i.overwrite
	if existing != nil && manifest.PathsEqual(existing.FilePath, relativePath) {
		overwrite = true
	}

	skipped := false
	if existing != nil && existing.VersionID == version.ID &&
		manifest.PathsEqual(existing.FilePath, relativePath) {
		info, statErr := os.Stat(targetPath)
		switch {
		case statErr == nil:
			if info.IsDir() {
				return installPlan{}, fmt.Errorf("o destino da instalação %q é um diretório", targetPath)
			}
			if !i.overwrite {
				skipped = true
			}
		case errors.Is(statErr, fs.ErrNotExist):
			// The file disappeared: reinstall it even though the manifest
			// already points to this version.
		default:
			return installPlan{}, instance.WrapFSError("verificar o arquivo instalado", targetPath, statErr)
		}
	}

	if !skipped {
		info, statErr := os.Stat(targetPath)
		switch {
		case statErr == nil:
			if info.IsDir() {
				return installPlan{}, fmt.Errorf("o destino da instalação %q é um diretório", targetPath)
			}
			if !overwrite {
				return installPlan{}, wrapInstallDownloadError(
					fmt.Errorf("%w: %q", download.ErrTargetExists, targetPath),
					safeName,
					targetDirectory,
				)
			}
		case errors.Is(statErr, fs.ErrNotExist):
		default:
			return installPlan{}, instance.WrapFSError("verificar o destino da instalação", targetPath, statErr)
		}
	}

	now := time.Now().UTC()
	entry := manifest.Entry{
		Provider:      provider,
		ProjectID:     project.ID,
		ProjectSlug:   project.Slug,
		ProjectName:   project.Name,
		Type:          project.Type,
		VersionID:     version.ID,
		VersionNumber: version.VersionNumber,
		VersionName:   version.Name,
		Channel:       version.Channel,
		GameVersion:   firstValue(i.filter.GameVersion, version.GameVersions),
		Loader:        firstValue(i.filter.Loader, version.Loaders),
		GameVersions:  append([]string(nil), version.GameVersions...),
		Loaders:       append([]string(nil), version.Loaders...),
		FileName:      safeName,
		FilePath:      relativePath,
		FileSize:      file.Size,
		Hashes:        hashesMap(file.Hashes),
		InstalledAt:   now,
		UpdatedAt:     now,
	}
	if existing != nil {
		entry.InstalledAt = existing.InstalledAt
		if entry.InstalledAt.IsZero() {
			entry.InstalledAt = now
		}
	}

	return installPlan{
		entry:      entry,
		file:       file,
		directory:  targetDirectory,
		safeName:   safeName,
		targetPath: targetPath,
		overwrite:  overwrite,
		skipped:    skipped,
		existing:   existing,
		oldEntries: existingEntries,
		dependency: dependency,
	}, nil
}

func (i *contentInstaller) resolveDependency(
	ctx context.Context,
	dependency modsource.Dependency,
	parentProject modsource.Project,
	parentVersion modsource.Version,
) (modsource.Project, modsource.Version, error) {
	projectID := strings.TrimSpace(dependency.ProjectID)
	versionID := strings.TrimSpace(dependency.VersionID)

	var version modsource.Version
	if versionID != "" {
		fetched, err := i.client.GetVersion(ctx, versionID)
		if err != nil {
			return modsource.Project{}, modsource.Version{}, fmt.Errorf("falha ao obter a versão %q: %w", versionID, err)
		}
		version = fetched
		version.ProjectID = strings.TrimSpace(version.ProjectID)
		if projectID != "" && version.ProjectID != "" && version.ProjectID != projectID {
			return modsource.Project{}, modsource.Version{}, fmt.Errorf(
				"a versão %q pertence ao projeto %q, mas a dependência aponta para %q",
				versionID,
				version.ProjectID,
				projectID,
			)
		}
		if projectID == "" && version.ProjectID == "" {
			return modsource.Project{}, modsource.Version{}, fmt.Errorf(
				"a versão %q da dependência de %q não informa o projeto associado",
				versionID,
				projectDisplayName(parentProject),
			)
		}
		if version.ProjectID != "" {
			projectID = version.ProjectID
		}
	}

	if projectID == "" {
		return modsource.Project{}, modsource.Version{}, fmt.Errorf(
			"a dependência de %q (%s) não informa projeto nem versão",
			projectDisplayName(parentProject),
			versionDisplayLabel(parentVersion),
		)
	}

	project, err := i.client.GetProject(ctx, projectID)
	if err != nil {
		return modsource.Project{}, modsource.Version{}, fmt.Errorf("falha ao obter o projeto %q: %w", projectID, err)
	}
	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return modsource.Project{}, modsource.Version{}, fmt.Errorf("o provedor não retornou o ID da dependência %q", projectID)
	}

	if versionID == "" {
		versions, err := i.client.ListVersions(ctx, project.ID, i.filter)
		if err != nil {
			return modsource.Project{}, modsource.Version{}, fmt.Errorf(
				"falha ao listar as versões de %q: %w",
				projectDisplayName(project),
				err,
			)
		}
		selected, ok, err := newestDownloadableVersion(versions)
		if err != nil {
			return modsource.Project{}, modsource.Version{}, err
		}
		if !ok {
			return modsource.Project{}, modsource.Version{}, fmt.Errorf(
				"nenhuma versão instalável de %q foi encontrada com os filtros informados",
				projectDisplayName(project),
			)
		}
		version = selected
	}

	version.ProjectID = strings.TrimSpace(version.ProjectID)
	if version.ProjectID == "" {
		version.ProjectID = project.ID
	}
	if version.ProjectID != project.ID {
		return modsource.Project{}, modsource.Version{}, fmt.Errorf(
			"a versão %q não pertence ao projeto %q",
			versionDisplayLabel(version),
			projectDisplayName(project),
		)
	}
	if err := validateVersionCompatibility(version, i.filter); err != nil {
		return modsource.Project{}, modsource.Version{}, fmt.Errorf(
			"a dependência %q não é compatível: %w",
			projectDisplayName(project),
			err,
		)
	}

	return project, version, nil
}

func (i *contentInstaller) downloadAndRecord(ctx context.Context, plan installPlan) (installOutcome, error) {
	if err := i.inst.EnsureLayout(); err != nil {
		return installOutcome{}, err
	}

	downloader := i.downloader
	if downloader == nil {
		downloader = download.New(download.WithUserAgent(providerUserAgent()))
	}

	progress := newProgressPrinter(i.errOut)
	result, downloadErr := downloader.Download(ctx, download.Request{
		URL:            plan.file.URL,
		Target:         plan.targetPath,
		ExpectedHashes: plan.file.Hashes,
		Overwrite:      plan.overwrite,
		Progress:       progress.Update,
	})
	finishErr := progress.Finish(downloadErr == nil)
	if downloadErr != nil {
		wrapped := wrapInstallDownloadError(downloadErr, plan.safeName, plan.directory)
		if finishErr != nil {
			return installOutcome{}, errors.Join(wrapped, finishErr)
		}

		return installOutcome{}, wrapped
	}

	entry := plan.entry
	entry.FileSize = result.Bytes
	entry.Hashes = hashesFromDownload(result.Hashes)
	entry.UpdatedAt = time.Now().UTC()

	outcome := outcomeFromPlan(plan, false)
	outcome.Entry = entry
	outcome.Path = result.Path

	if err := i.store.ReplaceProject(entry); err != nil {
		return outcome, fmt.Errorf(
			"arquivo instalado em %q, mas falha ao registrar no manifesto: %w",
			result.Path,
			err,
		)
	}

	var cleanupErrs []error
	for _, old := range plan.oldEntries {
		if manifest.PathsEqual(old.FilePath, entry.FilePath) {
			continue
		}
		oldPath, err := old.TargetPath(i.inst.Path)
		if err != nil {
			cleanupErrs = append(cleanupErrs, err)
			continue
		}
		if err := removePreviousInstalledFile(oldPath); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	if len(cleanupErrs) > 0 {
		return outcome, errors.Join(cleanupErrs...)
	}

	if finishErr != nil {
		return outcome, fmt.Errorf("arquivo instalado em %q, mas falha ao finalizar o progresso: %w", result.Path, finishErr)
	}

	return outcome, nil
}

func removePreviousInstalledFile(pathValue string) error {
	info, err := os.Lstat(pathValue)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return instance.WrapFSError("inspecionar a versão anterior", pathValue, err)
	case info.IsDir():
		return fmt.Errorf(
			"o caminho %q é um diretório; a versão anterior não foi removida",
			pathValue,
		)
	case !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0:
		return fmt.Errorf(
			"o caminho %q não é um arquivo regular ou link simbólico; a versão anterior não foi removida",
			pathValue,
		)
	}

	if err := os.Remove(pathValue); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return instance.WrapFSError("remover a versão anterior", pathValue, err)
	}

	return nil
}

func outcomeFromPlan(plan installPlan, skipped bool) installOutcome {
	outcome := installOutcome{
		Entry:      plan.entry,
		Path:       plan.targetPath,
		Dependency: plan.dependency,
		Skipped:    skipped,
	}
	if plan.existing != nil {
		outcome.PreviousVersionID = plan.existing.VersionID
		outcome.PreviousVersionLabel = plan.existing.VersionLabel()
	}

	return outcome
}

func firstValue(preferred string, values []string) string {
	if trimmed := strings.TrimSpace(preferred); trimmed != "" {
		return trimmed
	}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func hashesMap(hashes []modsource.Hash) map[string]string {
	if len(hashes) == 0 {
		return nil
	}

	result := make(map[string]string, len(hashes))
	for _, hash := range hashes {
		algorithm := strings.TrimSpace(string(hash.Algorithm))
		value := strings.TrimSpace(hash.Value)
		if algorithm == "" || value == "" {
			continue
		}
		result[algorithm] = value
	}
	if len(result) == 0 {
		return nil
	}

	return result
}

func hashesFromDownload(hashes map[modsource.HashAlgorithm]string) map[string]string {
	if len(hashes) == 0 {
		return nil
	}

	result := make(map[string]string, len(hashes))
	for algorithm, value := range hashes {
		algorithmValue := strings.TrimSpace(string(algorithm))
		value = strings.TrimSpace(value)
		if algorithmValue == "" || value == "" {
			continue
		}
		result[algorithmValue] = value
	}
	if len(result) == 0 {
		return nil
	}

	return result
}

func writeInstallOutcomes(out io.Writer, outcomes []installOutcome, updateMode bool) error {
	for _, outcome := range outcomes {
		label := sanitizeText(outcome.Entry.ProjectLabel())
		version := sanitizeText(outcome.Entry.VersionLabel())
		target := sanitizeText(outcome.Path)
		dependency := ""
		if outcome.Dependency {
			dependency = "dependência "
		}

		var line string
		switch {
		case outcome.Skipped:
			switch {
			case outcome.Dependency:
				line = fmt.Sprintf("Dependência já instalada %s %s em %s", label, version, target)
			case updateMode:
				line = fmt.Sprintf("Já atualizado %s %s em %s", label, version, target)
			default:
				line = fmt.Sprintf("Já instalado %s %s em %s", label, version, target)
			}
		case outcome.PreviousVersionID != "" && outcome.PreviousVersionID != outcome.Entry.VersionID:
			transition := formatVersionTransition(sanitizeText(outcome.PreviousVersionLabel), version)
			line = fmt.Sprintf(
				"Atualizado %s%s %s em %s (checksum verificado)",
				dependency,
				label,
				transition,
				target,
			)
		case outcome.PreviousVersionID != "":
			line = fmt.Sprintf(
				"Reinstalado %s%s %s em %s (checksum verificado)",
				dependency,
				label,
				version,
				target,
			)
		default:
			line = fmt.Sprintf(
				"Instalado %s%s %s em %s (checksum verificado)",
				dependency,
				label,
				version,
				target,
			)
		}

		if _, err := fmt.Fprintln(out, line); err != nil {
			return fmt.Errorf("falha ao escrever o resultado da instalação: %w", err)
		}
	}

	return nil
}

func formatVersionTransition(previous, current string) string {
	previous = strings.TrimSpace(previous)
	if previous == "" {
		return current
	}

	return previous + " -> " + current
}
