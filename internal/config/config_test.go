package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func writeConfigFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	return path
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("DefaultPath() = %q, want an absolute path", path)
	}
	if filepath.Base(path) != FileName {
		t.Fatalf("DefaultPath() = %q, want base %q", path, FileName)
	}
}

func TestLoadMissingDefaultIsNotAnError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Path != filepath.Join(home, FileName) {
		t.Fatalf("Load().Path = %q, want %q", cfg.Path, filepath.Join(home, FileName))
	}
	if _, err := os.Stat(cfg.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("default config should not be created automatically, Stat() error = %v", err)
	}
}

func TestLoadExplicitMissingFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadValidConfig(t *testing.T) {
	path := writeConfigFile(t, "curseforge:\n  api_key: \"  file-key  \"\n", 0o600)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Path != path {
		t.Fatalf("Load().Path = %q, want %q", cfg.Path, path)
	}
	if got := cfg.CurseForge.APIKey.Value(); got != "file-key" {
		t.Fatalf("APIKey.Value() = %q, want file-key", got)
	}
	if got := fmt.Sprint(cfg.CurseForge.APIKey); got != "[REDACTED]" {
		t.Fatalf("formatted APIKey = %q, want [REDACTED]", got)
	}
}

func TestLoadEmptyAndCommentOnlyFiles(t *testing.T) {
	for _, content := range []string{"", "# apenas um comentário\n", "---\n"} {
		t.Run(fmt.Sprintf("%q", content), func(t *testing.T) {
			path := writeConfigFile(t, content, 0o600)

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.CurseForge.APIKey.IsSet() {
				t.Fatal("API key should be empty")
			}
		})
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeConfigFile(t, "curseforge:\n  api_kye: value\n", 0o600)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an unknown field error")
	}
	if !strings.Contains(err.Error(), "api_kye") {
		t.Fatalf("Load() error = %v, want it to mention api_kye", err)
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	path := writeConfigFile(t, "curseforge: {}\n---\ncurseforge: {}\n", 0o600)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want a multiple documents error")
	}
	if !strings.Contains(err.Error(), "múltiplos documentos") {
		t.Fatalf("Load() error = %v, want a multiple documents error", err)
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	path := writeConfigFile(t, strings.Repeat("#", maxConfigSize+1), 0o600)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an oversized file error")
	}
	if !strings.Contains(err.Error(), "excede o limite") {
		t.Fatalf("Load() error = %v, want an oversized file error", err)
	}
}

func TestCurseForgeAPIKeyPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		file    string
		want    string
		wantErr bool
	}{
		{name: "environment wins", env: "env-key", file: "file-key", want: "env-key"},
		{name: "file fallback", env: "", file: "file-key", want: "file-key"},
		{name: "whitespace environment is ignored", env: "   ", file: "file-key", want: "file-key"},
		{name: "missing", env: "", file: "", wantErr: true},
		{name: "whitespace file is ignored", env: "", file: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvCurseForgeAPIKey, tt.env)

			cfg := &Config{
				Path:       "/tmp/config.yaml",
				CurseForge: CurseForgeConfig{APIKey: NewSecret(tt.file)},
			}

			key, err := cfg.CurseForgeAPIKey()
			if tt.wantErr {
				if !errors.Is(err, modsource.ErrMissingCredentials) {
					t.Fatalf("CurseForgeAPIKey() error = %v, want ErrMissingCredentials", err)
				}

				for _, fragment := range []string{EnvCurseForgeAPIKey, cfg.Path, "console.curseforge.com"} {
					if !strings.Contains(err.Error(), fragment) {
						t.Errorf("error message = %q, missing %q", err.Error(), fragment)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("CurseForgeAPIKey() error = %v", err)
			}
			if key.Value() != tt.want {
				t.Fatalf("CurseForgeAPIKey() = %q, want %q", key.Value(), tt.want)
			}
		})
	}
}

