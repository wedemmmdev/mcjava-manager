package modsource

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseProvider(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Provider
		wantErr bool
	}{
		{name: "modrinth", input: "modrinth", want: ProviderModrinth},
		{name: "modrinth alias and spaces", input: " MR ", want: ProviderModrinth},
		{name: "curseforge", input: "curseforge", want: ProviderCurseForge},
		{name: "curseforge hyphen", input: "Curse-Forge", want: ProviderCurseForge},
		{name: "curseforge alias", input: "cf", want: ProviderCurseForge},
		{name: "unknown", input: "unknown", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProvider(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseProvider() error = nil, want an error")
				}
				if got != "" {
					t.Fatalf("ParseProvider() = %q, want empty provider", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseProvider() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseProvider() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderMetadata(t *testing.T) {
	if !ProviderCurseForge.RequiresAPIKey() {
		t.Error("CurseForge must require an API key")
	}
	if ProviderModrinth.RequiresAPIKey() {
		t.Error("Modrinth must not require an API key")
	}
	if ProviderModrinth.DisplayName() != "Modrinth" {
		t.Errorf("Modrinth DisplayName() = %q", ProviderModrinth.DisplayName())
	}
	if ProviderCurseForge.APIKeyURL() == "" {
		t.Error("CurseForge APIKeyURL() must not be empty")
	}
	if Provider("invalid").Valid() {
		t.Error("unknown provider must be invalid")
	}
}

func TestFileDownloadable(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "direct URL", url: "https://example.com/mod.jar", want: true},
		{name: "empty URL", url: "", want: false},
		{name: "whitespace URL", url: "   ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := File{URL: tt.url}
			if got := file.Downloadable(); got != tt.want {
				t.Fatalf("Downloadable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionPrimaryFile(t *testing.T) {
	primary := File{Name: "primary.jar", URL: "https://example.com/primary.jar", Primary: true}
	alternate := File{Name: "alternate.jar", URL: "https://example.com/alternate.jar"}
	blocked := File{Name: "blocked.jar", Primary: true}

	tests := []struct {
		name    string
		version Version
		want    string
		wantErr error
	}{
		{
			name:    "prefers primary downloadable file",
			version: Version{ID: "v1", Files: []File{alternate, primary}},
			want:    primary.Name,
		},
		{
			name:    "falls back to any downloadable file",
			version: Version{ID: "v1", Files: []File{blocked, alternate}},
			want:    alternate.Name,
		},
		{
			name:    "no files",
			version: Version{ID: "v1"},
			wantErr: ErrNoFiles,
		},
		{
			name:    "no downloadable files",
			version: Version{ID: "v1", Files: []File{blocked}},
			wantErr: ErrDownloadUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.version.PrimaryFile()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PrimaryFile() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("PrimaryFile() error = %v", err)
			}
			if got.Name != tt.want {
				t.Fatalf("PrimaryFile() = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestProjectRef(t *testing.T) {
	project := Project{
		Provider: ProviderModrinth,
		ID:       "abc123",
		Slug:     "sodium",
		Name:     "Sodium",
	}

	ref := project.Ref()
	if ref.Provider != project.Provider || ref.ID != project.ID || ref.Slug != project.Slug || ref.Name != project.Name {
		t.Fatalf("Project.Ref() = %+v, want fields copied from %+v", ref, project)
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		Provider:   ProviderCurseForge,
		Operation:  "buscar projeto",
		StatusCode: 429,
		Message:    "too many requests",
		RetryAfter: 2 * time.Second,
		Err:        ErrRateLimited,
	}

	message := err.Error()
	for _, fragment := range []string{"CurseForge", "buscar projeto", "HTTP 429", "too many requests", "2s"} {
		if !strings.Contains(message, fragment) {
			t.Errorf("APIError.Error() = %q, missing %q", message, fragment)
		}
	}

	if !errors.Is(err, ErrRateLimited) {
		t.Error("APIError must unwrap to ErrRateLimited")
	}
}
