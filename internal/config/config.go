// Package config loads local settings and resolves provider credentials.
//
// The package never prints or serializes raw credentials: API keys are wrapped
// in Secret, which redacts itself when formatted or marshaled.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// FileName is the default local configuration file name.
	FileName = ".mcjava-manager.yaml"

	// EnvCurseForgeAPIKey is the environment variable checked before the
	// configuration file when resolving CurseForge credentials.
	EnvCurseForgeAPIKey = "CURSEFORGE_API_KEY"

	maxConfigSize = 1 << 20 // 1 MiB
)

// Config is the local configuration loaded from .mcjava-manager.yaml.
type Config struct {
	CurseForge CurseForgeConfig `yaml:"curseforge"`

	// Path is the resolved configuration file path. It is not serialized.
	Path string `yaml:"-"`

	// Warnings contains non-fatal security or environment notices, such as an
	// overly permissive configuration file.
	Warnings []string `yaml:"-"`
}

// CurseForgeConfig contains CurseForge-specific settings.
type CurseForgeConfig struct {
	APIKey Secret `yaml:"api_key"`
}

// DefaultPath returns the default configuration file path in the user's home
// directory.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("falha ao localizar o diretório home: %w", err)
	}

	return filepath.Join(home, FileName), nil
}

// Load reads the configuration file at path. When path is empty, the default
// path is used and a missing file is not an error. An explicitly provided path
// must exist. The path is normalized to absolute; a leading ~ must be expanded
// by the caller (the CLI does this).
func Load(path string) (*Config, error) {
	resolvedPath, explicit, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{Path: resolvedPath}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !explicit {
			return cfg, nil
		}

		return nil, fmt.Errorf("falha ao ler o arquivo de configuração %q: %w", resolvedPath, err)
	}

	if len(data) > maxConfigSize {
		return nil, fmt.Errorf("arquivo de configuração %q excede o limite de %d bytes", resolvedPath, maxConfigSize)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return cfg, nil
		}

		return nil, fmt.Errorf("falha ao interpretar o arquivo de configuração %q: %w", resolvedPath, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("arquivo de configuração %q contém múltiplos documentos YAML", resolvedPath)
		}

		return nil, fmt.Errorf("falha ao interpretar o arquivo de configuração %q: %w", resolvedPath, err)
	}

	cfg.CurseForge.APIKey = NewSecret(cfg.CurseForge.APIKey.Value())

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao inspecionar o arquivo de configuração %q: %w", resolvedPath, err)
	}
	if warning := insecurePermissionsWarning(resolvedPath, info); warning != "" {
		cfg.Warnings = append(cfg.Warnings, warning)
	}

	return cfg, nil
}

// CurseForgeAPIKey resolves the CurseForge API key using the documented
// precedence: CURSEFORGE_API_KEY first, then the configuration file.
func (c *Config) CurseForgeAPIKey() (Secret, error) {
	if key := NewSecret(os.Getenv(EnvCurseForgeAPIKey)); key.IsSet() {
		return key, nil
	}

	if c != nil && c.CurseForge.APIKey.IsSet() {
		return c.CurseForge.APIKey, nil
	}

	return "", c.missingCurseForgeKeyError()
}

func (c *Config) missingCurseForgeKeyError() error {
	path := ""
	if c != nil {
		path = c.Path
	}
	if path == "" {
		if defaultPath, err := DefaultPath(); err == nil {
			path = defaultPath
		}
	}
	if path == "" {
		path = "~/" + FileName
	}

	return &MissingAPIKeyError{ConfigPath: path}
}

func resolvePath(path string) (string, bool, error) {
	explicit := strings.TrimSpace(path) != ""
	if !explicit {
		defaultPath, err := DefaultPath()
		if err != nil {
			return "", false, err
		}
		path = defaultPath
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", explicit, fmt.Errorf("falha ao resolver o caminho da configuração %q: %w", path, err)
	}

	return filepath.Clean(absolute), explicit, nil
}

func insecurePermissionsWarning(path string, info fs.FileInfo) string {
	if runtime.GOOS == "windows" {
		return ""
	}

	if info.Mode().Perm()&0o077 == 0 {
		return ""
	}

	return fmt.Sprintf("o arquivo de configuração %q pode ser lido por outros usuários; restrinja o acesso com: chmod 600 %q", path, path)
}
