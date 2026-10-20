package modrinth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

const searchResponseJSON = `{
  "hits": [
    {
      "project_id": "AANobbMI",
      "project_type": "mod",
      "slug": "sodium",
      "author": "jellysquid3",
      "title": "Sodium",
      "description": "Um motor de renderização",
      "categories": ["fabric", "optimization"],
      "versions": ["1.20.1", "1.21"],
      "downloads": 1000,
      "follows": 50,
      "icon_url": "https://cdn.example/icon.png",
      "date_modified": "2025-01-02T03:04:05Z",
      "latest_version": "ver-release",
      "license": "lgpl-3"
    }
  ],
  "offset": 10,
  "limit": 25,
  "total_hits": 1
}`

const projectResponseJSON = `{
  "id": "AANobbMI",
  "slug": "sodium",
  "project_type": "mod",
  "title": "Sodium",
  "description": "Um motor de renderização",
  "body": "Descrição completa",
  "published": "2020-01-02T03:04:05Z",
  "updated": "2025-02-03T04:05:06Z",
  "downloads": 123456,
  "followers": 789,
  "categories": ["optimization"],
  "additional_categories": ["fabric"],
  "loaders": ["fabric", "quilt"],
  "game_versions": ["1.20.1", "1.21"],
  "icon_url": "https://cdn.example/icon.png",
  "license": {
    "id": "lgpl-3",
    "name": "GNU LGPLv3",
    "url": "https://example.com/license"
  },
  "source_url": "https://github.com/example/sodium",
  "issues_url": "https://github.com/example/sodium/issues",
  "wiki_url": "https://example.com/wiki",
  "discord_url": "https://example.com/discord"
}`

const releaseVersionJSON = `{
  "id": "ver-release",
  "project_id": "AANobbMI",
  "name": "Sodium 1.0",
  "version_number": "1.0",
  "changelog": "Correções",
  "version_type": "release",
  "game_versions": ["1.20.1"],
  "loaders": ["fabric"],
  "date_published": "2025-01-02T03:04:05Z",
  "downloads": 42,
  "files": [
    {
      "id": "file-1",
      "hashes": {
        "sha512": "sha512-value",
        "sha1": "sha1-value"
      },
      "url": "https://cdn.example/sodium.jar",
      "filename": "sodium.jar",
      "primary": true,
      "size": 1234
    }
  ],
  "dependencies": [
    {
      "version_id": "dep-version",
      "project_id": "dep-project",
      "file_name": null,
      "dependency_type": "required"
    }
  ]
}`

const betaVersionJSON = `{
  "id": "ver-beta",
  "project_id": "AANobbMI",
  "name": "Sodium Beta",
  "version_number": "1.1-beta",
  "version_type": "beta",
  "game_versions": ["1.21"],
  "loaders": ["fabric"],
  "date_published": "2025-03-04T05:06:07Z",
  "files": []
}`

const versionsResponseJSON = `[` + releaseVersionJSON + `,` + betaVersionJSON + `]`

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return New(WithBaseURL(server.URL + "/v2"))
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("falha ao escrever a resposta de teste: %v", err)
	}
}

func assertJSONRequest(t *testing.T, r *http.Request, method, path string) {
	t.Helper()

	if r.Method != method {
		t.Errorf("método = %q, esperado %q", r.Method, method)
	}
	if r.URL.Path != path {
		t.Errorf("caminho = %q, esperado %q", r.URL.Path, path)
	}
	if got := r.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, esperado application/json", got)
	}
	if got := r.Header.Get("User-Agent"); !strings.Contains(got, "mcjava-manager") {
		t.Errorf("User-Agent = %q, esperado identificação do mcjava-manager", got)
	}
}

func requireAPIError(t *testing.T, err error, status int, operation string, sentinel error) *modsource.APIError {
	t.Helper()

	if err == nil {
		t.Fatal("erro esperado, recebido nil")
	}

	var apiErr *modsource.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("erro %T não é *modsource.APIError: %v", err, err)
	}
	if apiErr.Provider != modsource.ProviderModrinth {
		t.Errorf("Provider = %q, esperado %q", apiErr.Provider, modsource.ProviderModrinth)
	}
	if apiErr.StatusCode != status {
		t.Errorf("StatusCode = %d, esperado %d", apiErr.StatusCode, status)
	}
	if apiErr.Operation != operation {
		t.Errorf("Operation = %q, esperado %q", apiErr.Operation, operation)
	}
	if sentinel != nil && !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(%v) = false, esperado true", sentinel)
	}

	return apiErr
}

