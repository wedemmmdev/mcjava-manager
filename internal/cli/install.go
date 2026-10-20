package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

var errInstallUnsupportedKind = errors.New("tipo de conteúdo não suportado para instalação")

type installOptions struct {
	provider    string
	versionID   string
	gameVersion string
	loader      string
	channels    []string
	overwrite   bool
}

func newInstallCommand(client modsource.Client, globalOpts *options, downloader *download.Client) *cobra.Command {
	opts := &installOptions{provider: string(modsource.ProviderModrinth)}

	cmd := &cobra.Command{
		Use:   "install <slug|id>",
		Short: "Instala um mod, shader ou resource pack em uma instância",
		Long: `Instala um mod, shader ou resource pack em uma instância do Minecraft.

A instância deve ser informada com --instance ou pela variável de ambiente
MCJAVA_MANAGER_INSTANCE.

Sem --version, a versão release mais recente compatível com --game-version e
--loader é selecionada. Use --version para instalar um ID específico listado
por "mcjava-manager versions"; nesse caso, os filtros informados são apenas
validados contra a versão escolhida.

Os checksums SHA1/SHA512 publicados pelo provedor são verificados antes da
instalação. Dependências obrigatórias ainda não são resolvidas automaticamente.`,
		Example: `  mcjava-manager install sodium --instance ./minha-instancia --game-version 1.21.1 --loader fabric
  mcjava-manager install sodium --instance ./minha-instancia --version SMxNOGZ6
  mcjava-manager install sodium --instance ./minha-instancia --overwrite`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := strings.TrimSpace(args[0])
			if projectID == "" {
				return fmt.Errorf("informe o slug ou ID do projeto")
			}

			filter, channelExplicit, err := opts.selectionFilter(cmd)
			if err != nil {
				return err
			}

			instancePath, err := globalOpts.resolveInstance()
			if err != nil {
				return err
			}
			inst, err := instance.Open(instancePath)
			if err != nil {
				return err
			}

			projectClient, err := resolveProviderClient(client, opts.provider)
			if err != nil {
				return err
			}

			project, err := projectClient.GetProject(cmd.Context(), projectID)
			if err != nil {
				return fmt.Errorf("falha ao obter o projeto %q: %w", projectID, err)
			}
			project.ID = strings.TrimSpace(project.ID)
			if project.ID == "" {
				return fmt.Errorf("o provedor não retornou o ID do projeto %q", projectID)
			}

			directoryName, err := installDirectoryName(project.Type)
			if err != nil {
				return err
			}

			version, err := selectInstallVersion(cmd.Context(), projectClient, project, opts, filter, channelExplicit)
			if err != nil {
				return err
			}

			file, err := version.PrimaryFile()
			if err != nil {
				return fmt.Errorf(
					"a versão %q de %q não possui um arquivo instalável: %w",
					versionDisplayLabel(version),
					projectDisplayName(project),
					err,
				)
			}
			safeName, err := download.SafeFilename(file.Name)
			if err != nil {
				return fmt.Errorf("o provedor retornou um nome de arquivo inválido para %q: %w", projectDisplayName(project), err)
			}

			targetDirectory := filepath.Join(inst.Path, directoryName)
			targetPath := filepath.Join(targetDirectory, safeName)

			if err := inst.EnsureLayout(); err != nil {
				return err
			}

			dl := downloader
			if dl == nil {
				dl = download.New(download.WithUserAgent(providerUserAgent()))
			}

			progress := newProgressPrinter(cmd.ErrOrStderr())
			result, downloadErr := dl.Download(cmd.Context(), download.Request{
				URL:            file.URL,
				Target:         targetPath,
				ExpectedHashes: file.Hashes,
				Overwrite:      opts.overwrite,
				Progress:       progress.Update,
			})
			finishErr := progress.Finish(downloadErr == nil)
			if downloadErr != nil {
				wrapped := wrapInstallDownloadError(downloadErr, safeName, targetDirectory)
				if finishErr != nil {
					return errors.Join(wrapped, finishErr)
				}
				return wrapped
			}
			if finishErr != nil {
				return fmt.Errorf("arquivo instalado em %q, mas falha ao finalizar o progresso: %w", result.Path, finishErr)
			}

			if _, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Instalado %s %s em %s (checksum verificado)\n",
				projectDisplayName(project),
				versionDisplayLabel(version),
				result.Path,
			); err != nil {
				return fmt.Errorf("arquivo instalado em %q, mas falha ao escrever o resultado: %w", result.Path, err)
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.provider, "provider", string(modsource.ProviderModrinth), "provedor de conteúdo (modrinth)")
	flags.StringVar(&opts.versionID, "version", "", "ID exato da versão a instalar")
	flags.StringVarP(&opts.gameVersion, "game-version", "g", "", "versão do Minecraft (ex.: 1.20.1)")
	flags.StringVarP(&opts.loader, "loader", "l", "", "loader (fabric, forge, neoforge, quilt, ...)")
	flags.StringSliceVarP(&opts.channels, "channel", "c", nil, "canal: release, beta, alpha (padrão: release na seleção automática)")
	flags.BoolVar(&opts.overwrite, "overwrite", false, "substitui o arquivo de mesmo nome se ele já existir")

	return cmd
}

