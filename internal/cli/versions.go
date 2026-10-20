package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/pkg/modrinth"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type versionsOptions struct {
	provider    string
	gameVersion string
	loader      string
	channels    []string
	limit       int
	offset      int
}

func newVersionsCommand(client modsource.Client) *cobra.Command {
	opts := &versionsOptions{
		provider: string(modsource.ProviderModrinth),
		limit:    modrinth.DefaultPageSize,
	}

	cmd := &cobra.Command{
		Use:   "versions <slug|id>",
		Short: "Lista as versões disponíveis de um projeto",
		Long: `Lista as versões de um projeto do provedor selecionado.

É possível filtrar por versão do Minecraft, loader e canal de lançamento.
A paginação é aplicada localmente sobre a lista retornada pelo provedor.`,
		Example: `  mcjava-manager versions sodium
  mcjava-manager versions sodium --game-version 1.20.1 --loader fabric
  mcjava-manager versions sodium --channel release --limit 10
  mcjava-manager versions sodium --offset 20`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := strings.TrimSpace(args[0])
			if projectID == "" {
				return fmt.Errorf("informe o slug ou ID do projeto")
			}

			filter, err := opts.filter()
			if err != nil {
				return err
			}

			projectClient, err := resolveProviderClient(client, opts.provider)
			if err != nil {
				return err
			}

			versions, err := projectClient.ListVersions(cmd.Context(), projectID, filter)
			if err != nil {
				return fmt.Errorf("falha ao listar as versões de %q: %w", projectID, err)
			}

			return writeVersionsResult(cmd.OutOrStdout(), projectClient.Provider(), projectID, filter, opts.limit, opts.offset, versions)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.provider, "provider", string(modsource.ProviderModrinth), "provedor de conteúdo (modrinth)")
	flags.StringVarP(&opts.gameVersion, "game-version", "g", "", "versão do Minecraft (ex.: 1.20.1)")
	flags.StringVarP(&opts.loader, "loader", "l", "", "loader (fabric, forge, neoforge, quilt, ...)")
	flags.StringSliceVarP(&opts.channels, "channel", "c", nil, "canal: release, beta, alpha (pode repetir ou separar por vírgula)")
	flags.IntVarP(&opts.limit, "limit", "n", modrinth.DefaultPageSize, "quantidade de versões exibidas (1-100)")
	flags.IntVar(&opts.offset, "offset", 0, "deslocamento inicial das versões")

	return cmd
}

func (o *versionsOptions) filter() (modsource.VersionFilter, error) {
	if o.limit < 1 || o.limit > modrinth.MaxPageSize {
		return modsource.VersionFilter{}, fmt.Errorf("--limit deve estar entre 1 e %d", modrinth.MaxPageSize)
	}
	if o.offset < 0 {
		return modsource.VersionFilter{}, fmt.Errorf("--offset não pode ser negativo")
	}

	channels, err := parseReleaseChannels(o.channels)
	if err != nil {
		return modsource.VersionFilter{}, err
	}

	return modsource.VersionFilter{
		GameVersion: strings.TrimSpace(o.gameVersion),
		Loader:      strings.TrimSpace(o.loader),
		Channels:    channels,
	}, nil
}

func parseReleaseChannels(values []string) ([]modsource.ReleaseChannel, error) {
	var channels []modsource.ReleaseChannel
	seen := make(map[modsource.ReleaseChannel]struct{}, len(values))

	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part == "" {
				continue
			}

			channel := modsource.ReleaseChannel(part)
			switch channel {
			case modsource.ChannelRelease, modsource.ChannelBeta, modsource.ChannelAlpha:
			default:
				return nil, fmt.Errorf("canal inválido %q: use release, beta ou alpha", part)
			}

			if _, ok := seen[channel]; ok {
				continue
			}
			seen[channel] = struct{}{}
			channels = append(channels, channel)
		}
	}

	return channels, nil
}

