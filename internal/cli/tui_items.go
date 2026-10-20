package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

var tuiSearchKindLabels = []string{
	"Todos",
	"Mod",
	"Shader",
	"Pacote de recursos",
}

var tuiSearchKinds = []modsource.ProjectKind{
	"",
	modsource.KindMod,
	modsource.KindShaderPack,
	modsource.KindResourcePack,
}

var tuiChannelLabels = []string{
	"release",
	"beta",
	"alpha",
	"todos",
}

func mergeTUIProjectFallback(details, fallback modsource.Project) modsource.Project {
	if details.ID == "" {
		details.ID = fallback.ID
	}
	if details.Slug == "" {
		details.Slug = fallback.Slug
	}
	if details.Name == "" {
		details.Name = fallback.Name
	}
	if details.Summary == "" {
		details.Summary = fallback.Summary
	}
	if details.Type == "" {
		details.Type = fallback.Type
	}
	if len(details.Authors) == 0 {
		details.Authors = append([]string(nil), fallback.Authors...)
	}
	if details.Downloads == 0 {
		details.Downloads = fallback.Downloads
	}
	if details.Provider == "" {
		details.Provider = fallback.Provider
	}

	return details
}

type tuiProjectItem struct {
	project modsource.Project
}

func (i tuiProjectItem) Title() string {
	title := projectDisplayName(i.project)
	if kind := displayProjectType(i.project.Type); kind != "-" {
		title += " · " + kind
	}

	return title
}

func (i tuiProjectItem) Description() string {
	parts := make([]string, 0, 4)

	if authors := authorsText(i.project.Authors); authors != "-" {
		parts = append(parts, authors)
	}
	if i.project.Downloads > 0 {
		parts = append(parts, formatCount(i.project.Downloads)+" downloads")
	}
	if loaders := summarizeValues(i.project.Loaders, 3); loaders != "-" {
		parts = append(parts, loaders)
	}
	if summary := truncate(i.project.Summary, 96); summary != "" {
		parts = append(parts, summary)
	}

	return strings.Join(parts, " • ")
}

func (i tuiProjectItem) FilterValue() string {
	return strings.Join([]string{
		projectDisplayName(i.project),
		projectIdentifier(i.project),
		authorsText(i.project.Authors),
		displayProjectType(i.project.Type),
	}, " ")
}

type tuiVersionItem struct {
	version modsource.Version
}

func (i tuiVersionItem) Title() string {
	title := versionDisplayLabel(i.version)
	if channel := displayChannel(i.version.Channel); channel != "-" {
		title += " · " + channel
	}

	return title
}

func (i tuiVersionItem) Description() string {
	parts := make([]string, 0, 4)

	if gameVersions := summarizeValues(i.version.GameVersions, 3); gameVersions != "-" {
		parts = append(parts, "jogo "+gameVersions)
	}
	if loaders := summarizeValues(i.version.Loaders, 3); loaders != "-" {
		parts = append(parts, "loader "+loaders)
	}
	if file, err := i.version.PrimaryFile(); err == nil {
		if name := truncate(file.Name, 48); name != "" {
			parts = append(parts, name)
		}
	} else {
		parts = append(parts, "sem arquivo instalável")
	}
	if !i.version.PublishedAt.IsZero() {
		parts = append(parts, i.version.PublishedAt.Format("2006-01-02"))
	}

	return strings.Join(parts, " • ")
}

func (i tuiVersionItem) FilterValue() string {
	return strings.Join([]string{
		versionDisplayLabel(i.version),
		i.version.ID,
		displayChannel(i.version.Channel),
		strings.Join(i.version.GameVersions, " "),
		strings.Join(i.version.Loaders, " "),
	}, " ")
}

type tuiInstalledItem struct {
	tracked   bool
	entry     manifest.Entry
	untracked untrackedFile
	status    string
}

func (i tuiInstalledItem) Title() string {
	if i.tracked {
		return i.entry.ProjectLabel() + " · " + i.entry.VersionLabel()
	}

	return i.untracked.rel + " · não rastreado"
}

func (i tuiInstalledItem) Description() string {
	if i.tracked {
		parts := []string{
			displayProjectType(i.entry.Type),
			sanitizeText(i.entry.FilePath),
			i.status,
		}
		if i.entry.FileSize > 0 {
			parts = append(parts, formatBytes(i.entry.FileSize))
		}
		if gameVersion := truncate(i.entry.GameVersion, 24); gameVersion != "" {
			parts = append(parts, "jogo "+gameVersion)
		}
		if loader := truncate(i.entry.Loader, 24); loader != "" {
			parts = append(parts, "loader "+loader)
		}

		return strings.Join(parts, " • ")
	}

	return strings.Join([]string{
		displayProjectType(i.untracked.kind),
		sanitizeText(i.untracked.rel),
		"não rastreado",
		formatBytes(i.untracked.size),
	}, " • ")
}

