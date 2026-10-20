package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func executeVersionsCommand(t *testing.T, client modsource.Client, args ...string) (string, string, error) {
	return executeCommand(t, newVersionsCommand(client), args...)
}

func TestVersionsCommandListsAndPaginates(t *testing.T) {
	client := &fakeProviderClient{
		versions: []modsource.Version{
			{
				ID:            "v1",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 1",
				VersionNumber: "1.0",
				Channel:       modsource.ChannelRelease,
				GameVersions:  []string{"1.20.1", "1.20.2", "1.20.3"},
				Loaders:       []string{"fabric", "quilt", "neoforge", "forge"},
				Files:         []modsource.File{{Name: "sodium-1.jar", URL: "https://cdn/sodium-1.jar", Primary: true}},
			},
			{
				ID:            "v2",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 2 beta",
				VersionNumber: "2.0-beta",
				Channel:       modsource.ChannelBeta,
				GameVersions:  []string{"1.21"},
				Loaders:       []string{"fabric"},
				Files:         []modsource.File{{Name: "sodium-2.jar", URL: "https://cdn/sodium-2.jar", Primary: true}},
			},
			{
				ID:            "v3",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 3",
				VersionNumber: "3.0",
				Channel:       modsource.ChannelRelease,
				GameVersions:  []string{"1.21"},
				Loaders:       []string{"fabric"},
				Files:         []modsource.File{{Name: "sodium-3.jar", URL: "https://cdn/sodium-3.jar", Primary: true}},
			},
		},
	}

	out, errOut, err := executeVersionsCommand(t, client,
		" sodium ",
		"--game-version", " 1.20.1 ",
		"--loader", " fabric ",
		"--channel", "release,beta",
		"--limit", "2",
		"--offset", "1",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
	if client.versionsCalls != 1 {
		t.Fatalf("versionsCalls = %d, want 1", client.versionsCalls)
	}
	if client.versionsID != "sodium" {
		t.Fatalf("versionsID = %q, want sodium", client.versionsID)
	}

	wantFilter := modsource.VersionFilter{
		GameVersion: "1.20.1",
		Loader:      "fabric",
		Channels:    []modsource.ReleaseChannel{modsource.ChannelRelease, modsource.ChannelBeta},
	}
	if !reflect.DeepEqual(client.versionFilter, wantFilter) {
		t.Fatalf("versionFilter = %#v, want %#v", client.versionFilter, wantFilter)
	}

	for _, fragment := range []string{
		`Modrinth: 3 versões para "sodium"`,
		"jogo=1.20.1",
		"loader=fabric",
		"canal=release,beta",
		"VERSÃO",
		"2.0-beta",
		"3.0",
		"beta",
		"release",
		"sodium-2.jar",
		"sodium-3.jar",
		"Exibindo 2-3 de 3.",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("output = %q, missing %q", out, fragment)
		}
	}
	if strings.Contains(out, "sodium-1.jar") {
		t.Errorf("output = %q, should not include the first page", out)
	}
}

func TestVersionsCommandSummarizesLists(t *testing.T) {
	client := &fakeProviderClient{
		versions: []modsource.Version{
			{
				ID:            "v1",
				VersionNumber: "1.0",
				Channel:       modsource.ChannelRelease,
				GameVersions:  []string{"1.20.1", "1.20.2", "1.20.3"},
				Loaders:       []string{"fabric", "quilt", "neoforge", "forge"},
			},
		},
	}

	out, _, err := executeVersionsCommand(t, client, "sodium")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "1.20.1, 1.20.2 (+1)") {
		t.Errorf("output = %q, missing summarized game versions", out)
	}
	if !strings.Contains(out, "fabric, quilt, neoforge (+1)") {
		t.Errorf("output = %q, missing summarized loaders", out)
	}
}

func TestVersionsCommandNoResults(t *testing.T) {
	client := &fakeProviderClient{}

	out, _, err := executeVersionsCommand(t, client, "sodium")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Nenhuma versão encontrada com esses filtros.") {
		t.Errorf("output = %q, missing empty result message", out)
	}
}

func TestVersionsCommandOffsetPastEnd(t *testing.T) {
	client := &fakeProviderClient{
		versions: []modsource.Version{
			{ID: "v1", VersionNumber: "1.0", Channel: modsource.ChannelRelease},
			{ID: "v2", VersionNumber: "2.0", Channel: modsource.ChannelRelease},
		},
	}

	out, _, err := executeVersionsCommand(t, client, "sodium", "--offset", "5")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Nenhuma versão nesta página. Use --offset entre 0 e 1.") {
		t.Errorf("output = %q, missing offset guidance", out)
	}
}