func writeVersionsResult(out io.Writer, provider modsource.Provider, projectID string, filter modsource.VersionFilter, limit, offset int, versions []modsource.Version) error {
	if limit < 1 {
		limit = modrinth.DefaultPageSize
	}
	if offset < 0 {
		offset = 0
	}

	total := len(versions)
	versionLabel := "versões"
	if total == 1 {
		versionLabel = "versão"
	}
	header := fmt.Sprintf("%s: %d %s para %q", providerDisplayName(provider), total, versionLabel, sanitizeText(projectID))
	if summary := versionFilterSummary(filter); summary != "" {
		header = fmt.Sprintf("%s [%s]", header, summary)
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "%s\n\n", header)

	if total == 0 {
		builder.WriteString("Nenhuma versão encontrada com esses filtros.\n")
		return writeBuilder(out, &builder, "falha ao escrever as versões")
	}

	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	if start == end {
		maxOffset := total - 1
		if maxOffset < 0 {
			maxOffset = 0
		}
		fmt.Fprintf(&builder, "Nenhuma versão nesta página. Use --offset entre 0 e %d.\n", maxOffset)
		return writeBuilder(out, &builder, "falha ao escrever as versões")
	}

	if err := writeVersionsTable(&builder, versions[start:end], start); err != nil {
		return err
	}

	fmt.Fprintf(&builder, "\nExibindo %d-%d de %d.\n", start+1, end, total)
	if end < total {
		fmt.Fprintf(&builder, "Próxima página: --offset %d\n", end)
	}

	return writeBuilder(out, &builder, "falha ao escrever as versões")
}

func writeVersionsTable(out io.Writer, versions []modsource.Version, offset int) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(writer, "#\tVERSÃO\tCANAL\tJOGO\tLOADERS\tARQUIVO\tID"); err != nil {
		return fmt.Errorf("falha ao escrever o cabeçalho da tabela de versões: %w", err)
	}

	for index, version := range versions {
		label := strings.TrimSpace(version.VersionNumber)
		if label == "" {
			label = strings.TrimSpace(version.Name)
		}
		if label == "" {
			label = version.ID
		}
		if label == "" {
			label = "-"
		}

		fileLabel, err := versionFileLabel(version)
		if err != nil {
			return err
		}

		versionID := sanitizeText(version.ID)
		if versionID == "" {
			versionID = "-"
		}

		if _, err := fmt.Fprintf(
			writer,
			"%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			offset+index+1,
			truncate(label, 36),
			displayChannel(version.Channel),
			summarizeValues(version.GameVersions, 2),
			summarizeValues(version.Loaders, 3),
			truncate(fileLabel, 32),
			versionID,
		); err != nil {
			return fmt.Errorf("falha ao escrever a tabela de versões: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("falha ao finalizar a tabela de versões: %w", err)
	}

	return nil
}

func versionFilterSummary(filter modsource.VersionFilter) string {
	parts := make([]string, 0, 3)

	if gameVersion := strings.TrimSpace(filter.GameVersion); gameVersion != "" {
		parts = append(parts, "jogo="+sanitizeText(gameVersion))
	}
	if loader := strings.TrimSpace(filter.Loader); loader != "" {
		parts = append(parts, "loader="+sanitizeText(loader))
	}
	if len(filter.Channels) > 0 {
		channels := make([]string, 0, len(filter.Channels))
		for _, channel := range filter.Channels {
			channels = append(channels, displayChannel(channel))
		}
		parts = append(parts, "canal="+strings.Join(channels, ","))
	}

	return strings.Join(parts, ", ")
}

func versionFileLabel(version modsource.Version) (string, error) {
	file, err := version.PrimaryFile()
	if err != nil {
		if errors.Is(err, modsource.ErrNoFiles) || errors.Is(err, modsource.ErrDownloadUnavailable) {
			return "-", nil
		}

		return "", fmt.Errorf("falha ao obter o arquivo principal da versão %q: %w", version.ID, err)
	}

	name := strings.TrimSpace(sanitizeText(file.Name))
	if name == "" {
		return "-", nil
	}

	return name, nil
}

func writeBuilder(out io.Writer, builder *strings.Builder, errorContext string) error {
	if _, err := io.WriteString(out, builder.String()); err != nil {
		return fmt.Errorf("%s: %w", errorContext, err)
	}

	return nil
}