func TestNewDefaults(t *testing.T) {
	client := New()

	if client.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, esperado %q", client.baseURL, DefaultBaseURL)
	}
	if client.userAgent != DefaultUserAgent {
		t.Errorf("userAgent = %q, esperado %q", client.userAgent, DefaultUserAgent)
	}
	if client.httpClient.Timeout != DefaultTimeout {
		t.Errorf("timeout = %s, esperado %s", client.httpClient.Timeout, DefaultTimeout)
	}
}

func TestOptions(t *testing.T) {
	customHTTPClient := &http.Client{Timeout: 3 * time.Second}
	client := New(
		WithBaseURL("  https://example.com/v2///  "),
		WithHTTPClient(customHTTPClient),
		WithUserAgent("  teste/1.0  "),
	)

	if client.baseURL != "https://example.com/v2" {
		t.Errorf("baseURL = %q, esperado https://example.com/v2", client.baseURL)
	}
	if client.userAgent != "teste/1.0" {
		t.Errorf("userAgent = %q, esperado teste/1.0", client.userAgent)
	}
	if client.httpClient.Timeout != 3*time.Second {
		t.Errorf("timeout = %s, esperado 3s", client.httpClient.Timeout)
	}

	zeroTimeout := New(WithHTTPClient(&http.Client{}))
	if zeroTimeout.httpClient.Timeout != DefaultTimeout {
		t.Errorf("timeout padrão = %s, esperado %s", zeroTimeout.httpClient.Timeout, DefaultTimeout)
	}

	nilClient := New(WithHTTPClient(nil))
	if nilClient.httpClient.Timeout != DefaultTimeout {
		t.Errorf("timeout com cliente nil = %s, esperado %s", nilClient.httpClient.Timeout, DefaultTimeout)
	}

	emptyOptions := New(WithBaseURL(""), WithUserAgent(""))
	if emptyOptions.baseURL != DefaultBaseURL {
		t.Errorf("baseURL vazia = %q, esperado %q", emptyOptions.baseURL, DefaultBaseURL)
	}
	if emptyOptions.userAgent != DefaultUserAgent {
		t.Errorf("userAgent vazio = %q, esperado %q", emptyOptions.userAgent, DefaultUserAgent)
	}
}

func TestProviderAndCapabilities(t *testing.T) {
	client := New()

	if got := client.Provider(); got != modsource.ProviderModrinth {
		t.Errorf("Provider() = %q, esperado %q", got, modsource.ProviderModrinth)
	}

	capabilities := client.Capabilities()
	if capabilities.MaxPageSize != MaxPageSize {
		t.Errorf("MaxPageSize = %d, esperado %d", capabilities.MaxPageSize, MaxPageSize)
	}

	wantedKinds := []modsource.ProjectKind{
		modsource.KindMod,
		modsource.KindModpack,
		modsource.KindResourcePack,
		modsource.KindShaderPack,
		modsource.KindDatapack,
		modsource.KindPlugin,
	}
	for _, kind := range wantedKinds {
		if !containsKind(capabilities.SupportedKinds, kind) {
			t.Errorf("tipo %q ausente nas capacidades", kind)
		}
	}
	if containsKind(capabilities.SupportedKinds, modsource.KindWorld) {
		t.Error("Modrinth não deve anunciar suporte a mundos")
	}
}

func containsKind(kinds []modsource.ProjectKind, wanted modsource.ProjectKind) bool {
	for _, kind := range kinds {
		if kind == wanted {
			return true
		}
	}

	return false
}

