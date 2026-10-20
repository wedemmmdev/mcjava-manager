package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type removeOptions struct {
	projectType string
}

type removalTarget struct {
	path  string
	rel   string
	kind  modsource.ProjectKind
	entry *manifest.Entry
}

type removalResult struct {
	path    string
	rel     string
	entry   *manifest.Entry
	existed bool
}

func newRemoveCommand(globalOpts *options) *cobra.Command {
	opts := &removeOptions{}

	cmd := &cobra.Command{
		Use:   "remove <arquivo|slug|id>",
		Short: "Remove um conteúdo instalado da instância",
		Long: `Remove um mod, shader ou resource pack de uma instância.

O argumento pode ser o slug, o ID, o nome do projeto, o nome do arquivo ou o
caminho relativo do arquivo. Arquivos não rastreados nas pastas gerenciadas
também podem ser removidos pelo nome ou caminho.

A remoção é explícita e não pede confirmação adicional. Use --type para
desambiguar quando o mesmo nome existir em pastas diferentes.`,
		Example: `  mcjava-manager remove sodium --instance ./minha-instancia
  mcjava-manager remove mods/sodium.jar --instance ./minha-instancia
  mcjava-manager remove sodium.jar --instance ./minha-instancia --type mod`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(args[0])
			if query == "" {
				return fmt.Errorf("informe o arquivo, slug ou ID do conteúdo")
			}

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

			target, err := resolveRemovalTarget(inst, store, query, kind)
			if err != nil {
				return err
			}

			result, removeErr := removeTarget(inst, store, target)
			if removeErr == nil || result.existed {
				writeErr := writeRemovalResult(cmd.OutOrStdout(), result)
				if writeErr != nil {
					if removeErr != nil {
						return errors.Join(removeErr, writeErr)
					}

					return writeErr
				}
			}

			return removeErr
		},
	}

	cmd.Flags().StringVarP(&opts.projectType, "type", "t", "", "tipo de conteúdo (mod, shader, resourcepack)")

	return cmd
}

func resolveRemovalTarget(
	inst *instance.Instance,
	store *manifest.Store,
	query string,
	kind modsource.ProjectKind,
) (removalTarget, error) {
	matches := matchManifestEntries(store.Entries(), query, kind)
	switch len(matches) {
	case 0:
		return resolveUntrackedRemoval(inst, store, query, kind)
	case 1:
		entry := matches[0]
		targetPath, err := entry.TargetPath(inst.Path)
		if err != nil {
			return removalTarget{}, err
		}
		relative, entryKind, err := validateManagedPath(inst, targetPath, kind)
		if err != nil {
			return removalTarget{}, err
		}

		return removalTarget{
			path:  targetPath,
			rel:   relative,
			kind:  entryKind,
			entry: &entry,
		}, nil
	default:
		return removalTarget{}, ambiguousManifestEntryError(query, matches)
	}
}