func (o *installOptions) selectionFilter(cmd *cobra.Command) (modsource.VersionFilter, bool, error) {
	channels, err := parseReleaseChannels(o.channels)
	if err != nil {
		return modsource.VersionFilter{}, false, err
	}

	channelExplicit := cmd.Flags().Changed("channel")
	if strings.TrimSpace(o.versionID) == "" && !channelExplicit {
		channels = []modsource.ReleaseChannel{modsource.ChannelRelease}
	}

	return modsource.VersionFilter{
		GameVersion: strings.TrimSpace(o.gameVersion),
		Loader:      strings.TrimSpace(o.loader),
		Channels:    channels,
	}, channelExplicit, nil
}

func selectInstallVersion(
	ctx context.Context,
	client modsource.Client,
	project modsource.Project,
	opts *installOptions,
	filter modsource.VersionFilter,
	channelExplicit bool,
) (modsource.Version, error) {
	if versionID := strings.TrimSpace(opts.versionID); versionID != "" {
		version, err := client.GetVersion(ctx, versionID)
		if err != nil {
			return modsource.Version{}, fmt.Errorf("falha ao obter a versão %q: %w", versionID, err)
		}
		versionProjectID := strings.TrimSpace(version.ProjectID)
		if versionProjectID == "" {
			return modsource.Version{}, fmt.Errorf("a versão %q não informa o projeto associado", versionID)
		}
		if versionProjectID != project.ID {
			return modsource.Version{}, fmt.Errorf(
				"a versão %q não pertence ao projeto %q",
				versionID,
				projectDisplayName(project),
			)
		}
		if err := validateVersionSelection(version, filter, channelExplicit); err != nil {
			return modsource.Version{}, err
		}

		return version, nil
	}

	versions, err := client.ListVersions(ctx, project.ID, filter)
	if err != nil {
		return modsource.Version{}, fmt.Errorf("falha ao listar as versões de %q: %w", projectDisplayName(project), err)
	}

	version, ok, err := newestDownloadableVersion(versions)
	if err != nil {
		return modsource.Version{}, err
	}
	if !ok {
		return modsource.Version{}, fmt.Errorf(
			"nenhuma versão instalável de %q foi encontrada com os filtros informados",
			projectDisplayName(project),
		)
	}

	return version, nil
}

func validateVersionSelection(version modsource.Version, filter modsource.VersionFilter, channelExplicit bool) error {
	if filter.GameVersion != "" && !containsFold(version.GameVersions, filter.GameVersion) {
		return fmt.Errorf(
			"a versão %q não é compatível com a versão do Minecraft %q",
			versionDisplayLabel(version),
			filter.GameVersion,
		)
	}
	if filter.Loader != "" && !containsFold(version.Loaders, filter.Loader) {
		return fmt.Errorf(
			"a versão %q não é compatível com o loader %q",
			versionDisplayLabel(version),
			filter.Loader,
		)
	}
	if channelExplicit && len(filter.Channels) > 0 && !containsChannel(version.Channel, filter.Channels) {
		return fmt.Errorf(
			"a versão %q não pertence aos canais selecionados",
			versionDisplayLabel(version),
		)
	}

	return nil
}

func newestDownloadableVersion(versions []modsource.Version) (modsource.Version, bool, error) {
	ordered := append([]modsource.Version(nil), versions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].PublishedAt.After(ordered[j].PublishedAt)
	})

	for _, version := range ordered {
		_, err := version.PrimaryFile()
		switch {
		case err == nil:
			return version, true, nil
		case errors.Is(err, modsource.ErrNoFiles), errors.Is(err, modsource.ErrDownloadUnavailable):
			continue
		default:
			return modsource.Version{}, false, fmt.Errorf(
				"falha ao inspecionar a versão %q: %w",
				versionDisplayLabel(version),
				err,
			)
		}
	}

	return modsource.Version{}, false, nil
}

func installDirectoryName(kind modsource.ProjectKind) (string, error) {
	switch kind {
	case modsource.KindMod:
		return instance.ModsDirName, nil
	case modsource.KindShaderPack:
		return instance.ShaderPacksDirName, nil
	case modsource.KindResourcePack:
		return instance.ResourcePacksDirName, nil
	default:
		label := displayProjectType(kind)
		if label == "-" {
			label = "tipo de projeto desconhecido"
		}

		return "", fmt.Errorf("%w: %s", errInstallUnsupportedKind, label)
	}
}

func wrapInstallDownloadError(err error, filename, directory string) error {
	switch {
	case errors.Is(err, download.ErrTargetExists):
		return fmt.Errorf("o arquivo %q já existe em %q; use --overwrite para substituí-lo: %w", filename, directory, err)
	case errors.Is(err, download.ErrNoChecksum):
		return fmt.Errorf("o arquivo %q não possui checksum SHA1/SHA512; instalação recusada: %w", filename, err)
	case errors.Is(err, download.ErrChecksumMismatch):
		return fmt.Errorf("o checksum do arquivo %q não confere; o arquivo não foi instalado: %w", filename, err)
	case errors.Is(err, download.ErrTooLarge):
		return fmt.Errorf("o arquivo %q excede o limite de download: %w", filename, err)
	case errors.Is(err, download.ErrInvalidFilename):
		return fmt.Errorf("nome de arquivo inválido retornado pelo provedor: %w", err)
	default:
		return fmt.Errorf("falha ao baixar %q para %q: %w", filename, directory, err)
	}
}

func containsFold(values []string, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}

	return false
}

func containsChannel(channel modsource.ReleaseChannel, channels []modsource.ReleaseChannel) bool {
	for _, candidate := range channels {
		if candidate == channel {
			return true
		}
	}

	return false
}
