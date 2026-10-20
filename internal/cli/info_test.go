package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

func executeInfoCommand(t *testing.T, client modsource.Client, args ...string) (string, string, error) {
	return executeCommand(t, newInfoCommand(client), args...)
}

func TestInfoCommandShowsProject(t *testing.T) {
	client := &fakeProviderClient{
		project: modsource.Project{
			Provider:    modsource.ProviderModrinth,
			ID:          "AANobbMI",
			Slug:        "sodium",
			Name:        "Sodium",
			Summary:     "Um motor de renderização",
			Description: "Descrição completa\ncom markdown",
			Authors:     []string{"jellysquid3"},
			License:     "lgpl-3",
			Categories:  []string{"optimization"},
			GameVersions: []string{
				"1.20.1",
				"1.21",
			},
			Loaders:   []string{"fabric", "neoforge"},
			Downloads: 12345,
			Followers: 678,
			Type:      modsource.KindMod,
			Links: modsource.Links{
				Website: "https://modrinth.com/mod/sodium",
				Source:  "https://github.com/example/sodium",
				Issues:  "https://github.com/example/sodium/issues",
				Wiki:    "https://example.com/wiki",
				Discord: "https://example.com/discord",
			},
			CreatedAt: time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt: time.Date(2025, time.February, 3, 4, 5, 6, 0, time.UTC),
		},
	}

	out, errOut, err := executeInfoCommand(t, client, " sodium ")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
	if client.projectCalls != 1 {
		t.Fatalf("projectCalls = %d, want 1", client.projectCalls)
	}
	if client.projectID != "sodium" {
		t.Fatalf("projectID = %q, want sodium", client.projectID)
	}

	for _, fragment := range []string{
		"Sodium",
		"Tipo: Mod",
		"Provedor: Modrinth",
		"ID: AANobbMI",
		"Slug: sodium",
		"Autores: jellysquid3",
		"Licença: lgpl-3",
		"Downloads: 12.3K",
		"Seguidores: 678",
		"Criado em: 2020-01-02",
		"Atualizado em: 2025-02-03",
		"Categorias: optimization",
		"Loaders: fabric, neoforge",
		"Versões do jogo: 1.20.1, 1.21",
		"Resumo:\nUm motor de renderização",
		"Links:",
		"Site: https://modrinth.com/mod/sodium",
		"Código: https://github.com/example/sodium",
		"Issues: https://github.com/example/sodium/issues",
		"Wiki: https://example.com/wiki",
		"Discord: https://example.com/discord",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("output = %q, missing %q", out, fragment)
		}
	}

	if strings.Contains(out, "Descrição completa") {
		t.Errorf("output = %q, should not include the full description without --description", out)
	}
}

func TestInfoCommandShowsFullDescription(t *testing.T) {
	client := &fakeProviderClient{
		project: modsource.Project{
			Name:        "Sodium",
			Type:        modsource.KindMod,
			Description: "Linha 1\nLinha 2",
		},
	}

	out, _, err := executeInfoCommand(t, client, "sodium", "--description")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Descrição completa:\nLinha 1\nLinha 2") {
		t.Errorf("output = %q, missing full description", out)
	}
}

func TestInfoCommandMinimalProject(t *testing.T) {
	client := &fakeProviderClient{project: modsource.Project{}}

	out, _, err := executeInfoCommand(t, client, "projeto")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "(sem nome)") {
		t.Errorf("output = %q, missing fallback name", out)
	}
	if strings.Contains(out, "Links:") {
		t.Errorf("output = %q, should not include empty links", out)
	}
	if strings.Contains(out, "Criado em:") {
		t.Errorf("output = %q, should not include zero dates", out)
	}
}

func TestInfoCommandRequiresProjectID(t *testing.T) {
	client := &fakeProviderClient{}

	_, _, err := executeInfoCommand(t, client, "  ")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "informe o slug ou ID") {
		t.Fatalf("Execute() error = %q", err)
	}
	if client.projectCalls != 0 {
		t.Fatalf("projectCalls = %d, want 0", client.projectCalls)
	}
}

func TestInfoCommandReturnsAPIError(t *testing.T) {
	client := &fakeProviderClient{
		projectErr: &modsource.APIError{
			Provider:   modsource.ProviderModrinth,
			Operation:  "obter projeto",
			StatusCode: 404,
			Message:    "projeto não encontrado",
			Err:        modsource.ErrNotFound,
		},
	}

	out, _, err := executeInfoCommand(t, client, "ausente")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}
	if !errors.Is(err, modsource.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "falha ao obter o projeto") {
		t.Fatalf("Execute() error = %q, want context", err)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty", out)
	}
}

func TestInfoCommandProviderUnavailable(t *testing.T) {
	_, _, err := executeInfoCommand(t, nil, "sodium", "--provider", "curseforge")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "CurseForge ainda não está implementado") {
		t.Fatalf("Execute() error = %q", err)
	}
}
