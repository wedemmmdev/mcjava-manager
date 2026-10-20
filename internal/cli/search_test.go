package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

func executeSearchCommand(t *testing.T, client modsource.Client, args ...string) (string, string, error) {
	return executeCommand(t, newSearchCommand(client), args...)
}

func TestSearchCommandBuildsQuery(t *testing.T) {
	client := &fakeProviderClient{
		result: modsource.SearchResult{
			Total:  12,
			Offset: 10,
			Limit:  2,
			Projects: []modsource.Project{
				{
					Name:      "Sodium",
					Type:      modsource.KindMod,
					Authors:   []string{"jellysquid3"},
					Downloads: 12345,
					Slug:      "sodium",
				},
				{
					Name:      "Shader sem autor",
					Type:      modsource.KindShaderPack,
					Downloads: 2_000_000,
					ID:        "abc",
				},
			},
		},
	}

	out, errOut, err := executeSearchCommand(t, client,
		" sodium ",
		"--type", "mod",
		"--loader", " fabric ",
		"--game-version", " 1.20.1 ",
		"--category", "optimization",
		"--category", "performance",
		"--sort", "downloads",
		"--limit", "5",
		"--offset", "10",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}

	wantQuery := modsource.SearchQuery{
		Text:        "sodium",
		Kind:        modsource.KindMod,
		GameVersion: "1.20.1",
		Loader:      "fabric",
		Categories:  []string{"optimization", "performance"},
		Sort:        modsource.SortDownloads,
		Limit:       5,
		Offset:      10,
	}
	if !reflect.DeepEqual(client.query, wantQuery) {
		t.Fatalf("query = %#v, want %#v", client.query, wantQuery)
	}
	if client.searchCalls != 1 {
		t.Fatalf("searchCalls = %d, want 1", client.searchCalls)
	}

	for _, fragment := range []string{
		`Modrinth: 12 resultados para "sodium"`,
		"NOME",
		"Sodium",
		"jellysquid3",
		"12.3K",
		"Shader",
		"2M",
		"abc",
		"Exibindo 11-12 de 12.",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("output = %q, missing %q", out, fragment)
		}
	}
}

func TestSearchCommandDefaults(t *testing.T) {
	client := &fakeProviderClient{
		result: modsource.SearchResult{Total: 0, Offset: 0, Limit: 20},
	}

	if _, _, err := executeSearchCommand(t, client, "sodium"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if client.query.Text != "sodium" {
		t.Errorf("Text = %q, want sodium", client.query.Text)
	}
	if client.query.Kind != "" {
		t.Errorf("Kind = %q, want empty", client.query.Kind)
	}
	if client.query.GameVersion != "" {
		t.Errorf("GameVersion = %q, want empty", client.query.GameVersion)
	}
	if client.query.Loader != "" {
		t.Errorf("Loader = %q, want empty", client.query.Loader)
	}
	if len(client.query.Categories) != 0 {
		t.Errorf("Categories = %#v, want empty", client.query.Categories)
	}
	if client.query.Sort != modsource.SortRelevance {
		t.Errorf("Sort = %q, want %q", client.query.Sort, modsource.SortRelevance)
	}
	if client.query.Limit != 20 {
		t.Errorf("Limit = %d, want 20", client.query.Limit)
	}
	if client.query.Offset != 0 {
		t.Errorf("Offset = %d, want 0", client.query.Offset)
	}
}

func TestSearchCommandAcceptsCommaSeparatedCategories(t *testing.T) {
	client := &fakeProviderClient{}

	if _, _, err := executeSearchCommand(t, client, "sodium", "--category", "optimization,performance"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := []string{"optimization", "performance"}
	if !reflect.DeepEqual(client.query.Categories, want) {
		t.Fatalf("Categories = %#v, want %#v", client.query.Categories, want)
	}
}

func TestSearchCommandShowsNextPage(t *testing.T) {
	client := &fakeProviderClient{
		result: modsource.SearchResult{
			Total:  20,
			Offset: 10,
			Limit:  2,
			Projects: []modsource.Project{
				{Name: "Sodium", Type: modsource.KindMod, Slug: "sodium"},
				{Name: "Lithium", Type: modsource.KindMod, Slug: "lithium"},
			},
		},
	}

	out, _, err := executeSearchCommand(t, client, "fabric", "--offset", "10", "--limit", "2")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Exibindo 11-12 de 20.") {
		t.Errorf("output = %q, missing page summary", out)
	}
	if !strings.Contains(out, "Próxima página: --offset 12") {
		t.Errorf("output = %q, missing next page hint", out)
	}
}

func TestSearchCommandNoResults(t *testing.T) {
	client := &fakeProviderClient{
		result: modsource.SearchResult{Total: 0, Offset: 0, Limit: 20},
	}

	out, _, err := executeSearchCommand(t, client, "sodium")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Nenhum projeto encontrado.") {
		t.Errorf("output = %q, missing empty result message", out)
	}
	if strings.Contains(out, "NOME") {
		t.Errorf("output = %q, should not contain a table for empty results", out)
	}
}

func TestSearchCommandRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name   string
		client modsource.Client
		args   []string
		want   string
	}{
		{
			name:   "tipo inválido",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--type", "invalido"},
			want:   "tipo de projeto inválido",
		},
		{
			name:   "ordenação inválida",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--sort", "invalida"},
			want:   "ordenação inválida",
		},
		{
			name:   "limite zero",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--limit", "0"},
			want:   "--limit deve estar entre 1 e 100",
		},
		{
			name:   "limite acima do máximo",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--limit", "101"},
			want:   "--limit deve estar entre 1 e 100",
		},
		{
			name:   "offset negativo",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--offset", "-1"},
			want:   "--offset não pode ser negativo",
		},
		{
			name:   "provedor desconhecido",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--provider", "invalido"},
			want:   "provedor desconhecido",
		},
		{
			name:   "provedor não implementado",
			client: nil,
			args:   []string{"sodium", "--provider", "curseforge"},
			want:   "CurseForge ainda não está implementado",
		},
		{
			name:   "cliente de outro provedor",
			client: &fakeProviderClient{provider: modsource.ProviderModrinth},
			args:   []string{"sodium", "--provider", "curseforge"},
			want:   "cliente disponível",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake, _ := test.client.(*fakeProviderClient)

			_, _, err := executeSearchCommand(t, test.client, test.args...)
			if err == nil {
				t.Fatal("Execute() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %q, want it to contain %q", err, test.want)
			}
			if fake != nil && fake.searchCalls != 0 {
				t.Fatalf("searchCalls = %d, want 0", fake.searchCalls)
			}
		})
	}
}

func TestSearchCommandReturnsAPIError(t *testing.T) {
	client := &fakeProviderClient{
		err: &modsource.APIError{
			Provider:   modsource.ProviderModrinth,
			Operation:  "buscar projetos",
			StatusCode: 429,
			Message:    "aguarde",
			RetryAfter: 3 * time.Second,
			Err:        modsource.ErrRateLimited,
		},
	}

	out, _, err := executeSearchCommand(t, client, "sodium")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}
	if !errors.Is(err, modsource.ErrRateLimited) {
		t.Fatalf("Execute() error = %v, want ErrRateLimited", err)
	}
	if !strings.Contains(err.Error(), "falha ao buscar projetos") {
		t.Fatalf("Execute() error = %q, want context", err)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty", out)
	}
}

func TestNewProviderClient(t *testing.T) {
	client, err := newProviderClient(modsource.ProviderModrinth)
	if err != nil {
		t.Fatalf("newProviderClient(modrinth) error = %v", err)
	}
	if client.Provider() != modsource.ProviderModrinth {
		t.Fatalf("Provider() = %q, want modrinth", client.Provider())
	}

	if _, err := newProviderClient(modsource.ProviderCurseForge); err == nil {
		t.Fatal("newProviderClient(curseforge) error = nil, want an error")
	}
	if _, err := newProviderClient(modsource.Provider("desconhecido")); err == nil {
		t.Fatal("newProviderClient(desconhecido) error = nil, want an error")
	}
}

