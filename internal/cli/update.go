package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type updateOptions struct {
	gameVersion    string
	loader         string
	channels       []string
	check          bool
	overwrite      bool
	noDependencies bool
}

func newUpdateCommand(client modsource.Client, globalOpts *options, downloader *download.Client) *cobra.Command {
	opts := &updateOptions{}

	cmd := &cobra.Command{
		Use:   "update [slug|id|arquivo ...]",
		Short: "Atualiza o conteúdo rastreado de uma instância",
		Long: `Verifica e instala versões mais recentes do conteúdo registrado no
manifesto da instância.

Sem argumentos, todos os itens rastreados são verificados. Informe slugs, IDs,
nomes de projeto, nomes de arquivo ou caminhos para limitar a atualização.

Por padrão, cada item mantém a versão do Minecraft, o loader e o canal usados
quando foi instalado. Use --game-version, --loader ou --channel para
sobrescrever esses filtros. Use --check para apenas exibir o que seria feito.

Dependências obrigatórias das novas versões são instaladas ou atualizadas
automaticamente. Use --no-dependencies para desativar esse comportamento.`,
		Example: `  mcjava-manager update --instance ./minha-instancia
  mcjava-manager update sodium --instance ./minha-instancia
  mcjava-manager update --instance ./minha-instancia --check
  mcjava-manager update sodium --instance ./minha-instancia --channel release`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.check && opts.overwrite {
				return fmt.Errorf("--overwrite não pode ser usado junto com --check")
			}

			channels, err := parseReleaseChannels(opts.channels)
			if err != nil {
				return err
			}
			channelExplicit := cmd.Flags().Changed("channel")

			instancePath, err := globalOpts.resolveInstance()
			if err != nil {
				return err
			}
			inst, err := instance.Open(instancePath)
			if err != nil {
				return err
			}

			store, err := manifest.Load(inst)
			if err != nil {
				return err
			}

			entries := store.Entries()
			if len(entries) == 0 {
				_, err := io.WriteString(cmd.OutOrStdout(), "Nenhum conteúdo rastreado nesta instância.\n")
				if err != nil {
					return fmt.Errorf("falha ao escrever o resultado da atualização: %w", err)
				}

				return nil
			}

			selected, err := selectUpdateEntries(entries, args)
			if err != nil {
				return err
			}

			dl := downloader
			if dl == nil {
				dl = download.New(download.WithUserAgent(providerUserAgent()))
			}

			var updateErrs []error
			for _, entry := range selected {
				err := updateEntry(
					cmd.Context(),
					client,
					dl,
					store,
					inst,
					entry,
					opts,
					channels,
					channelExplicit,
					cmd.OutOrStdout(),
					cmd.ErrOrStderr(),
				)
				if err != nil {
					updateErrs = append(updateErrs, fmt.Errorf(
						"falha ao atualizar %q: %w",
						sanitizeText(entry.ProjectLabel()),
						err,
					))
					continue
				}
			}

			if len(updateErrs) > 0 {
				return errors.Join(updateErrs...)
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.gameVersion, "game-version", "g", "", "versão do Minecraft (padrão: a usada na instalação)")
	flags.StringVarP(&opts.loader, "loader", "l", "", "loader (padrão: o usado na instalação)")
	flags.StringSliceVarP(&opts.channels, "channel", "c", nil, "canal: release, beta, alpha (padrão: o canal da versão instalada)")
	flags.BoolVar(&opts.check, "check", false, "apenas verifica atualizações, sem baixar arquivos")
	flags.BoolVar(&opts.overwrite, "overwrite", false, "substitui arquivos não rastreados com o mesmo nome")
	flags.BoolVar(&opts.noDependencies, "no-dependencies", false, "não instala dependências obrigatórias automaticamente")

	return cmd
}

func selectUpdateEntries(entries []manifest.Entry, args []string) ([]manifest.Entry, error) {
	if len(args) == 0 {
		return entries, nil
	}

	selected := make(map[string]manifest.Entry, len(args))
	for _, arg := range args {
		query := strings.TrimSpace(arg)
		if query == "" {
			return nil, fmt.Errorf("informe um slug, ID, arquivo ou caminho válido")
		}

		matches := matchManifestEntries(entries, query, "")
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("conteúdo instalado %q não encontrado", sanitizeText(query))
		case 1:
			selected[matches[0].FilePath] = matches[0]
		default:
			return nil, ambiguousManifestEntryError(query, matches)
		}
	}

	result := make([]manifest.Entry, 0, len(selected))
	for _, entry := range selected {
		result = append(result, entry)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].FilePath < result[j].FilePath
	})

	return result, nil
}

