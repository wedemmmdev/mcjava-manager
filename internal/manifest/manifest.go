// Package manifest persists the content installed in a Minecraft instance.
//
// The manifest lives inside the instance at
// .mcjava-manager/installed.json. It records the provider, project, version,
// selected compatibility filters, and target file for every artifact managed
// by mcjava-manager. Existing files that are not present in the manifest are
// deliberately left untouched.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

const (
	// StateDirName is the hidden directory that stores manager metadata.
	StateDirName = ".mcjava-manager"

	// FileName is the manifest file name inside StateDirName.
	FileName = "installed.json"

	// SchemaVersion is the current manifest schema.
	SchemaVersion = 1

	// maxFileBytes limits how much JSON the manager accepts from disk.
	maxFileBytes = 4 << 20 // 4 MiB
)

// Entry describes one installed content file.
type Entry struct {
	Provider      modsource.Provider       `json:"provider"`
	ProjectID     string                   `json:"project_id"`
	ProjectSlug   string                   `json:"project_slug,omitempty"`
	ProjectName   string                   `json:"project_name,omitempty"`
	Type          modsource.ProjectKind    `json:"type"`
	VersionID     string                   `json:"version_id"`
	VersionNumber string                   `json:"version_number,omitempty"`
	VersionName   string                   `json:"version_name,omitempty"`
	Channel       modsource.ReleaseChannel `json:"channel,omitempty"`
	GameVersion   string                   `json:"game_version,omitempty"`
	Loader        string                   `json:"loader,omitempty"`
	GameVersions  []string                 `json:"game_versions,omitempty"`
	Loaders       []string                 `json:"loaders,omitempty"`
	FileName      string                   `json:"file_name"`
	FilePath      string                   `json:"file_path"`
	FileSize      int64                    `json:"file_size,omitempty"`
	Hashes        map[string]string        `json:"hashes,omitempty"`
	InstalledAt   time.Time                `json:"installed_at,omitempty"`
	UpdatedAt     time.Time                `json:"updated_at,omitempty"`
}

// ProjectLabel returns a human-readable project name with safe fallbacks.
func (e Entry) ProjectLabel() string {
	for _, value := range []string{e.ProjectName, e.ProjectSlug, e.ProjectID} {
		if trimmed := strings.TrimSpace(sanitizeDisplayText(value)); trimmed != "" {
			return trimmed
		}
	}

	return "(sem nome)"
}

// VersionLabel returns a human-readable version label with safe fallbacks.
func (e Entry) VersionLabel() string {
	for _, value := range []string{e.VersionNumber, e.VersionName, e.VersionID} {
		if trimmed := strings.TrimSpace(sanitizeDisplayText(value)); trimmed != "" {
			return trimmed
		}
	}

	return "-"
}

func sanitizeDisplayText(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t' || r == 0x2028 || r == 0x2029:
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		case r >= 0x80 && r <= 0x9f:
			return -1
		case r == 0x061c || (r >= 0x200e && r <= 0x200f) ||
			(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			return -1
		default:
			return r
		}
	}, value)
}

// TargetPath resolves the absolute path of the installed file.
func (e Entry) TargetPath(instancePath string) (string, error) {
	relative, err := validateFilePath(e.Type, e.FilePath)
	if err != nil {
		return "", err
	}
	instancePath = strings.TrimSpace(instancePath)
	if instancePath == "" {
		return "", errors.New("caminho da instância não informado")
	}

	return filepath.Join(instancePath, filepath.FromSlash(relative)), nil
}

// ContentDirectory maps a supported content kind to its instance folder.
func ContentDirectory(kind modsource.ProjectKind) (string, bool) {
	switch kind {
	case modsource.KindMod:
		return instance.ModsDirName, true
	case modsource.KindShaderPack:
		return instance.ShaderPacksDirName, true
	case modsource.KindResourcePack:
		return instance.ResourcePacksDirName, true
	default:
		return "", false
	}
}

// PathsEqual reports whether two normalized, slash-separated manifest paths
// identify the same file. Windows filesystems compare paths case-insensitively.
func PathsEqual(a, b string) bool {
	return pathsEqual(a, b, runtime.GOOS == "windows")
}

func pathsEqual(a, b string, foldCase bool) bool {
	if foldCase {
		return strings.EqualFold(a, b)
	}

	return a == b
}

func pathIdentity(filePath string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(filePath)
	}

	return filePath
}

// RelativePath combines a content kind and a provider-supplied file name into
// a normalized, slash-separated manifest path.
func RelativePath(kind modsource.ProjectKind, filename string) (string, error) {
	directory, ok := ContentDirectory(kind)
	if !ok {
		return "", fmt.Errorf("tipo de conteúdo não suportado no manifesto: %q", kind)
	}

	safeName, err := download.SafeFilename(filename)
	if err != nil {
		return "", err
	}

	return directory + "/" + safeName, nil
}