func TestSearch(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, http.MethodGet, "/v2/search")

		query := r.URL.Query()
		if got := query.Get("query"); got != "sodium" {
			t.Errorf("query = %q, esperado sodium", got)
		}
		if got := query.Get("limit"); got != "25" {
			t.Errorf("limit = %q, esperado 25", got)
		}
		if got := query.Get("offset"); got != "10" {
			t.Errorf("offset = %q, esperado 10", got)
		}
		if got := query.Get("index"); got != "downloads" {
			t.Errorf("index = %q, esperado downloads", got)
		}

		var facets [][]string
		if err := json.Unmarshal([]byte(query.Get("facets")), &facets); err != nil {
			t.Errorf("facets inválidos: %v", err)
		}
		wantedFacets := [][]string{
			{"project_type:mod"},
			{"categories:optimization"},
			{"categories:fabric"},
			{"versions:1.20.1"},
		}
		if !reflect.DeepEqual(facets, wantedFacets) {
			t.Errorf("facets = %#v, esperado %#v", facets, wantedFacets)
		}

		writeJSON(t, w, searchResponseJSON)
	})

	result, err := client.Search(context.Background(), modsource.SearchQuery{
		Text:        " sodium ",
		Kind:        modsource.KindMod,
		GameVersion: " 1.20.1 ",
		Loader:      " fabric ",
		Categories:  []string{"optimization", "optimization", "  "},
		Sort:        modsource.SortDownloads,
		Limit:       25,
		Offset:      10,
	})
	if err != nil {
		t.Fatalf("Search() erro = %v", err)
	}

	if result.Total != 1 || result.Offset != 10 || result.Limit != 25 {
		t.Errorf("paginação = total %d, offset %d, limit %d", result.Total, result.Offset, result.Limit)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("quantidade de projetos = %d, esperado 1", len(result.Projects))
	}

	project := result.Projects[0]
	if project.Provider != modsource.ProviderModrinth {
		t.Errorf("Provider = %q, esperado %q", project.Provider, modsource.ProviderModrinth)
	}
	if project.ID != "AANobbMI" || project.Slug != "sodium" || project.Name != "Sodium" {
		t.Errorf("identificação do projeto = %#v", project)
	}
	if project.Summary != "Um motor de renderização" {
		t.Errorf("Summary = %q", project.Summary)
	}
	if project.Description != "" {
		t.Errorf("Description de busca = %q, esperado vazio", project.Description)
	}
	if !reflect.DeepEqual(project.Authors, []string{"jellysquid3"}) {
		t.Errorf("Authors = %#v", project.Authors)
	}
	if !reflect.DeepEqual(project.Categories, []string{"optimization"}) {
		t.Errorf("Categories = %#v", project.Categories)
	}
	if !reflect.DeepEqual(project.Loaders, []string{"fabric"}) {
		t.Errorf("Loaders = %#v", project.Loaders)
	}
	if !reflect.DeepEqual(project.GameVersions, []string{"1.20.1", "1.21"}) {
		t.Errorf("GameVersions = %#v", project.GameVersions)
	}
	if project.Downloads != 1000 || project.Followers != 50 {
		t.Errorf("métricas = downloads %d, followers %d", project.Downloads, project.Followers)
	}
	if project.Type != modsource.KindMod {
		t.Errorf("Type = %q, esperado %q", project.Type, modsource.KindMod)
	}
	if project.License != "lgpl-3" {
		t.Errorf("License = %q, esperado lgpl-3", project.License)
	}
	if project.LatestVersionID != "ver-release" {
		t.Errorf("LatestVersionID = %q, esperado ver-release", project.LatestVersionID)
	}
	if project.IconURL != "https://cdn.example/icon.png" {
		t.Errorf("IconURL = %q", project.IconURL)
	}
	wantedTime := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	if !project.UpdatedAt.Equal(wantedTime) {
		t.Errorf("UpdatedAt = %s, esperado %s", project.UpdatedAt, wantedTime)
	}
}

func TestSearchDefaultsAndEmptyFilters(t *testing.T) {
	calls := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, http.MethodGet, "/v2/search")

		query := r.URL.Query()
		switch calls {
		case 0:
			if got := query.Get("limit"); got != "20" {
				t.Errorf("limit = %q, esperado 20", got)
			}
			if got := query.Get("offset"); got != "0" {
				t.Errorf("offset = %q, esperado 0", got)
			}
			if got := query.Get("index"); got != "" {
				t.Errorf("index = %q, esperado vazio", got)
			}
			if got := query.Get("facets"); got != "" {
				t.Errorf("facets = %q, esperado vazio", got)
			}
		case 1:
			if got := query.Get("limit"); got != "100" {
				t.Errorf("limit = %q, esperado 100", got)
			}
			if got := query.Get("offset"); got != "0" {
				t.Errorf("offset = %q, esperado 0", got)
			}
			if got := query.Get("index"); got != "relevance" {
				t.Errorf("index = %q, esperado relevance", got)
			}
		default:
			t.Errorf("requisição inesperada: %d", calls)
		}
		calls++

		writeJSON(t, w, `{"hits":[],"offset":0,"limit":20,"total_hits":0}`)
	})

	if _, err := client.Search(context.Background(), modsource.SearchQuery{Limit: 0, Offset: -10}); err != nil {
		t.Fatalf("primeira Search() erro = %v", err)
	}
	if _, err := client.Search(context.Background(), modsource.SearchQuery{
		Limit:  1000,
		Offset: -1,
		Sort:   modsource.SortRelevance,
	}); err != nil {
		t.Fatalf("segunda Search() erro = %v", err)
	}
}

