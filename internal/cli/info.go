package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type infoOptions struct {
	provider    string
	description bool
}

func newInfoCommand(client modsource.Client) *cobra.Command {
	opts := &infoOptions{provider: string(modsource.ProviderModrinth)}

	cmd := &cobra.Command{
		Use:   "info <slug|id>",
		Short: "Exibe detalhes de um projeto",
		Long: `Exibe os metadados de um projeto do provedor selecionado, incluindo
resumo, licença, categorias, loaders, versões do jogo e links.

Use --description para imprimir também a descrição completa mantida pelo
provedor.`,
		Example: `  mcjava-manager info sodium
  mcjava-manager info sodium --description
  mcjava-manager info AANobbMI`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			if id == "" {
				return fmt.Errorf("informe o slug ou ID do projeto")
			}

			projectClient, err := resolveProviderClient(client, opts.provider)
			if err != nil {
				return err
			}

			project, err := projectClient.GetProject(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("falha ao obter o projeto %q: %w", id, err)
			}

			return writeInfoResult(cmd.OutOrStdout(), projectClient.Provider(), project, opts.description)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.provider, "provider", string(modsource.ProviderModrinth), "provedor de conteúdo (modrinth)")
	flags.BoolVar(&opts.description, "description", false, "exibe a descrição completa do projeto")

	return cmd
}

func writeInfoResult(out io.Writer, provider modsource.Provider, project modsource.Project, includeDescription bool) error {
	var builder strings.Builder

	fmt.Fprintf(&builder, "%s\n\n", projectDisplayName(project))
	fmt.Fprintf(&builder, "Tipo: %s\n", displayProjectType(project.Type))
	fmt.Fprintf(&builder, "Provedor: %s\n", providerDisplayName(provider))
	writeInfoField(&builder, "ID", sanitizeText(project.ID))
	writeInfoField(&builder, "Slug", sanitizeText(project.Slug))
	writeInfoField(&builder, "Autores", authorsText(project.Authors))
	writeInfoField(&builder, "Licença", sanitizeText(project.License))
	fmt.Fprintf(&builder, "Downloads: %s\n", formatCount(project.Downloads))
	fmt.Fprintf(&builder, "Seguidores: %s\n", formatCount(project.Followers))

	if !project.CreatedAt.IsZero() {
		fmt.Fprintf(&builder, "Criado em: %s\n", project.CreatedAt.Format("2006-01-02"))
	}
	if !project.UpdatedAt.IsZero() {
		fmt.Fprintf(&builder, "Atualizado em: %s\n", project.UpdatedAt.Format("2006-01-02"))
	}
	if len(project.Categories) > 0 {
		fmt.Fprintf(&builder, "Categorias: %s\n", summarizeValues(project.Categories, 8))
	}
	if len(project.Loaders) > 0 {
		fmt.Fprintf(&builder, "Loaders: %s\n", summarizeValues(project.Loaders, 8))
	}
	if len(project.GameVersions) > 0 {
		fmt.Fprintf(&builder, "Versões do jogo: %s\n", summarizeValues(project.GameVersions, 12))
	}

	if summary := strings.TrimSpace(sanitizeText(project.Summary)); summary != "" {
		fmt.Fprintf(&builder, "\nResumo:\n%s\n", summary)
	}

	if includeDescription {
		builder.WriteString("\nDescrição completa:\n")
		if description := strings.TrimSpace(sanitizeBlock(project.Description)); description != "" {
			builder.WriteString(description)
			builder.WriteString("\n")
		} else {
			builder.WriteString("-\n")
		}
	}

	writeInfoLinks(&builder, project.Links)

	if _, err := io.WriteString(out, builder.String()); err != nil {
		return fmt.Errorf("falha ao escrever os detalhes do projeto: %w", err)
	}

	return nil
}

func writeInfoField(builder *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "-"
	}

	fmt.Fprintf(builder, "%s: %s\n", label, value)
}

func writeInfoLinks(builder *strings.Builder, links modsource.Links) {
	entries := []struct {
		label string
		url   string
	}{
		{label: "Site", url: links.Website},
		{label: "Código", url: links.Source},
		{label: "Issues", url: links.Issues},
		{label: "Wiki", url: links.Wiki},
		{label: "Discord", url: links.Discord},
	}

	present := false
	for _, entry := range entries {
		if strings.TrimSpace(sanitizeText(entry.url)) != "" {
			present = true
			break
		}
	}
	if !present {
		return
	}

	builder.WriteString("\nLinks:\n")
	for _, entry := range entries {
		url := strings.TrimSpace(sanitizeText(entry.url))
		if url != "" {
			fmt.Fprintf(builder, "  %s: %s\n", entry.label, url)
		}
	}
}