func resolveUntrackedRemoval(
	inst *instance.Instance,
	store *manifest.Store,
	query string,
	kind modsource.ProjectKind,
) (removalTarget, error) {
	var candidates []string

	if filepath.IsAbs(query) || strings.ContainsAny(query, `/\`) {
		candidate := query
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(inst.Path, filepath.FromSlash(strings.ReplaceAll(candidate, `\`, "/")))
		}
		candidate = filepath.Clean(candidate)

		relative, candidateKind, err := validateManagedPath(inst, candidate, kind)
		if err != nil {
			return removalTarget{}, err
		}
		if err := ensureRemovableFile(candidate); err != nil {
			return removalTarget{}, err
		}

		return newRemovalTarget(inst, store, candidate, relative, candidateKind), nil
	}

	for _, folder := range managedContentFolders() {
		if kind != "" && folder.kind != kind {
			continue
		}
		candidate := filepath.Join(inst.Path, folder.name, query)
		if _, err := os.Lstat(candidate); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return removalTarget{}, instance.WrapFSError("inspecionar o arquivo", candidate, err)
		}
		if err := ensureRemovableFile(candidate); err != nil {
			return removalTarget{}, err
		}
		candidates = append(candidates, candidate)
	}

	switch len(candidates) {
	case 0:
		return removalTarget{}, fmt.Errorf("conteúdo instalado %q não encontrado", sanitizeText(query))
	case 1:
		relative, candidateKind, err := validateManagedPath(inst, candidates[0], kind)
		if err != nil {
			return removalTarget{}, err
		}

		return newRemovalTarget(inst, store, candidates[0], relative, candidateKind), nil
	default:
		var builder strings.Builder
		fmt.Fprintf(
			&builder,
			"mais de um arquivo corresponde a %q; use o caminho do arquivo ou --type",
			sanitizeText(query),
		)
		for _, candidate := range candidates {
			relative, _, err := validateManagedPath(inst, candidate, kind)
			if err != nil {
				continue
			}
			builder.WriteString("\n  - ")
			builder.WriteString(sanitizeText(relative))
		}

		return removalTarget{}, errors.New(builder.String())
	}
}

func newRemovalTarget(
	inst *instance.Instance,
	store *manifest.Store,
	pathValue, relative string,
	kind modsource.ProjectKind,
) removalTarget {
	target := removalTarget{
		path: pathValue,
		rel:  relative,
		kind: kind,
	}
	if entry, ok := store.FindByPath(relative); ok {
		copy := entry
		target.entry = &copy
	}

	return target
}

func validateManagedPath(
	inst *instance.Instance,
	absolute string,
	kind modsource.ProjectKind,
) (string, modsource.ProjectKind, error) {
	if inst == nil {
		return "", "", errors.New("instância inválida")
	}

	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(inst.Path, absolute)
	if err != nil {
		return "", "", fmt.Errorf("falha ao resolver o caminho %q: %w", absolute, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("o caminho %q está fora da instância", absolute)
	}

	slashRelative := filepath.ToSlash(relative)
	parts := strings.Split(slashRelative, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf(
			"o caminho %q deve apontar para um arquivo diretamente em mods/, shaderpacks/ ou resourcepacks/",
			absolute,
		)
	}

	var folder managedFolder
	folderName := parts[0]
	found := false
	for _, candidate := range managedContentFolders() {
		if candidate.name == parts[0] ||
			(runtime.GOOS == "windows" && strings.EqualFold(candidate.name, parts[0])) {
			folder = candidate
			folderName = candidate.name
			found = true
			break
		}
	}
	if !found {
		return "", "", fmt.Errorf(
			"o caminho %q não está em uma pasta gerenciada (mods/, shaderpacks/ ou resourcepacks/)",
			absolute,
		)
	}
	if kind != "" && folder.kind != kind {
		return "", "", fmt.Errorf(
			"o arquivo %q pertence ao tipo %s, mas --type=%s foi informado",
			absolute,
			folder.kind,
			kind,
		)
	}
	if _, err := download.SafeFilename(parts[1]); err != nil {
		return "", "", fmt.Errorf("nome de arquivo inválido em %q: %w", absolute, err)
	}

	return folderName + "/" + parts[1], folder.kind, nil
}

func ensureRemovableFile(pathValue string) error {
	info, err := os.Lstat(pathValue)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("o arquivo %q não existe", pathValue)
		}

		return instance.WrapFSError("inspecionar o arquivo", pathValue, err)
	}
	if info.IsDir() {
		return fmt.Errorf(
			"o caminho %q é um diretório; remova-o manualmente se tiver certeza",
			pathValue,
		)
	}
	if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("o caminho %q não é um arquivo regular ou link simbólico", pathValue)
	}

	return nil
}

func removeTarget(inst *instance.Instance, store *manifest.Store, target removalTarget) (removalResult, error) {
	result := removalResult{
		path:  target.path,
		rel:   target.rel,
		entry: target.entry,
	}

	info, err := os.Lstat(target.path)
	switch {
	case err == nil:
		if info.IsDir() {
			return result, fmt.Errorf(
				"o caminho %q é um diretório; remova-o manualmente se tiver certeza",
				target.path,
			)
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return result, fmt.Errorf("o caminho %q não é um arquivo regular ou link simbólico", target.path)
		}
		if err := os.Remove(target.path); err != nil {
			return result, instance.WrapFSError("remover o arquivo", target.path, err)
		}
		result.existed = true
	case errors.Is(err, fs.ErrNotExist):
		if target.entry == nil {
			return result, fmt.Errorf("o arquivo %q não existe", target.path)
		}
	default:
		return result, instance.WrapFSError("inspecionar o arquivo", target.path, err)
	}

	if target.entry != nil {
		if _, _, err := store.RemoveByPath(target.entry.FilePath); err != nil {
			if result.existed {
				return result, fmt.Errorf(
					"arquivo removido em %q, mas falha ao atualizar o manifesto: %w",
					target.path,
					err,
				)
			}

			return result, fmt.Errorf("falha ao atualizar o manifesto: %w", err)
		}
	}

	return result, nil
}

func writeRemovalResult(out io.Writer, result removalResult) error {
	if result.entry == nil {
		if _, err := fmt.Fprintf(out, "Removido arquivo não rastreado %s\n", sanitizeText(result.rel)); err != nil {
			return fmt.Errorf("falha ao escrever o resultado da remoção: %w", err)
		}

		return nil
	}

	entry := result.entry
	if result.existed {
		if _, err := fmt.Fprintf(
			out,
			"Removido %s %s %s (%s)\n",
			displayProjectType(entry.Type),
			sanitizeText(entry.ProjectLabel()),
			sanitizeText(entry.VersionLabel()),
			sanitizeText(entry.FilePath),
		); err != nil {
			return fmt.Errorf("falha ao escrever o resultado da remoção: %w", err)
		}

		return nil
	}

	if _, err := fmt.Fprintf(
		out,
		"Registro removido: %s %s %s (%s); o arquivo já não existia.\n",
		displayProjectType(entry.Type),
		sanitizeText(entry.ProjectLabel()),
		sanitizeText(entry.VersionLabel()),
		sanitizeText(entry.FilePath),
	); err != nil {
		return fmt.Errorf("falha ao escrever o resultado da remoção: %w", err)
	}

	return nil
}