func TestResolveProviderClient(t *testing.T) {
	injected := &fakeProviderClient{provider: modsource.ProviderModrinth}

	client, err := resolveProviderClient(injected, "modrinth")
	if err != nil {
		t.Fatalf("resolveProviderClient() error = %v", err)
	}
	if client != injected {
		t.Fatal("resolveProviderClient() should return the injected client")
	}

	if _, err := resolveProviderClient(injected, "curseforge"); err == nil {
		t.Fatal("resolveProviderClient() with a different provider should fail")
	}
	if _, err := resolveProviderClient(nil, "curseforge"); err == nil {
		t.Fatal("resolveProviderClient(curseforge) should fail while the adapter is unavailable")
	}
	if _, err := resolveProviderClient(nil, "desconhecido"); err == nil {
		t.Fatal("resolveProviderClient(desconhecido) should fail")
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		value int64
		want  string
	}{
		{value: -1, want: "0"},
		{value: 0, want: "0"},
		{value: 999, want: "999"},
		{value: 1_000, want: "1K"},
		{value: 12_345, want: "12.3K"},
		{value: 1_000_000, want: "1M"},
		{value: 2_500_000, want: "2.5M"},
		{value: 1_000_000_000, want: "1B"},
	}

	for _, test := range tests {
		if got := formatCount(test.value); got != test.want {
			t.Errorf("formatCount(%d) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		value string
		max   int
		want  string
	}{
		{value: "Sodium", max: 10, want: "Sodium"},
		{value: "Sodium", max: 5, want: "Sodi…"},
		{value: "Sodium", max: 1, want: "S"},
		{value: "áéíóú", max: 3, want: "áé…"},
		{value: "  Sodium  ", max: 10, want: "Sodium"},
		{value: "Sodium", max: 0, want: ""},
	}

	for _, test := range tests {
		if got := truncate(test.value, test.max); got != test.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", test.value, test.max, got, test.want)
		}
	}
}

func TestAuthorsText(t *testing.T) {
	tests := []struct {
		name    string
		authors []string
		want    string
	}{
		{name: "nenhum", authors: nil, want: "-"},
		{name: "vazios", authors: []string{"  ", ""}, want: "-"},
		{name: "um", authors: []string{" jellysquid3 "}, want: "jellysquid3"},
		{name: "vários", authors: []string{"a", "b"}, want: "a, b"},
		{name: "controle removido", authors: []string{"a\x1bb"}, want: "ab"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := authorsText(test.authors); got != test.want {
				t.Errorf("authorsText(%#v) = %q, want %q", test.authors, got, test.want)
			}
		})
	}
}

func TestSanitizeText(t *testing.T) {
	got := sanitizeText("a\x1bb\nc\td\x7f\u202e\u2028e\u200ef")
	want := "ab c d ef"

	if got != want {
		t.Fatalf("sanitizeText() = %q, want %q", got, want)
	}
	if strings.ContainsRune(got, '\x1b') {
		t.Fatal("sanitizeText() kept an escape character")
	}
	if strings.ContainsRune(got, '\u2028') || strings.ContainsRune(got, '\u200e') {
		t.Fatal("sanitizeText() kept a line separator or bidi mark")
	}
}

func TestSanitizeBlock(t *testing.T) {
	got := sanitizeBlock("a\x1bb\nc\td\x7f\u202e\u2028e\u200ef")
	want := "ab\nc\td\nef"

	if got != want {
		t.Fatalf("sanitizeBlock() = %q, want %q", got, want)
	}
	if strings.ContainsRune(got, '\x1b') {
		t.Fatal("sanitizeBlock() kept an escape character")
	}
	if strings.ContainsRune(got, '\u200e') {
		t.Fatal("sanitizeBlock() kept a bidi mark")
	}
}
