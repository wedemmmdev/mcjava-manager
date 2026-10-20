package cli

import (
	"context"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

type fakeProviderClient struct {
	provider modsource.Provider

	result      modsource.SearchResult
	err         error
	query       modsource.SearchQuery
	searchCalls int

	project      modsource.Project
	projectErr   error
	projectID    string
	projectCalls int

	versions      []modsource.Version
	versionsErr   error
	versionFilter modsource.VersionFilter
	versionsID    string
	versionsCalls int

	version      modsource.Version
	versionErr   error
	versionID    string
	versionCalls int
}

func (f *fakeProviderClient) Provider() modsource.Provider {
	if f.provider == "" {
		return modsource.ProviderModrinth
	}

	return f.provider
}

func (f *fakeProviderClient) Capabilities() modsource.Capabilities {
	return modsource.Capabilities{MaxPageSize: 100}
}

func (f *fakeProviderClient) Search(_ context.Context, query modsource.SearchQuery) (modsource.SearchResult, error) {
	f.searchCalls++
	f.query = query

	return f.result, f.err
}

func (f *fakeProviderClient) GetProject(_ context.Context, id string) (modsource.Project, error) {
	f.projectCalls++
	f.projectID = id

	return f.project, f.projectErr
}

func (f *fakeProviderClient) ListVersions(_ context.Context, projectID string, filter modsource.VersionFilter) ([]modsource.Version, error) {
	f.versionsCalls++
	f.versionsID = projectID
	f.versionFilter = filter

	return f.versions, f.versionsErr
}

func (f *fakeProviderClient) GetVersion(_ context.Context, versionID string) (modsource.Version, error) {
	f.versionCalls++
	f.versionID = versionID

	return f.version, f.versionErr
}