func TestVersionsCommandDefaults(t *testing.T) {
	client := &fakeProviderClient{
		versions: []modsource.Version{
			{ID: "v1", VersionNumber: "1.0", Channel: modsource.ChannelRelease},
		},
	}

	out, _, err := executeVersionsCommand(t, client, "sodium")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(client.versionFilter, modsource.VersionFilter{}) {
		t.Fatalf("versionFilter = %#v, want empty", client.versionFilter)
	}
	if !strings.Contains(out, `1 versão para "sodium"`) {
		t.Errorf("output = %q, missing singular header", out)
	}
	if !strings.Contains(out, "Exibindo 1-1 de 1.") {
		t.Errorf("output = %q, missing default page summary", out)
	}
}

func TestVersionsCommandRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name   string
		client modsource.Client
		args   []string
		want   string
	}{
		{
			name:   "ID vazio",
			client: &fakeProviderClient{},
			args:   []string{"  "},
			want:   "informe o slug ou ID",
		},
		{
			name:   "canal inválido",
			client: &fakeProviderClient{},
			args:   []string{"sodium", "--channel", "invalido"},
			want:   "canal inválido",
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
			name:   "provedor indisponível",
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

			_, _, err := executeVersionsCommand(t, test.client, test.args...)
			if err == nil {
				t.Fatal("Execute() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %q, want it to contain %q", err, test.want)
			}
			if fake != nil && fake.versionsCalls != 0 {
				t.Fatalf("versionsCalls = %d, want 0", fake.versionsCalls)
			}
		})
	}
}

func TestVersionsCommandReturnsAPIError(t *testing.T) {
	client := &fakeProviderClient{
		versionsErr: &modsource.APIError{
			Provider:   modsource.ProviderModrinth,
			Operation:  "listar versões",
			StatusCode: 404,
			Message:    "projeto não encontrado",
			Err:        modsource.ErrNotFound,
		},
	}

	out, _, err := executeVersionsCommand(t, client, "ausente")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}
	if !errors.Is(err, modsource.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "falha ao listar as versões") {
		t.Fatalf("Execute() error = %q, want context", err)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty", out)
	}
}

func TestParseReleaseChannels(t *testing.T) {
	channels, err := parseReleaseChannels([]string{"release,beta", "beta", "alpha"})
	if err != nil {
		t.Fatalf("parseReleaseChannels() error = %v", err)
	}

	want := []modsource.ReleaseChannel{
		modsource.ChannelRelease,
		modsource.ChannelBeta,
		modsource.ChannelAlpha,
	}
	if !reflect.DeepEqual(channels, want) {
		t.Fatalf("channels = %#v, want %#v", channels, want)
	}

	if _, err := parseReleaseChannels([]string{"stable"}); err == nil {
		t.Fatal("parseReleaseChannels() error = nil, want an invalid channel error")
	}
}

func TestSummarizeValues(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		max    int
		want   string
	}{
		{name: "vazio", values: nil, max: 3, want: "-"},
		{name: "limpa e deduplica", values: []string{" fabric ", "fabric", "", "quilt"}, max: 3, want: "fabric, quilt"},
		{name: "trunca", values: []string{"1.20.1", "1.20.2", "1.20.3"}, max: 2, want: "1.20.1, 1.20.2 (+1)"},
		{name: "sem limite", values: []string{"a", "b"}, max: 0, want: "a, b"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := summarizeValues(test.values, test.max); got != test.want {
				t.Errorf("summarizeValues(%#v, %d) = %q, want %q", test.values, test.max, got, test.want)
			}
		})
	}
}

func TestVersionFileLabel(t *testing.T) {
	for _, test := range []struct {
		name    string
		version modsource.Version
		want    string
	}{
		{name: "sem arquivos", version: modsource.Version{}, want: "-"},
		{
			name: "sem download disponível",
			version: modsource.Version{
				Files: []modsource.File{{Name: "indisponivel.jar"}},
			},
			want: "-",
		},
		{
			name: "arquivo primário",
			version: modsource.Version{
				Files: []modsource.File{
					{Name: "secundario.jar", URL: "https://cdn/secundario.jar"},
					{Name: "principal.jar", URL: "https://cdn/principal.jar", Primary: true},
				},
			},
			want: "principal.jar",
		},
		{
			name: "nome vazio",
			version: modsource.Version{
				Files: []modsource.File{{Name: "  ", URL: "https://cdn/sem-nome.jar"}},
			},
			want: "-",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := versionFileLabel(test.version)
			if err != nil {
				t.Fatalf("versionFileLabel() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("versionFileLabel() = %q, want %q", got, test.want)
			}
		})
	}
}