// Store is an in-memory view of a manifest file.
type Store struct {
	instancePath string
	path         string
	doc          document
}

type document struct {
	Schema    int       `json:"schema"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Entries   []Entry   `json:"entries"`
}

// Load reads the manifest for inst. A missing manifest is not an error and
// yields an empty store without creating any files.
func Load(inst *instance.Instance) (*Store, error) {
	if inst == nil || strings.TrimSpace(inst.Path) == "" {
		return nil, errors.New("instância inválida: caminho vazio")
	}

	store := &Store{
		instancePath: inst.Path,
		path:         filepath.Join(inst.Path, StateDirName, FileName),
		doc: document{
			Schema:  SchemaVersion,
			Entries: []Entry{},
		},
	}

	info, err := os.Stat(store.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return store, nil
	case err != nil:
		return nil, instance.WrapFSError("acessar o manifesto de conteúdo instalado", store.path, err)
	case info.IsDir():
		return nil, fmt.Errorf("o manifesto %q é um diretório, mas deveria ser um arquivo", store.path)
	case info.Size() > maxFileBytes:
		return nil, fmt.Errorf("o manifesto %q excede o limite de %d bytes", store.path, maxFileBytes)
	}

	data, err := os.ReadFile(store.path)
	if err != nil {
		return nil, instance.WrapFSError("ler o manifesto de conteúdo instalado", store.path, err)
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("manifesto de conteúdo instalado inválido em %q: %w", store.path, err)
	}
	if doc.Schema != SchemaVersion {
		return nil, fmt.Errorf(
			"manifesto %q usa o esquema %d, mas esta versão suporta apenas o esquema %d",
			store.path,
			doc.Schema,
			SchemaVersion,
		)
	}

	entries, err := normalizeEntries(doc.Entries)
	if err != nil {
		return nil, fmt.Errorf("manifesto de conteúdo instalado inválido em %q: %w", store.path, err)
	}
	doc.Entries = entries
	store.doc = doc

	return store, nil
}

// Path returns the absolute path of the manifest file.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}

	return s.path
}

// Entries returns a sorted copy of the tracked entries.
func (s *Store) Entries() []Entry {
	if s == nil {
		return nil
	}

	entries := make([]Entry, 0, len(s.doc.Entries))
	for _, entry := range s.doc.Entries {
		entries = append(entries, cloneEntry(entry))
	}
	sortEntries(entries)

	return entries
}

// FindByProject returns every entry for the given provider and project ID.
func (s *Store) FindByProject(provider modsource.Provider, projectID string) []Entry {
	if s == nil {
		return nil
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil
	}

	var entries []Entry
	for _, entry := range s.doc.Entries {
		if entry.Provider == provider && entry.ProjectID == projectID {
			entries = append(entries, cloneEntry(entry))
		}
	}
	sortEntries(entries)

	return entries
}

// FindByPath returns the entry tracked at filePath, if any.
func (s *Store) FindByPath(filePath string) (Entry, bool) {
	if s == nil {
		return Entry{}, false
	}

	normalized, err := normalizeFilePath(filePath)
	if err != nil {
		return Entry{}, false
	}

	for _, entry := range s.doc.Entries {
		if PathsEqual(entry.FilePath, normalized) {
			return cloneEntry(entry), true
		}
	}

	return Entry{}, false
}

// ReplaceProject installs entry and removes any previous entry for the same
// provider and project. The manifest is written atomically before the
// in-memory state is updated.
func (s *Store) ReplaceProject(entry Entry) error {
	if s == nil {
		return errors.New("manifesto não inicializado")
	}

	normalized, err := normalizeEntry(entry)
	if err != nil {
		return err
	}

	for _, existing := range s.doc.Entries {
		if PathsEqual(existing.FilePath, normalized.FilePath) &&
			(existing.Provider != normalized.Provider || existing.ProjectID != normalized.ProjectID) {
			return fmt.Errorf(
				"o arquivo %q já está registrado para %s",
				normalized.FilePath,
				existing.ProjectLabel(),
			)
		}
	}

	entries := make([]Entry, 0, len(s.doc.Entries)+1)
	for _, existing := range s.doc.Entries {
		if existing.Provider == normalized.Provider && existing.ProjectID == normalized.ProjectID {
			continue
		}
		entries = append(entries, existing)
	}
	entries = append(entries, normalized)

	return s.commit(entries)
}

// RemoveByPath removes the entry tracked at filePath. It reports whether an
// entry was found. A missing entry is not an error.
func (s *Store) RemoveByPath(filePath string) (Entry, bool, error) {
	if s == nil {
		return Entry{}, false, errors.New("manifesto não inicializado")
	}

	normalized, err := normalizeFilePath(filePath)
	if err != nil {
		return Entry{}, false, err
	}

	entries := make([]Entry, 0, len(s.doc.Entries))
	var removed Entry
	found := false
	for _, entry := range s.doc.Entries {
		if PathsEqual(entry.FilePath, normalized) {
			removed = cloneEntry(entry)
			found = true
			continue
		}
		entries = append(entries, entry)
	}
	if !found {
		return Entry{}, false, nil
	}

	if err := s.commit(entries); err != nil {
		return Entry{}, false, err
	}

	return removed, true, nil
}

// RemoveProject removes every entry for the given provider and project.
func (s *Store) RemoveProject(provider modsource.Provider, projectID string) ([]Entry, error) {
	if s == nil {
		return nil, errors.New("manifesto não inicializado")
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("ID do projeto não informado")
	}

	entries := make([]Entry, 0, len(s.doc.Entries))
	var removed []Entry
	for _, entry := range s.doc.Entries {
		if entry.Provider == provider && entry.ProjectID == projectID {
			removed = append(removed, cloneEntry(entry))
			continue
		}
		entries = append(entries, entry)
	}
	if len(removed) == 0 {
		return nil, nil
	}

	if err := s.commit(entries); err != nil {
		return nil, err
	}

	return removed, nil
}

func (s *Store) commit(entries []Entry) error {
	normalized, err := normalizeEntries(entries)
	if err != nil {
		return err
	}

	doc := document{
		Schema:    SchemaVersion,
		UpdatedAt: time.Now().UTC(),
		Entries:   normalized,
	}
	if err := s.write(doc); err != nil {
		return err
	}
	s.doc = doc

	return nil
}

func (s *Store) write(doc document) (err error) {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return instance.WrapFSError("criar a pasta do manifesto", directory, err)
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar o manifesto: %w", err)
	}
	data = append(data, '\n')

	file, err := os.CreateTemp(directory, ".installed-*.tmp")
	if err != nil {
		return instance.WrapFSError("criar o arquivo temporário do manifesto em", directory, err)
	}
	tempPath := file.Name()
	closed := false
	removeTemp := true

	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				wrapped := instance.WrapFSError("fechar o arquivo temporário do manifesto", tempPath, closeErr)
				if err == nil {
					err = wrapped
				} else {
					err = errors.Join(err, wrapped)
				}
			}
		}
		if removeTemp {
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				wrapped := instance.WrapFSError("remover o arquivo temporário do manifesto", tempPath, removeErr)
				if err == nil {
					err = wrapped
				} else {
					err = errors.Join(err, wrapped)
				}
			}
		}
	}()

	if err := file.Chmod(0o644); err != nil {
		return instance.WrapFSError("ajustar as permissões do manifesto", tempPath, err)
	}
	if _, err := file.Write(data); err != nil {
		return instance.WrapFSError("gravar o manifesto", tempPath, err)
	}
	if err := file.Sync(); err != nil {
		return instance.WrapFSError("sincronizar o manifesto", tempPath, err)
	}
	if err := file.Close(); err != nil {
		return instance.WrapFSError("fechar o manifesto", tempPath, err)
	}
	closed = true

	if err := os.Rename(tempPath, s.path); err != nil {
		return instance.WrapFSError("substituir o manifesto", s.path, err)
	}
	removeTemp = false

	return nil
}

func normalizeEntries(entries []Entry) ([]Entry, error) {
	normalized := make([]Entry, 0, len(entries))
	paths := make(map[string]struct{}, len(entries))
	projects := make(map[string]struct{}, len(entries))

	for index, entry := range entries {
		item, err := normalizeEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("entrada %d: %w", index+1, err)
		}

		identity := pathIdentity(item.FilePath)
		if _, exists := paths[identity]; exists {
			return nil, fmt.Errorf("o arquivo %q está registrado mais de uma vez", item.FilePath)
		}
		paths[identity] = struct{}{}

		projectKey := string(item.Provider) + "\x00" + item.ProjectID
		if _, exists := projects[projectKey]; exists {
			return nil, fmt.Errorf(
				"o projeto %q está registrado mais de uma vez",
				item.ProjectLabel(),
			)
		}
		projects[projectKey] = struct{}{}

		normalized = append(normalized, item)
	}

	sortEntries(normalized)

	return normalized, nil
}

func normalizeEntry(entry Entry) (Entry, error) {
	entry.Provider = modsource.Provider(strings.TrimSpace(string(entry.Provider)))
	if !entry.Provider.Valid() {
		return Entry{}, fmt.Errorf("provedor inválido %q", entry.Provider)
	}

	entry.ProjectID = strings.TrimSpace(entry.ProjectID)
	if entry.ProjectID == "" {
		return Entry{}, errors.New("ID do projeto não informado")
	}
	entry.ProjectSlug = strings.TrimSpace(entry.ProjectSlug)
	entry.ProjectName = strings.TrimSpace(entry.ProjectName)

	if !entry.Type.Valid() {
		return Entry{}, fmt.Errorf("tipo de conteúdo inválido %q", entry.Type)
	}
	if _, ok := ContentDirectory(entry.Type); !ok {
		return Entry{}, fmt.Errorf("tipo de conteúdo não suportado no manifesto: %q", entry.Type)
	}

	entry.VersionID = strings.TrimSpace(entry.VersionID)
	if entry.VersionID == "" {
		return Entry{}, errors.New("ID da versão não informado")
	}
	entry.VersionNumber = strings.TrimSpace(entry.VersionNumber)
	entry.VersionName = strings.TrimSpace(entry.VersionName)
	entry.Channel = modsource.ReleaseChannel(strings.TrimSpace(string(entry.Channel)))
	if entry.Channel != "" {
		switch entry.Channel {
		case modsource.ChannelRelease, modsource.ChannelBeta, modsource.ChannelAlpha:
		default:
			return Entry{}, fmt.Errorf("canal inválido %q", entry.Channel)
		}
	}

	entry.GameVersion = strings.TrimSpace(entry.GameVersion)
	entry.Loader = strings.TrimSpace(entry.Loader)
	entry.GameVersions = cleanStrings(entry.GameVersions)
	entry.Loaders = cleanStrings(entry.Loaders)

	filePath, err := validateFilePath(entry.Type, entry.FilePath)
	if err != nil {
		return Entry{}, err
	}
	entry.FilePath = filePath

	entry.FileName = strings.TrimSpace(entry.FileName)
	if entry.FileName == "" {
		entry.FileName = path.Base(filePath)
	}
	if entry.FileName != path.Base(filePath) {
		return Entry{}, fmt.Errorf(
			"o nome do arquivo %q não corresponde ao caminho %q",
			entry.FileName,
			entry.FilePath,
		)
	}
	if _, err := download.SafeFilename(entry.FileName); err != nil {
		return Entry{}, err
	}

	if entry.FileSize < 0 {
		return Entry{}, errors.New("tamanho do arquivo não pode ser negativo")
	}

	if len(entry.Hashes) > 0 {
		hashes := make(map[string]string, len(entry.Hashes))
		for algorithm, value := range entry.Hashes {
			algorithm = strings.TrimSpace(algorithm)
			value = strings.TrimSpace(value)
			if algorithm == "" || value == "" {
				continue
			}
			hashes[algorithm] = value
		}
		if len(hashes) == 0 {
			entry.Hashes = nil
		} else {
			entry.Hashes = hashes
		}
	}

	return entry, nil
}

func validateFilePath(kind modsource.ProjectKind, filePath string) (string, error) {
	filePath, err := normalizeFilePath(filePath)
	if err != nil {
		return "", err
	}

	directory, ok := ContentDirectory(kind)
	if !ok {
		return "", fmt.Errorf("tipo de conteúdo não suportado no manifesto: %q", kind)
	}

	parts := strings.Split(filePath, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("o arquivo %q deve estar diretamente em %s/", filePath, directory)
	}
	if parts[0] != directory {
		return "", fmt.Errorf(
			"o arquivo %q não pertence à pasta %s/ do tipo %s",
			filePath,
			directory,
			kind,
		)
	}
	if _, err := download.SafeFilename(parts[1]); err != nil {
		return "", err
	}

	return filePath, nil
}

func normalizeFilePath(filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", errors.New("caminho do arquivo não informado")
	}

	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if path.IsAbs(filePath) {
		return "", fmt.Errorf("o caminho do arquivo deve ser relativo à instância: %q", filePath)
	}

	cleaned := path.Clean(filePath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("o caminho do arquivo escapa da instância: %q", filePath)
	}

	return cleaned, nil
}

func cloneEntry(entry Entry) Entry {
	clone := entry
	clone.GameVersions = append([]string(nil), entry.GameVersions...)
	clone.Loaders = append([]string(nil), entry.Loaders...)
	if entry.Hashes != nil {
		clone.Hashes = make(map[string]string, len(entry.Hashes))
		for algorithm, value := range entry.Hashes {
			clone.Hashes[algorithm] = value
		}
	}

	return clone
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].FilePath != entries[j].FilePath {
			return entries[i].FilePath < entries[j].FilePath
		}
		if entries[i].Provider != entries[j].Provider {
			return entries[i].Provider < entries[j].Provider
		}

		return entries[i].ProjectID < entries[j].ProjectID
	})
}

func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}

	return cleaned
}
