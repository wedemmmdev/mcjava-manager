package cli

import (
	"context"

	"github.com/wedemmmdev/modink/pkg/modsource"
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

	projects    map[string]modsource.Project
	projectErrs map[string]error

	versions      []modsource.Version
	versionsErr   error
	versionFilter modsource.VersionFilter
	versionsID    string
	versionsCalls int

	versionsByProject       map[string][]modsource.Version
	versionsByProjectErrs   map[string]error
	versionFiltersByProject map[string]modsource.VersionFilter

	version      modsource.Version
	versionErr   error
	versionID    string
	versionCalls int

	versionsByID map[string]modsource.Version
	versionErrs  map[string]error
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

	if f.projects != nil {
		if project, ok := f.projects[id]; ok {
			return project, nil
		}
		if err, ok := f.projectErrs[id]; ok {
			return modsource.Project{}, err
		}

		return modsource.Project{}, modsource.ErrNotFound
	}

	return f.project, f.projectErr
}

func (f *fakeProviderClient) ListVersions(_ context.Context, projectID string, filter modsource.VersionFilter) ([]modsource.Version, error) {
	f.versionsCalls++
	f.versionsID = projectID
	f.versionFilter = filter
	if f.versionFiltersByProject != nil {
		f.versionFiltersByProject[projectID] = filter
	}

	if f.versionsByProject != nil {
		if versions, ok := f.versionsByProject[projectID]; ok {
			return versions, nil
		}
		if err, ok := f.versionsByProjectErrs[projectID]; ok {
			return nil, err
		}

		return nil, nil
	}

	return f.versions, f.versionsErr
}

func (f *fakeProviderClient) GetVersion(_ context.Context, versionID string) (modsource.Version, error) {
	f.versionCalls++
	f.versionID = versionID

	if f.versionsByID != nil {
		if version, ok := f.versionsByID[versionID]; ok {
			return version, nil
		}
		if err, ok := f.versionErrs[versionID]; ok {
			return modsource.Version{}, err
		}

		return modsource.Version{}, modsource.ErrNotFound
	}

	return f.version, f.versionErr
}