func TestSearchRejectsUnsupportedKinds(t *testing.T) {
	client := New(WithBaseURL("http://127.0.0.1:1"))

	for _, kind := range []modsource.ProjectKind{modsource.KindWorld, modsource.ProjectKind("desconhecido")} {
		_, err := client.Search(context.Background(), modsource.SearchQuery{Kind: kind})
		if !errors.Is(err, modsource.ErrUnsupportedKind) {
			t.Errorf("tipo %q: erro = %v, esperado ErrUnsupportedKind", kind, err)
		}
	}
}

func TestGetProject(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, http.MethodGet, "/v2/project/sodium")
		writeJSON(t, w, projectResponseJSON)
	})

	project, err := client.GetProject(context.Background(), " sodium ")
	if err != nil {
		t.Fatalf("GetProject() erro = %v", err)
	}

	if project.Provider != modsource.ProviderModrinth || project.ID != "AANobbMI" || project.Slug != "sodium" {
		t.Errorf("identificação do projeto = %#v", project)
	}
	if project.Name != "Sodium" || project.Summary != "Um motor de renderização" {
		t.Errorf("nome/resumo = %q / %q", project.Name, project.Summary)
	}
	if project.Description != "Descrição completa" {
		t.Errorf("Description = %q", project.Description)
	}
	if !reflect.DeepEqual(project.Categories, []string{"optimization"}) {
		t.Errorf("Categories = %#v", project.Categories)
	}
	if !reflect.DeepEqual(project.Loaders, []string{"fabric", "quilt"}) {
		t.Errorf("Loaders = %#v", project.Loaders)
	}
	if !reflect.DeepEqual(project.GameVersions, []string{"1.20.1", "1.21"}) {
		t.Errorf("GameVersions = %#v", project.GameVersions)
	}
	if project.Downloads != 123456 || project.Followers != 789 {
		t.Errorf("métricas = downloads %d, followers %d", project.Downloads, project.Followers)
	}
	if project.Type != modsource.KindMod {
		t.Errorf("Type = %q, esperado %q", project.Type, modsource.KindMod)
	}
	if project.License != "lgpl-3" {
		t.Errorf("License = %q, esperado lgpl-3", project.License)
	}
	if project.Links.Website != "https://modrinth.com/mod/sodium" {
		t.Errorf("Website = %q", project.Links.Website)
	}
	if project.Links.Source != "https://github.com/example/sodium" {
		t.Errorf("Source = %q", project.Links.Source)
	}
	if project.Links.Issues != "https://github.com/example/sodium/issues" {
		t.Errorf("Issues = %q", project.Links.Issues)
	}
	if project.Links.Wiki != "https://example.com/wiki" {
		t.Errorf("Wiki = %q", project.Links.Wiki)
	}
	if project.Links.Discord != "https://example.com/discord" {
		t.Errorf("Discord = %q", project.Links.Discord)
	}
	if len(project.Authors) != 0 {
		t.Errorf("Authors = %#v, esperado vazio no endpoint de projeto", project.Authors)
	}

	created := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2025, time.February, 3, 4, 5, 6, 0, time.UTC)
	if !project.CreatedAt.Equal(created) || !project.UpdatedAt.Equal(updated) {
		t.Errorf("datas = %s / %s, esperado %s / %s", project.CreatedAt, project.UpdatedAt, created, updated)
	}
}

