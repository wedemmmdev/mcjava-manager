package modrinth

import (
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

var loaderCategories = map[string]struct{}{
	"bukkit":     {},
	"bungeecord": {},
	"fabric":     {},
	"folia":      {},
	"forge":      {},
	"liteloader": {},
	"modloader":  {},
	"neoforge":   {},
	"paper":      {},
	"purpur":     {},
	"quilt":      {},
	"rift":       {},
	"spigot":     {},
	"sponge":     {},
	"velocity":   {},
	"waterfall":  {},
}

type errorResponse struct {
	Error       string `json:"error"`
	Description string `json:"description"`
}

type searchResponse struct {
	Hits      []searchHit `json:"hits"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
	TotalHits int         `json:"total_hits"`
}

type searchHit struct {
	ProjectID     string    `json:"project_id"`
	ProjectType   string    `json:"project_type"`
	Slug          string    `json:"slug"`
	Author        string    `json:"author"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Categories    []string  `json:"categories"`
	Versions      []string  `json:"versions"`
	Downloads     int64     `json:"downloads"`
	Follows       int64     `json:"follows"`
	IconURL       string    `json:"icon_url"`
	DateModified  time.Time `json:"date_modified"`
	LatestVersion string    `json:"latest_version"`
	License       string    `json:"license"`
}

type projectResponse struct {
	ID                   string          `json:"id"`
	Slug                 string          `json:"slug"`
	ProjectType          string          `json:"project_type"`
	Title                string          `json:"title"`
	Description          string          `json:"description"`
	Body                 string          `json:"body"`
	Published            time.Time       `json:"published"`
	Updated              time.Time       `json:"updated"`
	Downloads            int64           `json:"downloads"`
	Followers            int64           `json:"followers"`
	Categories           []string        `json:"categories"`
	AdditionalCategories []string        `json:"additional_categories"`
	Loaders              []string        `json:"loaders"`
	GameVersions         []string        `json:"game_versions"`
	IconURL              string          `json:"icon_url"`
	License              licenseResponse `json:"license"`
	SourceURL            string          `json:"source_url"`
	IssuesURL            string          `json:"issues_url"`
	WikiURL              string          `json:"wiki_url"`
	DiscordURL           string          `json:"discord_url"`
}

type licenseResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type versionResponse struct {
	ID            string               `json:"id"`
	ProjectID     string               `json:"project_id"`
	Name          string               `json:"name"`
	VersionNumber string               `json:"version_number"`
	Changelog     string               `json:"changelog"`
	VersionType   string               `json:"version_type"`
	GameVersions  []string             `json:"game_versions"`
	Loaders       []string             `json:"loaders"`
	DatePublished time.Time            `json:"date_published"`
	Downloads     int64                `json:"downloads"`
	Files         []fileResponse       `json:"files"`
	Dependencies  []dependencyResponse `json:"dependencies"`
}

type fileResponse struct {
	ID       string            `json:"id"`
	Hashes   map[string]string `json:"hashes"`
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
	Size     int64             `json:"size"`
}

type dependencyResponse struct {
	VersionID      *string `json:"version_id"`
	ProjectID      *string `json:"project_id"`
	FileName       *string `json:"file_name"`
	DependencyType string  `json:"dependency_type"`
}

func (hit searchHit) toProject() modsource.Project {
	loaders, categories := splitLoaderCategories(hit.Categories)

	return modsource.Project{
		Provider:        modsource.ProviderModrinth,
		ID:              hit.ProjectID,
		Slug:            hit.Slug,
		Name:            hit.Title,
		Summary:         hit.Description,
		Authors:         authorsFrom(hit.Author),
		IconURL:         hit.IconURL,
		License:         strings.TrimSpace(hit.License),
		Categories:      categories,
		GameVersions:    hit.Versions,
		Loaders:         loaders,
		Downloads:       hit.Downloads,
		Followers:       hit.Follows,
		Type:            modsource.ProjectKind(hit.ProjectType),
		LatestVersionID: hit.LatestVersion,
		UpdatedAt:       hit.DateModified,
	}
}

func (project projectResponse) toProject() modsource.Project {
	categories := append(append([]string{}, project.Categories...), project.AdditionalCategories...)
	detectedLoaders, categories := splitLoaderCategories(categories)
	loaders := uniqueStrings(append(append([]string{}, project.Loaders...), detectedLoaders...))

	// Modrinth's project endpoint exposes only the team ID, not member names.
	// Search results include the author, so Authors stays empty here.
	return modsource.Project{
		Provider:    modsource.ProviderModrinth,
		ID:          project.ID,
		Slug:        project.Slug,
		Name:        project.Title,
		Summary:     project.Description,
		Description: project.Body,
		IconURL:     project.IconURL,
		Links: modsource.Links{
			Website: projectWebsite(project.ProjectType, project.Slug),
			Source:  project.SourceURL,
			Issues:  project.IssuesURL,
			Wiki:    project.WikiURL,
			Discord: project.DiscordURL,
		},
		License:      firstNonEmpty(project.License.ID, project.License.Name),
		Categories:   categories,
		GameVersions: project.GameVersions,
		Loaders:      loaders,
		Downloads:    project.Downloads,
		Followers:    project.Followers,
		Type:         modsource.ProjectKind(project.ProjectType),
		CreatedAt:    project.Published,
		UpdatedAt:    project.Updated,
	}
}

func (version versionResponse) toVersion() modsource.Version {
	files := make([]modsource.File, 0, len(version.Files))
	for _, file := range version.Files {
		files = append(files, modsource.File{
			ID:      file.ID,
			Name:    file.Filename,
			URL:     file.URL,
			Size:    file.Size,
			Primary: file.Primary,
			Hashes:  hashesFromAPI(file.Hashes),
		})
	}

	dependencies := make([]modsource.Dependency, 0, len(version.Dependencies))
	for _, dependency := range version.Dependencies {
		dependencies = append(dependencies, modsource.Dependency{
			ProjectID: stringValue(dependency.ProjectID),
			VersionID: stringValue(dependency.VersionID),
			FileName:  stringValue(dependency.FileName),
			Kind:      modsource.DependencyKind(dependency.DependencyType),
		})
	}

	return modsource.Version{
		Provider:      modsource.ProviderModrinth,
		ID:            version.ID,
		ProjectID:     version.ProjectID,
		Name:          version.Name,
		VersionNumber: version.VersionNumber,
		Changelog:     version.Changelog,
		Channel:       modsource.ReleaseChannel(version.VersionType),
		GameVersions:  version.GameVersions,
		Loaders:       version.Loaders,
		PublishedAt:   version.DatePublished,
		Downloads:     version.Downloads,
		Files:         files,
		Dependencies:  dependencies,
	}
}

func hashesFromAPI(hashes map[string]string) []modsource.Hash {
	if len(hashes) == 0 {
		return nil
	}

	algorithms := make([]string, 0, len(hashes))
	for algorithm := range hashes {
		algorithms = append(algorithms, algorithm)
	}
	sort.Strings(algorithms)

	result := make([]modsource.Hash, 0, len(algorithms))
	for _, algorithm := range algorithms {
		value := strings.TrimSpace(hashes[algorithm])
		if value == "" {
			continue
		}
		result = append(result, modsource.Hash{
			Algorithm: modsource.HashAlgorithm(algorithm),
			Value:     value,
		})
	}

	return result
}

func splitLoaderCategories(categories []string) (loaders, rest []string) {
	seenLoaders := make(map[string]struct{})
	seenRest := make(map[string]struct{})

	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}

		if _, ok := loaderCategories[category]; ok {
			if _, seen := seenLoaders[category]; seen {
				continue
			}
			seenLoaders[category] = struct{}{}
			loaders = append(loaders, category)
			continue
		}

		if _, seen := seenRest[category]; seen {
			continue
		}
		seenRest[category] = struct{}{}
		rest = append(rest, category)
	}

	return loaders, rest
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func authorsFrom(author string) []string {
	if trimmed := strings.TrimSpace(author); trimmed != "" {
		return []string{trimmed}
	}

	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func projectWebsite(projectType, slug string) string {
	projectType = strings.TrimSpace(projectType)
	slug = strings.TrimSpace(slug)
	if projectType == "" || slug == "" {
		return ""
	}

	return "https://modrinth.com/" + url.PathEscape(projectType) + "/" + url.PathEscape(slug)
}
