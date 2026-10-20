// Package modsource defines the provider-agnostic abstractions used to search,
// inspect, and download Minecraft content from mod repositories.
package modsource

import (
	"fmt"
	"strings"
)

// Provider identifies a remote content repository.
type Provider string

const (
	ProviderModrinth   Provider = "modrinth"
	ProviderCurseForge Provider = "curseforge"
)

// DisplayName returns the provider name suitable for user-facing output.
func (p Provider) DisplayName() string {
	switch p {
	case ProviderModrinth:
		return "Modrinth"
	case ProviderCurseForge:
		return "CurseForge"
	default:
		return string(p)
	}
}

// RequiresAPIKey reports whether requests to the provider need credentials.
func (p Provider) RequiresAPIKey() bool {
	return p == ProviderCurseForge
}

// APIKeyURL returns where users can obtain credentials for the provider.
func (p Provider) APIKeyURL() string {
	if p == ProviderCurseForge {
		return "https://console.curseforge.com/"
	}

	return ""
}

// Valid reports whether the provider is supported.
func (p Provider) Valid() bool {
	return p == ProviderModrinth || p == ProviderCurseForge
}

func (p Provider) String() string {
	return string(p)
}

// ParseProvider converts a user-supplied provider name into a Provider.
func ParseProvider(value string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "modrinth", "mr":
		return ProviderModrinth, nil
	case "curseforge", "curse-forge", "cf":
		return ProviderCurseForge, nil
	default:
		return "", fmt.Errorf("provedor desconhecido %q: use %q ou %q", value, ProviderModrinth, ProviderCurseForge)
	}
}