func TestCurseForgeAPIKeyOnNilConfig(t *testing.T) {
	t.Setenv(EnvCurseForgeAPIKey, "")

	var cfg *Config
	key, err := cfg.CurseForgeAPIKey()
	if err == nil {
		t.Fatal("CurseForgeAPIKey() error = nil, want an error")
	}
	if !errors.Is(err, modsource.ErrMissingCredentials) {
		t.Fatalf("CurseForgeAPIKey() error = %v, want ErrMissingCredentials", err)
	}
	if key.IsSet() {
		t.Fatal("key should be empty")
	}
}

func TestInsecurePermissionsWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions are not meaningful on Windows")
	}

	path := writeConfigFile(t, "curseforge:\n  api_key: file-key\n", 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Warnings) == 0 {
		t.Fatal("Load() should warn about group/other-readable permissions")
	}
	if !strings.Contains(cfg.Warnings[0], "chmod 600") {
		t.Fatalf("warning = %q, want chmod 600 guidance", cfg.Warnings[0])
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("Warnings = %v, want none for 0600 permissions", cfg.Warnings)
	}
}

func TestSecretRedaction(t *testing.T) {
	secret := NewSecret("  super-secret  ")

	if secret.Value() != "super-secret" {
		t.Fatalf("Value() = %q, want super-secret", secret.Value())
	}
	if !secret.IsSet() {
		t.Fatal("IsSet() = false, want true")
	}

	formatted := fmt.Sprintf("%s|%v|%#v", secret, secret, secret)
	if strings.Contains(formatted, "super-secret") {
		t.Fatalf("formatted secret leaked the raw value: %q", formatted)
	}

	marshaled, err := yaml.Marshal(struct {
		Key Secret `yaml:"key"`
	}{Key: secret})
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if strings.Contains(string(marshaled), "super-secret") {
		t.Fatalf("marshaled secret leaked the raw value: %s", marshaled)
	}
	if !strings.Contains(string(marshaled), "[REDACTED]") {
		t.Fatalf("marshaled secret = %s, want [REDACTED]", marshaled)
	}

	jsonData, err := json.Marshal(struct {
		Key Secret `json:"key"`
	}{Key: secret})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(jsonData), "super-secret") {
		t.Fatalf("JSON secret leaked the raw value: %s", jsonData)
	}
	if !strings.Contains(string(jsonData), "[REDACTED]") {
		t.Fatalf("JSON secret = %s, want [REDACTED]", jsonData)
	}

	textData, err := secret.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if strings.Contains(string(textData), "super-secret") {
		t.Fatalf("text secret leaked the raw value: %s", textData)
	}
	if string(textData) != "[REDACTED]" {
		t.Fatalf("MarshalText() = %q, want [REDACTED]", textData)
	}
}

func TestSecretEmpty(t *testing.T) {
	secret := NewSecret("   ")

	if secret.IsSet() {
		t.Fatal("IsSet() = true, want false")
	}
	if secret.String() != "" {
		t.Fatalf("String() = %q, want empty", secret.String())
	}

	marshaled, err := yaml.Marshal(struct {
		Key Secret `yaml:"key"`
	}{Key: secret})
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if !strings.Contains(string(marshaled), `key: ""`) {
		t.Fatalf("marshaled empty secret = %q, want an empty value", marshaled)
	}
}

func TestMissingAPIKeyErrorMessage(t *testing.T) {
	err := &MissingAPIKeyError{ConfigPath: "/home/user/.mcjava-manager.yaml"}

	for _, fragment := range []string{
		EnvCurseForgeAPIKey,
		"/home/user/.mcjava-manager.yaml",
		"curseforge:",
		"api_key:",
		"https://console.curseforge.com/",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("MissingAPIKeyError = %q, missing %q", err.Error(), fragment)
		}
	}

	if !errors.Is(err, modsource.ErrMissingCredentials) {
		t.Error("MissingAPIKeyError must unwrap to ErrMissingCredentials")
	}
}
