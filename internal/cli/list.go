package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type listOptions struct {
	projectType string
}

type untrackedFile struct {
	kind modsource.ProjectKind
	rel  string
	size int64
}

func newListCommand(globalOpts *options) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista o conteúdo instalado na instância",
		Long: `Lista os mods, shaders e resource packs registrados no manifesto da
instância. Arquivos presentes nas pastas gerenciadas que não foram instalados
pelo mcjava-manager também são exibidos como não rastreados.

Use --type para limitar a listagem a um tipo de conteúdo.`,
		Example: `  mcjava-manager list --instance ./minha-instancia
  mcjava-manager list --instance ./minha-instancia --type mod
  mcjava-manager list --instance ./minha-instancia --type shader`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := parseManagedKind(opts.projectType)
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

			store, err := manifest.Load(inst)
			if err != nil {
				return err
			}

			return writeListResult(cmd.OutOrStdout(), inst, store, kind)
		},
	}

	cmd.Flags().StringVarP(&opts.projectType, "type", "t", "", "tipo de conteúdo (mod, shader, resourcepack)")

	return cmd
}

func writeListResult(out io.Writer, inst *instance.Instance, store *manifest.Store, kind modsource.ProjectKind) error {
	entries := store.Entries()
	if kind != "" {
		filtered := make([]manifest.Entry, 0, len(entries))
		for _, entry := range entries {
			if entry.Type == kind {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	untracked, err := scanUntrackedFiles(inst, store, kind)
	if err != nil {
		return err
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Instância: %s\n", sanitizeText(inst.Path))
	fmt.Fprintf(&builder, "Conteúdo rastreado: %d\n\n", len(entries))

	if len(entries) == 0 {
		builder.WriteString("Nenhum conteúdo rastreado.\n")
	} else if err := writeTrackedTable(&builder, inst, entries); err != nil {
		return err
	}

	fmt.Fprintf(&builder, "\nArquivos não rastreados: %d\n\n", len(untracked))
	if len(untracked) == 0 {
		builder.WriteString("Nenhum arquivo não rastreado nas pastas gerenciadas.\n")
	} else if err := writeUntrackedTable(&builder, untracked); err != nil {
		return err
	}

	if _, err := io.WriteString(out, builder.String()); err != nil {
		return fmt.Errorf("falha ao escrever a lista de conteúdo: %w", err)
	}

	return nil
}

func writeTrackedTable(out io.Writer, inst *instance.Instance, entries []manifest.Entry) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "TIPO\tNOME\tVERSÃO\tPROVEDOR\tARQUIVO\tSTATUS"); err != nil {
		return fmt.Errorf("falha ao escrever o cabeçalho da lista: %w", err)
	}

	for _, entry := range entries {
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			displayProjectType(entry.Type),
			truncate(entry.ProjectLabel(), 32),
			truncate(entry.VersionLabel(), 24),
			providerDisplayName(entry.Provider),
			truncate(entry.FilePath, 56),
			manifestEntryStatus(inst, entry),
		); err != nil {
			return fmt.Errorf("falha ao escrever a tabela de conteúdo: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("falha ao finalizar a tabela de conteúdo: %w", err)
	}

	return nil
}

func writeUntrackedTable(out io.Writer, untracked []untrackedFile) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "TIPO\tARQUIVO\tTAMANHO"); err != nil {
		return fmt.Errorf("falha ao escrever o cabeçalho dos arquivos não rastreados: %w", err)
	}

	for _, file := range untracked {
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%s\t%s\n",
			displayProjectType(file.kind),
			truncate(file.rel, 64),
			formatBytes(file.size),
		); err != nil {
			return fmt.Errorf("falha ao escrever a tabela de arquivos não rastreados: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("falha ao finalizar a tabela de arquivos não rastreados: %w", err)
	}

	return nil
}

func scanUntrackedFiles(inst *instance.Instance, store *manifest.Store, kind modsource.ProjectKind) ([]untrackedFile, error) {
	var files []untrackedFile

	for _, folder := range managedContentFolders() {
		if kind != "" && folder.kind != kind {
			continue
		}

		directory := filepath.Join(inst.Path, folder.name)
		entries, err := os.ReadDir(directory)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, instance.WrapFSError("listar a pasta", directory, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".mcjava-manager-") {
				continue
			}
			if _, err := download.SafeFilename(name); err != nil {
				continue
			}

			relative := folder.name + "/" + name
			if _, tracked := store.FindByPath(relative); tracked {
				continue
			}

			info, err := entry.Info()
			switch {
			case errors.Is(err, fs.ErrNotExist):
				// The entry disappeared during the scan, or it is a broken
				// symbolic link. There is no reliable file to list.
				continue
			case err != nil:
				target := filepath.Join(directory, name)
				return nil, instance.WrapFSError("inspecionar o arquivo", target, err)
			case info.IsDir():
				// A symbolic link to a directory is not a managed artifact.
				continue
			}

			files = append(files, untrackedFile{
				kind: folder.kind,
				rel:  relative,
				size: info.Size(),
			})
		}
	}

	sort.SliceStable(files, func(i, j int) bool {
		return files[i].rel < files[j].rel
	})

	return files, nil
}