func TestListVersions(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, http.MethodGet, "/v2/project/AANobbMI/version")

		query := r.URL.Query()
		var gameVersions []string
		if err := json.Unmarshal([]byte(query.Get("game_versions")), &gameVersions); err != nil {
			t.Errorf("game_versions inválido: %v", err)
		}
		if !reflect.DeepEqual(gameVersions, []string{"1.20.1"}) {
			t.Errorf("game_versions = %#v", gameVersions)
		}

		var loaders []string
		if err := json.Unmarshal([]byte(query.Get("loaders")), &loaders); err != nil {
			t.Errorf("loaders inválido: %v", err)
		}
		if !reflect.DeepEqual(loaders, []string{"fabric"}) {
			t.Errorf("loaders = %#v", loaders)
		}

		writeJSON(t, w, versionsResponseJSON)
	})

	versions, err := client.ListVersions(context.Background(), " AANobbMI ", modsource.VersionFilter{
		GameVersion: "1.20.1",
		Loader:      "fabric",
		Channels:    []modsource.ReleaseChannel{modsource.ChannelRelease},
	})
	if err != nil {
		t.Fatalf("ListVersions() erro = %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("quantidade de versões = %d, esperado 1", len(versions))
	}

	version := versions[0]
	if version.Provider != modsource.ProviderModrinth || version.ID != "ver-release" || version.ProjectID != "AANobbMI" {
		t.Errorf("identificação da versão = %#v", version)
	}
	if version.Name != "Sodium 1.0" || version.VersionNumber != "1.0" {
		t.Errorf("nome/número = %q / %q", version.Name, version.VersionNumber)
	}
	if version.Changelog != "Correções" {
		t.Errorf("Changelog = %q", version.Changelog)
	}
	if version.Channel != modsource.ChannelRelease {
		t.Errorf("Channel = %q", version.Channel)
	}
	if !reflect.DeepEqual(version.GameVersions, []string{"1.20.1"}) {
		t.Errorf("GameVersions = %#v", version.GameVersions)
	}
	if !reflect.DeepEqual(version.Loaders, []string{"fabric"}) {
		t.Errorf("Loaders = %#v", version.Loaders)
	}
	if version.Downloads != 42 {
		t.Errorf("Downloads = %d, esperado 42", version.Downloads)
	}
	published := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	if !version.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %s, esperado %s", version.PublishedAt, published)
	}

	if len(version.Files) != 1 {
		t.Fatalf("quantidade de arquivos = %d, esperado 1", len(version.Files))
	}
	file := version.Files[0]
	if file.ID != "file-1" || file.Name != "sodium.jar" || file.URL != "https://cdn.example/sodium.jar" {
		t.Errorf("arquivo = %#v", file)
	}
	if file.Size != 1234 || !file.Primary {
		t.Errorf("tamanho/primário = %d / %t", file.Size, file.Primary)
	}
	wantedHashes := []modsource.Hash{
		{Algorithm: modsource.HashSHA1, Value: "sha1-value"},
		{Algorithm: modsource.HashSHA512, Value: "sha512-value"},
	}
	if !reflect.DeepEqual(file.Hashes, wantedHashes) {
		t.Errorf("hashes = %#v, esperado %#v", file.Hashes, wantedHashes)
	}
	if _, err := version.PrimaryFile(); err != nil {
		t.Errorf("PrimaryFile() erro = %v", err)
	}

	if len(version.Dependencies) != 1 {
		t.Fatalf("quantidade de dependências = %d, esperado 1", len(version.Dependencies))
	}
	dependency := version.Dependencies[0]
	if dependency.ProjectID != "dep-project" || dependency.VersionID != "dep-version" {
		t.Errorf("dependência = %#v", dependency)
	}
	if dependency.FileName != "" {
		t.Errorf("FileName = %q, esperado vazio", dependency.FileName)
	}
	if dependency.Kind != modsource.DependencyRequired {
		t.Errorf("Kind = %q, esperado %q", dependency.Kind, modsource.DependencyRequired)
	}
}

func TestListVersionsAllChannelsAndNoFilters(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, http.MethodGet, "/v2/project/AANobbMI/version")
		query := r.URL.Query()
		if got := query.Get("game_versions"); got != "" {
			t.Errorf("game_versions = %q, esperado vazio", got)
		}
		if got := query.Get("loaders"); got != "" {
			t.Errorf("loaders = %q, esperado vazio", got)
		}

		writeJSON(t, w, versionsResponseJSON)
	})

	versions, err := client.ListVersions(context.Background(), "AANobbMI", modsource.VersionFilter{})
	if err != nil {
		t.Fatalf("ListVersions() erro = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("quantidade de versões = %d, esperado 2", len(versions))
	}
	if versions[0].Channel != modsource.ChannelRelease || versions[1].Channel != modsource.ChannelBeta {
		t.Errorf("canais = %q / %q", versions[0].Channel, versions[1].Channel)
	}
}

func TestGetVersion(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, http.MethodGet, "/v2/version/ver-release")
		writeJSON(t, w, releaseVersionJSON)
	})

	version, err := client.GetVersion(context.Background(), " ver-release ")
	if err != nil {
		t.Fatalf("GetVersion() erro = %v", err)
	}
	if version.ID != "ver-release" || version.ProjectID != "AANobbMI" {
		t.Errorf("identificação da versão = %#v", version)
	}
	if version.Provider != modsource.ProviderModrinth {
		t.Errorf("Provider = %q", version.Provider)
	}
	if version.Channel != modsource.ChannelRelease {
		t.Errorf("Channel = %q", version.Channel)
	}
}

