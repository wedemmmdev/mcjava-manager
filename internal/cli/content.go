package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/wedemmmdev/modink/internal/instance"
	"github.com/wedemmmdev/modink/internal/manifest"
	"github.com/wedemmmdev/modink/pkg/modsource"
)

type managedFolder struct {
	kind modsource.ProjectKind
	name string
}

func managedContentFolders() []managedFolder {
	return []managedFolder{
		{kind: modsource.KindMod, name: instance.ModsDirName},
		{kind: modsource.KindShaderPack, name: instance.ShaderPacksDirName},
		{kind: modsource.KindResourcePack, name: instance.ResourcePacksDirName},
	}
}

func parseManagedKind(value string) (modsource.ProjectKind, error) {
	kind, err := parseProjectKind(value)
	if err != nil {
		return "", err
	}
	if kind == "" {
		return "", nil
	}
	if _, ok := manifest.ContentDirectory(kind); !ok {
		return "", fmt.Errorf("tipo inválido %q: use mod, shader ou resourcepack", strings.TrimSpace(value))
	}

	return kind, nil
}

func matchManifestEntries(entries []manifest.Entry, query string, kind modsource.ProjectKind) []manifest.Entry {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	queryPath := normalizeQueryPath(query)

	seen := make(map[string]struct{})
	var matches []manifest.Entry
	for _, entry := range entries {
		if kind != "" && entry.Type != kind {
			continue
		}
		if !entryMatchesQuery(entry, query, queryPath) {
			continue
		}
		if _, ok := seen[entry.FilePath]; ok {
			continue
		}
		seen[entry.FilePath] = struct{}{}
		matches = append(matches, entry)
	}

	return matches
}

func entryMatchesQuery(entry manifest.Entry, query, queryPath string) bool {
	switch {
	case strings.EqualFold(entry.ProjectID, query):
		return true
	case strings.EqualFold(entry.ProjectSlug, query):
		return true
	case strings.EqualFold(entry.ProjectName, query):
		return true
	case strings.EqualFold(entry.FileName, query):
		return true
	case queryPath != "" && manifest.PathsEqual(entry.FilePath, queryPath):
		return true
	default:
		return false
	}
}

func normalizeQueryPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "./")
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return ""
	}

	return cleaned
}

func describeManifestEntry(entry manifest.Entry) string {
	return fmt.Sprintf(
		"%s %s %s (%s)",
		displayProjectType(entry.Type),
		sanitizeText(entry.ProjectLabel()),
		sanitizeText(entry.VersionLabel()),
		sanitizeText(entry.FilePath),
	)
}

func ambiguousManifestEntryError(query string, matches []manifest.Entry) error {
	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"mais de um conteúdo instalado corresponde a %q; use o caminho do arquivo",
		sanitizeText(strings.TrimSpace(query)),
	)
	for _, entry := range matches {
		builder.WriteString("\n  - ")
		builder.WriteString(describeManifestEntry(entry))
	}

	return errors.New(builder.String())
}

func manifestEntryStatus(inst *instance.Instance, entry manifest.Entry) string {
	if inst == nil {
		return "erro"
	}

	target, err := entry.TargetPath(inst.Path)
	if err != nil {
		return "inválido"
	}

	info, err := os.Stat(target)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "ausente"
	case err != nil:
		return "erro"
	case info.IsDir():
		return "diretório"
	default:
		return "ok"
	}
}
