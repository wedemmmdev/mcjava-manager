// Package modrinth implements the modsource.Client contract for the Modrinth
// API. The adapter is read-only and uses only the Go standard library.
package modrinth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

const (
	// DefaultBaseURL is the public Modrinth v2 API endpoint.
	DefaultBaseURL = "https://api.modrinth.com/v2"

	// DefaultUserAgent identifies this application to Modrinth.
	DefaultUserAgent = "mcjava-manager/dev (https://github.com/wedemmmdev/mcjava-manager)"

	// DefaultTimeout is applied when the caller does not provide an HTTP
	// client with a timeout.
	DefaultTimeout = 20 * time.Second

	// DefaultPageSize is used when SearchQuery.Limit is zero or negative.
	DefaultPageSize = 20

	// MaxPageSize is the largest page accepted by Modrinth search.
	MaxPageSize = 100

	maxResponseBytes = 8 << 20 // 8 MiB
)

// Client implements modsource.Client for Modrinth.
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

var _ modsource.Client = (*Client)(nil)

// Option configures a Client.
type Option func(*Client)

// New creates a Modrinth client with safe defaults.
func New(opts ...Option) *Client {
	client := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		userAgent:  DefaultUserAgent,
	}

	for _, opt := range opts {
		opt(client)
	}

	client.baseURL = strings.TrimRight(client.baseURL, "/")

	return client
}

// WithBaseURL overrides the API base URL. It is primarily useful for tests.
func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		if trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/"); trimmed != "" {
			client.baseURL = trimmed
		}
	}
}

// WithHTTPClient provides a custom HTTP client. A zero timeout is replaced by
// DefaultTimeout so that requests can never hang indefinitely.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient == nil {
			return
		}

		copy := &http.Client{
			Transport:     httpClient.Transport,
			CheckRedirect: httpClient.CheckRedirect,
			Jar:           httpClient.Jar,
			Timeout:       httpClient.Timeout,
		}
		if copy.Timeout == 0 {
			copy.Timeout = DefaultTimeout
		}

		client.httpClient = copy
	}
}

// WithUserAgent overrides the User-Agent sent to Modrinth.
func WithUserAgent(userAgent string) Option {
	return func(client *Client) {
		if trimmed := strings.TrimSpace(userAgent); trimmed != "" {
			client.userAgent = trimmed
		}
	}
}

// Provider identifies the remote repository.
func (c *Client) Provider() modsource.Provider {
	return modsource.ProviderModrinth
}

// Capabilities describes the content kinds and page limits supported by
// Modrinth.
func (c *Client) Capabilities() modsource.Capabilities {
	return modsource.Capabilities{
		SupportedKinds: []modsource.ProjectKind{
			modsource.KindMod,
			modsource.KindModpack,
			modsource.KindResourcePack,
			modsource.KindShaderPack,
			modsource.KindDatapack,
			modsource.KindPlugin,
		},
		MaxPageSize: MaxPageSize,
	}
}

// Search queries Modrinth projects.
func (c *Client) Search(ctx context.Context, query modsource.SearchQuery) (modsource.SearchResult, error) {
	projectType, err := projectTypeForKind(query.Kind)
	if err != nil {
		return modsource.SearchResult{}, err
	}

	values := make(url.Values)
	if text := strings.TrimSpace(query.Text); text != "" {
		values.Set("query", text)
	}
	values.Set("limit", strconv.Itoa(normalizeLimit(query.Limit)))
	values.Set("offset", strconv.Itoa(normalizeOffset(query.Offset)))
	if query.Sort != "" {
		values.Set("index", sortIndex(query.Sort))
	}

	facets := buildFacets(query, projectType)
	if len(facets) > 0 {
		encoded, err := json.Marshal(facets)
		if err != nil {
			return modsource.SearchResult{}, fmt.Errorf("falha ao montar os filtros de busca: %w", err)
		}
		values.Set("facets", string(encoded))
	}

	endpoint := c.baseURL + "/search?" + values.Encode()

	var response searchResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, "buscar projetos", &response); err != nil {
		return modsource.SearchResult{}, err
	}

	result := modsource.SearchResult{
		Projects: make([]modsource.Project, 0, len(response.Hits)),
		Total:    response.TotalHits,
		Offset:   response.Offset,
		Limit:    response.Limit,
	}
	for _, hit := range response.Hits {
		result.Projects = append(result.Projects, hit.toProject())
	}

	return result, nil
}

// GetProject fetches a project by Modrinth ID or slug.
func (c *Client) GetProject(ctx context.Context, idOrSlug string) (modsource.Project, error) {
	idOrSlug = strings.TrimSpace(idOrSlug)
	if idOrSlug == "" {
		return modsource.Project{}, fmt.Errorf("ID ou slug do projeto não informado")
	}

	endpoint := c.baseURL + "/project/" + url.PathEscape(idOrSlug)

	var response projectResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, "obter projeto", &response); err != nil {
		return modsource.Project{}, err
	}

	return response.toProject(), nil
}

