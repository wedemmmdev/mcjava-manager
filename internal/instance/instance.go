// Package instance validates Minecraft instance directories and prepares the
// folder layout used to install mods, shader packs, resource packs, and worlds.
//
// No validation depends on mod loader names or folder naming conventions: any
// existing directory is a valid instance.
package instance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ModsDirName is the folder that receives mods.
	ModsDirName = "mods"

	// ShaderPacksDirName is the folder that receives shader packs.
	ShaderPacksDirName = "shaderpacks"

	// ResourcePacksDirName is the folder that receives resource packs.
	ResourcePacksDirName = "resourcepacks"

	// SavesDirName is the folder that receives worlds and maps.
	SavesDirName = "saves"
)

var (
	// ErrInstanceNotFound indicates that the instance directory does not exist.
	ErrInstanceNotFound = errors.New("diretório da instância não encontrado")

	// ErrNotDirectory indicates that the instance path exists but is not a
	// directory.
	ErrNotDirectory = errors.New("o caminho da instância não é um diretório")
)

// Instance represents an existing Minecraft instance directory.
type Instance struct {
	Path string
}

// Directories contains the resolved paths of the instance content folders.
type Directories struct {
	Mods          string
	ShaderPacks   string
	ResourcePacks string
	Saves         string
}

// RequiredDirectoryNames returns the content folders created inside an
// instance. A new slice is returned on every call.
func RequiredDirectoryNames() []string {
	return []string{
		ModsDirName,
		ShaderPacksDirName,
		ResourcePacksDirName,
		SavesDirName,
	}
}

// Open validates that path exists and is a directory, then returns an
// Instance. It does not create or modify anything. The path is normalized to
// absolute; a leading ~ must be expanded by the caller (the CLI does this).
func Open(path string) (*Instance, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("o caminho da instância não foi informado")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("falha ao resolver o caminho da instância %q: %w", path, err)
	}
	absolute = filepath.Clean(absolute)

	info, err := os.Stat(absolute)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrInstanceNotFound, absolute)
		}

		return nil, wrapFSError("acessar o diretório da instância", absolute, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %q", ErrNotDirectory, absolute)
	}

	return &Instance{Path: absolute}, nil
}

// Prepare validates the instance and creates any missing content folders.
func Prepare(path string) (*Instance, error) {
	inst, err := Open(path)
	if err != nil {
		return nil, err
	}

	if err := inst.EnsureLayout(); err != nil {
		return nil, err
	}

	return inst, nil
}

// EnsureLayout creates every required content folder that does not exist.
// Existing folders and their contents are preserved.
func (i *Instance) EnsureLayout() error {
	if i == nil || strings.TrimSpace(i.Path) == "" {
		return errors.New("instância inválida: caminho vazio")
	}

	for _, name := range RequiredDirectoryNames() {
		path := filepath.Join(i.Path, name)
		if err := ensureDirectory(path); err != nil {
			return err
		}
	}

	return nil
}

// Directories returns the resolved content folder paths.
func (i *Instance) Directories() Directories {
	if i == nil || strings.TrimSpace(i.Path) == "" {
		return Directories{}
	}

	return Directories{
		Mods:          filepath.Join(i.Path, ModsDirName),
		ShaderPacks:   filepath.Join(i.Path, ShaderPacksDirName),
		ResourcePacks: filepath.Join(i.Path, ResourcePacksDirName),
		Saves:         filepath.Join(i.Path, SavesDirName),
	}
}

func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if !info.IsDir() {
			return fmt.Errorf("o caminho %q existe, mas não é um diretório", path)
		}

		return nil
	case errors.Is(err, fs.ErrNotExist):
		if mkdirErr := os.MkdirAll(path, 0o755); mkdirErr != nil {
			return wrapFSError("criar a pasta", path, mkdirErr)
		}

		return nil
	default:
		return wrapFSError("acessar a pasta", path, err)
	}
}
