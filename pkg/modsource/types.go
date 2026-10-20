package modsource

import (
	"fmt"
	"strings"
	"time"
)

// ProjectKind classifies the type of content hosted by a provider.
type ProjectKind string

const (
	KindMod          ProjectKind = "mod"
	KindModpack      ProjectKind = "modpack"
	KindResourcePack ProjectKind = "resourcepack"
	KindShaderPack   ProjectKind = "shader"
	KindDatapack     ProjectKind = "datapack"
	KindWorld        ProjectKind = "world"
	KindPlugin       ProjectKind = "plugin"
)

// DisplayName returns the content kind suitable for user-facing output.
func (k ProjectKind) DisplayName() string {
	switch k {
	case KindMod:
		return "Mod"
	case KindModpack:
		return "Modpack"
	case KindResourcePack:
		return "Pacote de recursos"
	case KindShaderPack:
		return "Shader"
	case KindDatapack:
		return "Datapack"
	case KindWorld:
		return "Mapa/Mundo"
	case KindPlugin:
		return "Plugin"
	default:
		return string(k)
	}
}

// Valid reports whether the content kind is known.
func (k ProjectKind) Valid() bool {
	switch k {
	case KindMod, KindModpack, KindResourcePack, KindShaderPack, KindDatapack, KindWorld, KindPlugin:
		return true
	default:
		return false
	}
}

// ReleaseChannel is the stability channel of a version.
type ReleaseChannel string

const (
	ChannelRelease ReleaseChannel = "release"
	ChannelBeta    ReleaseChannel = "beta"
	ChannelAlpha   ReleaseChannel = "alpha"
)

// SortBy controls how search results are ordered.
type SortBy string

const (
	SortRelevance SortBy = "relevance"
	SortDownloads SortBy = "downloads"
	SortFollows   SortBy = "follows"
	SortNewest    SortBy = "newest"
	SortUpdated   SortBy = "updated"
)

// HashAlgorithm identifies a file checksum algorithm.
type HashAlgorithm string

const (
	HashSHA1   HashAlgorithm = "sha1"
	HashSHA512 HashAlgorithm = "sha512"
	HashMD5    HashAlgorithm = "md5"
)

// Hash is a checksum of a downloadable file.
type Hash struct {
	Algorithm HashAlgorithm
	Value     string
}

// File is a downloadable artifact attached to a version.
type File struct {
	ID      string
	Name    string
	URL     string
	Size    int64
	Primary bool
	Hashes  []Hash
}

// Downloadable reports whether the provider exposes a direct download URL.
// Some authors disable third-party distribution, leaving URL empty.
func (f File) Downloadable() bool {
	return strings.TrimSpace(f.URL) != ""
}

// DependencyKind describes how a dependency relates to a version.
type DependencyKind string

const (
	DependencyRequired     DependencyKind = "required"
	DependencyOptional     DependencyKind = "optional"
	DependencyIncompatible DependencyKind = "incompatible"
	DependencyEmbedded     DependencyKind = "embedded"
)

// Dependency references another project or version required by a version.
type Dependency struct {
	ProjectID string
	VersionID string
	FileName  string
	Kind      DependencyKind
}

// Version is a concrete release of a project.
type Version struct {
	Provider      Provider
	ID            string
	ProjectID     string
	Name          string
	VersionNumber string
	Changelog     string
	Channel       ReleaseChannel
	GameVersions  []string
	Loaders       []string
	PublishedAt   time.Time
	Downloads     int64
	Files         []File
	Dependencies  []Dependency
}

// PrimaryFile returns the preferred downloadable artifact of a version. It
// falls back to any downloadable file when the primary one is unavailable.
func (v Version) PrimaryFile() (File, error) {
	if len(v.Files) == 0 {
		return File{}, fmt.Errorf("versão %q: %w", v.ID, ErrNoFiles)
	}

	for _, file := range v.Files {
		if file.Primary && file.Downloadable() {
			return file, nil
		}
	}

	for _, file := range v.Files {
		if file.Downloadable() {
			return file, nil
		}
	}

	return File{}, fmt.Errorf("versão %q: %w", v.ID, ErrDownloadUnavailable)
}

// Links groups the external pages associated with a project.
type Links struct {
	Website string
	Source  string
	Issues  string
	Wiki    string
	Discord string
}

// Project is a provider-agnostic summary of a mod, shader, resource pack, etc.
type Project struct {
	Provider        Provider
	ID              string
	Slug            string
	Name            string
	Summary         string
	Description     string
	Authors         []string
	IconURL         string
	Links           Links
	License         string
	Categories      []string
	GameVersions    []string
	Loaders         []string
	Downloads       int64
	Followers       int64
	Type            ProjectKind
	LatestVersionID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Ref returns a lightweight, provider-scoped reference to the project.
func (p Project) Ref() ProjectRef {
	return ProjectRef{
		Provider: p.Provider,
		ID:       p.ID,
		Slug:     p.Slug,
		Name:     p.Name,
	}
}

// ProjectRef identifies a project without carrying its full metadata.
type ProjectRef struct {
	Provider Provider
	ID       string
	Slug     string
	Name     string
}

// SearchQuery describes a project search. Empty Text browses the provider.
type SearchQuery struct {
	Text        string
	Kind        ProjectKind
	GameVersion string
	Loader      string
	Categories  []string
	Sort        SortBy
	Limit       int
	Offset      int
}

// SearchResult is a page of projects returned by Client.Search.
type SearchResult struct {
	Projects []Project
	Total    int
	Offset   int
	Limit    int
}

// VersionFilter narrows the versions returned for a project.
type VersionFilter struct {
	GameVersion string
	Loader      string
	Channels    []ReleaseChannel
}