// ListVersions returns every project version matching filter. Modrinth returns
// the full list in one request; channel filtering is applied locally.
func (c *Client) ListVersions(ctx context.Context, projectID string, filter modsource.VersionFilter) ([]modsource.Version, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("ID do projeto não informado")
	}

	values := make(url.Values)
	if gameVersion := strings.TrimSpace(filter.GameVersion); gameVersion != "" {
		encoded, err := json.Marshal([]string{gameVersion})
		if err != nil {
			return nil, fmt.Errorf("falha ao montar o filtro de versão do jogo: %w", err)
		}
		values.Set("game_versions", string(encoded))
	}
	if loader := strings.TrimSpace(filter.Loader); loader != "" {
		encoded, err := json.Marshal([]string{loader})
		if err != nil {
			return nil, fmt.Errorf("falha ao montar o filtro de loader: %w", err)
		}
		values.Set("loaders", string(encoded))
	}

	endpoint := c.baseURL + "/project/" + url.PathEscape(projectID) + "/version"
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	var response []versionResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, "listar versões", &response); err != nil {
		return nil, err
	}

	versions := make([]modsource.Version, 0, len(response))
	for _, item := range response {
		version := item.toVersion()
		if matchesChannels(version.Channel, filter.Channels) {
			versions = append(versions, version)
		}
	}

	return versions, nil
}

// GetVersion fetches a single Modrinth version by its ID.
func (c *Client) GetVersion(ctx context.Context, versionID string) (modsource.Version, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return modsource.Version{}, fmt.Errorf("ID da versão não informado")
	}

	endpoint := c.baseURL + "/version/" + url.PathEscape(versionID)

	var response versionResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, "obter versão", &response); err != nil {
		return modsource.Version{}, err
	}

	return response.toVersion(), nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint, operation string, target any) error {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return fmt.Errorf("falha ao criar a requisição para %s no Modrinth: %w", operation, err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("falha ao %s no Modrinth: %w", operation, err)
	}

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		return fmt.Errorf("falha ao ler a resposta do Modrinth ao %s: %w", operation, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("falha ao fechar a resposta do Modrinth ao %s: %w", operation, closeErr)
	}
	if len(body) > maxResponseBytes {
		return &modsource.APIError{
			Provider:   modsource.ProviderModrinth,
			Operation:  operation,
			StatusCode: response.StatusCode,
			Message:    fmt.Sprintf("resposta excede o limite de %d bytes", maxResponseBytes),
			Err:        modsource.ErrInvalidResponse,
		}
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(response, operation, body)
	}

	if target == nil {
		return nil
	}

	if err := json.Unmarshal(body, target); err != nil {
		return &modsource.APIError{
			Provider:   modsource.ProviderModrinth,
			Operation:  operation,
			StatusCode: response.StatusCode,
			Message:    "resposta JSON inválida",
			Err:        fmt.Errorf("%w: %v", modsource.ErrInvalidResponse, err),
		}
	}

	return nil
}

func newAPIError(response *http.Response, operation string, body []byte) error {
	apiErr := &modsource.APIError{
		Provider:   modsource.ProviderModrinth,
		Operation:  operation,
		StatusCode: response.StatusCode,
		RetryAfter: retryAfter(response),
	}

	var payload errorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Message = firstNonEmpty(payload.Description, payload.Error)
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(response.StatusCode)
	}

	switch response.StatusCode {
	case http.StatusNotFound:
		apiErr.Err = modsource.ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		apiErr.Err = modsource.ErrUnauthorized
	case http.StatusTooManyRequests:
		apiErr.Err = modsource.ErrRateLimited
	}

	return apiErr
}

func retryAfter(response *http.Response) time.Duration {
	if value := strings.TrimSpace(response.Header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if when, err := http.ParseTime(value); err == nil {
			if duration := time.Until(when); duration > 0 {
				return duration
			}
		}
	}

	if value := strings.TrimSpace(response.Header.Get("X-Ratelimit-Reset")); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	return 0
}

func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultPageSize
	case limit > MaxPageSize:
		return MaxPageSize
	default:
		return limit
	}
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}

	return offset
}

func sortIndex(sort modsource.SortBy) string {
	switch sort {
	case modsource.SortDownloads, modsource.SortFollows, modsource.SortNewest, modsource.SortUpdated:
		return string(sort)
	default:
		return string(modsource.SortRelevance)
	}
}

func buildFacets(query modsource.SearchQuery, projectType string) [][]string {
	var facets [][]string
	if projectType != "" {
		facets = append(facets, []string{"project_type:" + projectType})
	}

	seenCategories := make(map[string]struct{}, len(query.Categories)+1)
	for _, category := range query.Categories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}
		if _, seen := seenCategories[category]; seen {
			continue
		}
		seenCategories[category] = struct{}{}
		facets = append(facets, []string{"categories:" + category})
	}

	if loader := strings.TrimSpace(query.Loader); loader != "" {
		if _, seen := seenCategories[loader]; !seen {
			facets = append(facets, []string{"categories:" + loader})
		}
	}

	if gameVersion := strings.TrimSpace(query.GameVersion); gameVersion != "" {
		facets = append(facets, []string{"versions:" + gameVersion})
	}

	return facets
}

func projectTypeForKind(kind modsource.ProjectKind) (string, error) {
	if kind == "" {
		return "", nil
	}
	if kind == modsource.KindWorld {
		return "", fmt.Errorf("%w: o Modrinth não hospeda mundos/mapas", modsource.ErrUnsupportedKind)
	}
	if !kind.Valid() {
		return "", fmt.Errorf("%w: %q", modsource.ErrUnsupportedKind, kind)
	}

	return string(kind), nil
}

func matchesChannels(channel modsource.ReleaseChannel, channels []modsource.ReleaseChannel) bool {
	if len(channels) == 0 {
		return true
	}

	for _, allowed := range channels {
		if channel == allowed {
			return true
		}
	}

	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}