func TestEmptyIdentifiers(t *testing.T) {
	client := New(WithBaseURL("http://127.0.0.1:1"))

	if _, err := client.GetProject(context.Background(), "  "); err == nil {
		t.Error("GetProject() com ID vazio deveria falhar")
	}
	if _, err := client.ListVersions(context.Background(), "  ", modsource.VersionFilter{}); err == nil {
		t.Error("ListVersions() com ID vazio deveria falhar")
	}
	if _, err := client.GetVersion(context.Background(), "  "); err == nil {
		t.Error("GetVersion() com ID vazio deveria falhar")
	}
}

func TestHTTPErrors(t *testing.T) {
	t.Run("não encontrado", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assertJSONRequest(t, r, http.MethodGet, "/v2/project/ausente")
			w.WriteHeader(http.StatusNotFound)
			writeJSON(t, w, `{"error":"not_found","description":"projeto não encontrado"}`)
		})

		_, err := client.GetProject(context.Background(), "ausente")
		apiErr := requireAPIError(t, err, http.StatusNotFound, "obter projeto", modsource.ErrNotFound)
		if apiErr.Message != "projeto não encontrado" {
			t.Errorf("Message = %q", apiErr.Message)
		}
	})

	t.Run("não autorizado", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assertJSONRequest(t, r, http.MethodGet, "/v2/search")
			w.WriteHeader(http.StatusForbidden)
		})

		_, err := client.Search(context.Background(), modsource.SearchQuery{})
		requireAPIError(t, err, http.StatusForbidden, "buscar projetos", modsource.ErrUnauthorized)
	})

	t.Run("limite de requisições", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assertJSONRequest(t, r, http.MethodGet, "/v2/project/AANobbMI/version")
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			writeJSON(t, w, `{"error":"rate_limited","description":"aguarde"}`)
		})

		_, err := client.ListVersions(context.Background(), "AANobbMI", modsource.VersionFilter{})
		apiErr := requireAPIError(t, err, http.StatusTooManyRequests, "listar versões", modsource.ErrRateLimited)
		if apiErr.RetryAfter != 3*time.Second {
			t.Errorf("RetryAfter = %s, esperado 3s", apiErr.RetryAfter)
		}
	})

	t.Run("erro do servidor", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assertJSONRequest(t, r, http.MethodGet, "/v2/version/ver-release")
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(t, w, `{"error":"internal_error","description":"falha interna"}`)
		})

		_, err := client.GetVersion(context.Background(), "ver-release")
		apiErr := requireAPIError(t, err, http.StatusInternalServerError, "obter versão", nil)
		if apiErr.Message != "falha interna" {
			t.Errorf("Message = %q", apiErr.Message)
		}
		if apiErr.Err != nil {
			t.Errorf("Err = %v, esperado nil", apiErr.Err)
		}
	})

	t.Run("JSON inválido", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assertJSONRequest(t, r, http.MethodGet, "/v2/search")
			writeJSON(t, w, `não é JSON`)
		})

		_, err := client.Search(context.Background(), modsource.SearchQuery{})
		apiErr := requireAPIError(t, err, http.StatusOK, "buscar projetos", modsource.ErrInvalidResponse)
		if apiErr.Message != "resposta JSON inválida" {
			t.Errorf("Message = %q", apiErr.Message)
		}
	})
}

func TestContextCancellation(t *testing.T) {
	started := make(chan struct{})
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-started
		cancel()
	}()

	_, err := client.GetProject(ctx, "sodium")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("erro = %v, esperado context.Canceled", err)
	}
}

func TestProjectWebsite(t *testing.T) {
	tests := []struct {
		name        string
		projectType string
		slug        string
		want        string
	}{
		{name: "mod", projectType: "mod", slug: "sodium", want: "https://modrinth.com/mod/sodium"},
		{name: "shader", projectType: "shader", slug: "iris", want: "https://modrinth.com/shader/iris"},
		{name: "slug vazio", projectType: "mod", slug: "", want: ""},
		{name: "tipo vazio", projectType: "", slug: "sodium", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := projectWebsite(test.projectType, test.slug); got != test.want {
				t.Errorf("projectWebsite(%q, %q) = %q, esperado %q", test.projectType, test.slug, got, test.want)
			}
		})
	}
}