func updateEntry(
	ctx context.Context,
	client modsource.Client,
	downloader *download.Client,
	store *manifest.Store,
	inst *instance.Instance,
	entry manifest.Entry,
	opts *updateOptions,
	channels []modsource.ReleaseChannel,
	channelExplicit bool,
	out io.Writer,
	errOut io.Writer,
) error {
	entryClient, err := resolveProviderClient(client, string(entry.Provider))
	if err != nil {
		return err
	}

	filter := updateFilter(entry, opts, channels, channelExplicit)

	project, err := entryClient.GetProject(ctx, entry.ProjectID)
	if err != nil {
		if errors.Is(err, modsource.ErrNotFound) && strings.TrimSpace(entry.ProjectSlug) != "" {
			project, err = entryClient.GetProject(ctx, entry.ProjectSlug)
		}
		if err != nil {
			return fmt.Errorf("falha ao obter o projeto %q: %w", entry.ProjectID, err)
		}
	}
	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return fmt.Errorf("o provedor não retornou o ID do projeto %q", entry.ProjectID)
	}

	version, err := selectLatestVersion(ctx, entryClient, project, filter)
	if err != nil {
		return err
	}

	if version.ID == entry.VersionID {
		exists, err := entryFileExists(inst, entry)
		if err != nil {
			return err
		}
		if exists {
			return writeUpdateCheckResult(out, entry, version, nil)
		}
		if opts.check {
			return writeUpdateCheckResult(out, entry, version, fmt.Errorf("o arquivo está ausente"))
		}
		// The manifest points to the current version, but the file is gone.
		// Reinstall it below.
	} else if opts.check {
		return writeUpdateCheckResult(out, entry, version, nil)
	}

	installer := newContentInstaller(
		entryClient,
		downloader,
		store,
		inst,
		filter,
		channelExplicit,
		opts.overwrite,
		true,
		!opts.noDependencies,
		errOut,
	)
	outcomes, installErr := installer.installResolved(ctx, project, version, false, 0)
	writeErr := writeInstallOutcomes(out, outcomes, true)
	if writeErr != nil {
		if installErr != nil {
			return errors.Join(installErr, writeErr)
		}

		return writeErr
	}

	return installErr
}

func updateFilter(
	entry manifest.Entry,
	opts *updateOptions,
	channels []modsource.ReleaseChannel,
	channelExplicit bool,
) modsource.VersionFilter {
	gameVersion := strings.TrimSpace(opts.gameVersion)
	if gameVersion == "" {
		gameVersion = firstValue(entry.GameVersion, entry.GameVersions)
	}
	loader := strings.TrimSpace(opts.loader)
	if loader == "" {
		loader = firstValue(entry.Loader, entry.Loaders)
	}

	selectedChannels := channels
	if !channelExplicit {
		if entry.Channel != "" {
			selectedChannels = []modsource.ReleaseChannel{entry.Channel}
		} else {
			selectedChannels = []modsource.ReleaseChannel{modsource.ChannelRelease}
		}
	}

	return modsource.VersionFilter{
		GameVersion: gameVersion,
		Loader:      loader,
		Channels:    selectedChannels,
	}
}

func entryFileExists(inst *instance.Instance, entry manifest.Entry) (bool, error) {
	target, err := entry.TargetPath(inst.Path)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(target)
	switch {
	case err == nil:
		if info.IsDir() {
			return false, fmt.Errorf("o caminho %q é um diretório, mas deveria ser um arquivo", target)
		}

		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, instance.WrapFSError("verificar o arquivo instalado", target, err)
	}
}

func writeUpdateCheckResult(out io.Writer, entry manifest.Entry, latest modsource.Version, problem error) error {
	label := sanitizeText(entry.ProjectLabel())
	pathLabel := sanitizeText(entry.FilePath)

	if problem != nil {
		_, err := fmt.Fprintf(
			out,
			"Reinstalação necessária: %s %s (%s); %s.\n",
			label,
			sanitizeText(entry.VersionLabel()),
			pathLabel,
			problem,
		)
		if err != nil {
			return fmt.Errorf("falha ao escrever o resultado da atualização: %w", err)
		}

		return nil
	}

	if latest.ID == entry.VersionID {
		_, err := fmt.Fprintf(
			out,
			"Já atualizado: %s %s (%s)\n",
			label,
			sanitizeText(entry.VersionLabel()),
			pathLabel,
		)
		if err != nil {
			return fmt.Errorf("falha ao escrever o resultado da atualização: %w", err)
		}

		return nil
	}

	_, err := fmt.Fprintf(
		out,
		"Atualização disponível: %s %s -> %s (%s)\n",
		label,
		sanitizeText(entry.VersionLabel()),
		sanitizeText(versionDisplayLabel(latest)),
		pathLabel,
	)
	if err != nil {
		return fmt.Errorf("falha ao escrever o resultado da atualização: %w", err)
	}

	return nil
}
