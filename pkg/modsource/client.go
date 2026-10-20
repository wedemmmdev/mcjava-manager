package modsource

import "context"

// Capabilities describes provider limits and supported content kinds.
type Capabilities struct {
	SupportedKinds []ProjectKind
	MaxPageSize    int
}

// Client is the provider-agnostic contract implemented by the Modrinth and
// CurseForge adapters. Implementations are responsible for translating
// provider-specific pagination, filters, and error responses.
type Client interface {
	// Provider identifies the remote repository backing this client.
	Provider() Provider

	// Capabilities describes limits and content kinds supported by the client.
	Capabilities() Capabilities

	// Search returns projects matching the query.
	Search(ctx context.Context, query SearchQuery) (SearchResult, error)

	// GetProject fetches a project by provider-specific ID or slug.
	GetProject(ctx context.Context, idOrSlug string) (Project, error)

	// ListVersions returns every version of a project that matches filter.
	// Implementations handle provider pagination internally.
	ListVersions(ctx context.Context, projectID string, filter VersionFilter) ([]Version, error)

	// GetVersion fetches one version by its provider-specific ID.
	GetVersion(ctx context.Context, versionID string) (Version, error)
}
