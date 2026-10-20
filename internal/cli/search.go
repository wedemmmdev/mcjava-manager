package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/pkg/modrinth"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type searchOptions struct {
	projectType string
	loader      string
	gameVersion string
	categories  []string
	sort        string
	limit       int
	offset      int
	provider    string
}

func newSearchCommand(client modsource.Client) *cobra.Command {
	opts := &searchOptions{
		sort:     string(modsource.SortRelevance),
		limit:    modrinth.DefaultPageSize,
		provider: string(modsource.ProviderModrinth),
	}

	cmd := &cobra.Command{
		Use:   "search [termo]",
		Short: "Busca mods, shaders, resourcepacks e outros projetos",
		Long: `Busca projetos no provedor selecionado sem precisar informar uma instância.

O Modrinth é usado por padrão. Combine --type, --loader, --game-version e
--category para refinar os resultados. Sem um termo, a busca lista projetos
em destaque para os filtros informados.`,
		Example: `  mcjava-manager search sodium
  mcjava-manager search --type shader --game-version 1.20.1
  mcjava-manager search "resource pack" --category vanilla-like --limit 10
  mcjava-manager search sodium --loader fabric --sort downloads`,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := opts.query(args)
			if err != nil {
				return err
			}

			searchClient, err := resolveProviderClient(client, opts.provider)
			if err != nil {
				return err
			}

			result, err := searchClient.Search(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("falha ao buscar projetos: %w", err)
			}

			return writeSearchResult(cmd.OutOrStdout(), searchClient.Provider(), query, result)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.projectType, "type", "t", "", "tipo do projeto (mod, modpack, resourcepack, shader, datapack, plugin, world)")
	flags.StringVarP(&opts.loader, "loader", "l", "", "loader (fabric, forge, neoforge, quilt, ...)")
	flags.StringVarP(&opts.gameVersion, "game-version", "g", "", "versão do Minecraft (ex.: 1.20.1)")
	flags.StringSliceVarP(&opts.categories, "category", "c", nil, "categoria (pode repetir ou separar por vírgula)")
	flags.StringVarP(&opts.sort, "sort", "s", string(modsource.SortRelevance), "ordenação: relevance, downloads, follows, newest, updated")
	flags.IntVarP(&opts.limit, "limit", "n", modrinth.DefaultPageSize, "quantidade de resultados (1-100)")
	flags.IntVar(&opts.offset, "offset", 0, "deslocamento inicial dos resultados")
	flags.StringVar(&opts.provider, "provider", string(modsource.ProviderModrinth), "provedor de conteúdo (modrinth)")

	return cmd
}

func (o *searchOptions) query(args []string) (modsource.SearchQuery, error) {
	text := ""
	if len(args) > 0 {
		text = strings.TrimSpace(args[0])
	}

	kind, err := parseProjectKind(o.projectType)
	if err != nil {
		return modsource.SearchQuery{}, err
	}

	sortBy, err := parseSort(o.sort)
	if err != nil {
		return modsource.SearchQuery{}, err
	}

	if o.limit < 1 || o.limit > modrinth.MaxPageSize {
		return modsource.SearchQuery{}, fmt.Errorf("--limit deve estar entre 1 e %d", modrinth.MaxPageSize)
	}
	if o.offset < 0 {
		return modsource.SearchQuery{}, fmt.Errorf("--offset não pode ser negativo")
	}

	categories := make([]string, 0, len(o.categories))
	for _, category := range o.categories {
		if trimmed := strings.TrimSpace(category); trimmed != "" {
			categories = append(categories, trimmed)
		}
	}

	return modsource.SearchQuery{
		Text:        text,
		Kind:        kind,
		GameVersion: strings.TrimSpace(o.gameVersion),
		Loader:      strings.TrimSpace(o.loader),
		Categories:  categories,
		Sort:        sortBy,
		Limit:       o.limit,
		Offset:      o.offset,
	}, nil
}

func parseProjectKind(value string) (modsource.ProjectKind, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}

	kind := modsource.ProjectKind(value)
	if !kind.Valid() {
		return "", fmt.Errorf("tipo de projeto inválido %q: use mod, modpack, resourcepack, shader, datapack, plugin ou world", value)
	}

	return kind, nil
}

func parseSort(value string) (modsource.SortBy, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	sortBy := modsource.SortBy(value)

	switch sortBy {
	case modsource.SortRelevance, modsource.SortDownloads, modsource.SortFollows, modsource.SortNewest, modsource.SortUpdated:
		return sortBy, nil
	default:
		return "", fmt.Errorf("ordenação inválida %q: use relevance, downloads, follows, newest ou updated", value)
	}
}

func writeSearchResult(out io.Writer, provider modsource.Provider, query modsource.SearchQuery, result modsource.SearchResult) error {
	providerName := providerDisplayName(provider)

	resultLabel := "resultados"
	if result.Total == 1 {
		resultLabel = "resultado"
	}
	header := fmt.Sprintf("%s: %d %s", providerName, result.Total, resultLabel)
	if query.Text != "" {
		header = fmt.Sprintf("%s para %q", header, query.Text)
	}
	if _, err := fmt.Fprintf(out, "%s\n\n", header); err != nil {
		return fmt.Errorf("falha ao escrever o cabeçalho da busca: %w", err)
	}

	if len(result.Projects) == 0 {
		if _, err := io.WriteString(out, "Nenhum projeto encontrado.\n"); err != nil {
			return fmt.Errorf("falha ao escrever o resultado da busca: %w", err)
		}
		return nil
	}

	if err := writeSearchTable(out, result); err != nil {
		return err
	}

	start := result.Offset + 1
	end := result.Offset + len(result.Projects)
	if _, err := fmt.Fprintf(out, "\nExibindo %d-%d de %d.\n", start, end, result.Total); err != nil {
		return fmt.Errorf("falha ao escrever o resumo da busca: %w", err)
	}
	if end < result.Total {
		if _, err := fmt.Fprintf(out, "Próxima página: --offset %d\n", end); err != nil {
			return fmt.Errorf("falha ao escrever a próxima página: %w", err)
		}
	}

	return nil
}

func writeSearchTable(out io.Writer, result modsource.SearchResult) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(writer, "#\tNOME\tTIPO\tAUTOR\tDOWNLOADS\tSLUG/ID"); err != nil {
		return fmt.Errorf("falha ao escrever o cabeçalho da tabela: %w", err)
	}

	for index, project := range result.Projects {
		name := truncate(strings.TrimSpace(project.Name), 40)
		if name == "" {
			name = truncate(projectIdentifier(project), 40)
		}
		if name == "" {
			name = "(sem nome)"
		}

		identifier := projectIdentifier(project)
		if identifier == "" {
			identifier = "-"
		}

		if _, err := fmt.Fprintf(
			writer,
			"%d\t%s\t%s\t%s\t%s\t%s\n",
			result.Offset+index+1,
			name,
			displayProjectType(project.Type),
			truncate(authorsText(project.Authors), 24),
			formatCount(project.Downloads),
			identifier,
		); err != nil {
			return fmt.Errorf("falha ao escrever a tabela de resultados: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("falha ao finalizar a tabela de resultados: %w", err)
	}

	return nil
}