func (i tuiInstalledItem) FilterValue() string {
	if i.tracked {
		return strings.Join([]string{
			i.entry.ProjectLabel(),
			i.entry.ProjectSlug,
			i.entry.ProjectID,
			i.entry.VersionLabel(),
			i.entry.FileName,
			i.entry.FilePath,
			displayProjectType(i.entry.Type),
			i.status,
		}, " ")
	}

	return strings.Join([]string{
		i.untracked.rel,
		displayProjectType(i.untracked.kind),
		"não rastreado",
	}, " ")
}

func (i tuiInstalledItem) Key() string {
	if i.tracked {
		return i.entry.FilePath
	}

	return i.untracked.rel
}

func (i tuiInstalledItem) kind() modsource.ProjectKind {
	if i.tracked {
		return i.entry.Type
	}

	return i.untracked.kind
}

func buildTUIInstalledItems(inst *instance.Instance, store *manifest.Store) ([]tuiInstalledItem, error) {
	if store == nil {
		return nil, nil
	}

	entries := store.Entries()
	items := make([]tuiInstalledItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, tuiInstalledItem{
			tracked: true,
			entry:   entry,
			status:  manifestEntryStatus(inst, entry),
		})
	}

	untracked, err := scanUntrackedFiles(inst, store, "")
	for _, file := range untracked {
		items = append(items, tuiInstalledItem{
			untracked: file,
			status:    "não rastreado",
		})
	}

	return items, err
}

func tuiInstalledDetails(inst *instance.Instance, item tuiInstalledItem) string {
	var builder strings.Builder

	if item.tracked {
		entry := item.entry
		fmt.Fprintf(&builder, "%s\n\n", sanitizeText(entry.ProjectLabel()))
		fmt.Fprintf(&builder, "Tipo: %s\n", displayProjectType(entry.Type))
		fmt.Fprintf(&builder, "Versão: %s (%s)\n", sanitizeText(entry.VersionLabel()), displayChannel(entry.Channel))
		fmt.Fprintf(&builder, "Projeto: %s (%s)\n", sanitizeText(entry.ProjectID), sanitizeText(entry.ProjectSlug))
		fmt.Fprintf(&builder, "Provedor: %s\n", providerDisplayName(entry.Provider))
		fmt.Fprintf(&builder, "Arquivo: %s\n", sanitizeText(entry.FilePath))
		fmt.Fprintf(&builder, "Status: %s\n", item.status)
		if entry.FileSize > 0 {
			fmt.Fprintf(&builder, "Tamanho: %s\n", formatBytes(entry.FileSize))
		}
		if entry.GameVersion != "" {
			fmt.Fprintf(&builder, "Versão do jogo: %s\n", sanitizeText(entry.GameVersion))
		}
		if entry.Loader != "" {
			fmt.Fprintf(&builder, "Loader: %s\n", sanitizeText(entry.Loader))
		}
		if len(entry.GameVersions) > 0 {
			fmt.Fprintf(&builder, "Jogos compatíveis: %s\n", summarizeValues(entry.GameVersions, 12))
		}
		if len(entry.Loaders) > 0 {
			fmt.Fprintf(&builder, "Loaders compatíveis: %s\n", summarizeValues(entry.Loaders, 12))
		}
		if !entry.InstalledAt.IsZero() {
			fmt.Fprintf(&builder, "Instalado em: %s\n", entry.InstalledAt.Format("2006-01-02 15:04:05 MST"))
		}
		if !entry.UpdatedAt.IsZero() {
			fmt.Fprintf(&builder, "Atualizado em: %s\n", entry.UpdatedAt.Format("2006-01-02 15:04:05 MST"))
		}
		if len(entry.Hashes) > 0 {
			builder.WriteString("\nChecksums:\n")
			algorithms := make([]string, 0, len(entry.Hashes))
			for algorithm := range entry.Hashes {
				algorithms = append(algorithms, algorithm)
			}
			sort.Strings(algorithms)
			for _, algorithm := range algorithms {
				fmt.Fprintf(&builder, "  %s: %s\n", sanitizeText(algorithm), sanitizeText(entry.Hashes[algorithm]))
			}
		}

		return strings.TrimRight(builder.String(), "\n")
	}

	file := item.untracked
	fmt.Fprintf(&builder, "%s\n\n", sanitizeText(file.rel))
	fmt.Fprintf(&builder, "Tipo: %s\n", displayProjectType(file.kind))
	fmt.Fprintf(&builder, "Status: arquivo não rastreado\n")
	fmt.Fprintf(&builder, "Tamanho: %s\n", formatBytes(file.size))
	fmt.Fprintf(&builder, "\nEste arquivo está na pasta gerenciada, mas não foi instalado pelo mcjava-manager.\n")

	return strings.TrimRight(builder.String(), "\n")
}
