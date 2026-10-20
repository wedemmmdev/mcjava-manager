package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

func providerDisplayName(provider modsource.Provider) string {
	if name := provider.DisplayName(); name != "" {
		return sanitizeText(name)
	}
	if provider == "" {
		return "-"
	}

	return sanitizeText(string(provider))
}

func displayProjectType(kind modsource.ProjectKind) string {
	if kind == "" {
		return "-"
	}
	if display := kind.DisplayName(); display != "" {
		return sanitizeText(display)
	}

	return sanitizeText(string(kind))
}

func displayChannel(channel modsource.ReleaseChannel) string {
	if channel == "" {
		return "-"
	}

	return sanitizeText(string(channel))
}

func authorsText(authors []string) string {
	values := make([]string, 0, len(authors))
	for _, author := range authors {
		if trimmed := strings.TrimSpace(sanitizeText(author)); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return "-"
	}

	return strings.Join(values, ", ")
}

func projectIdentifier(project modsource.Project) string {
	if slug := strings.TrimSpace(sanitizeText(project.Slug)); slug != "" {
		return slug
	}

	return strings.TrimSpace(sanitizeText(project.ID))
}

func projectDisplayName(project modsource.Project) string {
	if name := strings.TrimSpace(sanitizeText(project.Name)); name != "" {
		return name
	}
	if identifier := projectIdentifier(project); identifier != "" {
		return identifier
	}

	return "(sem nome)"
}

func versionDisplayLabel(version modsource.Version) string {
	for _, value := range []string{version.VersionNumber, version.Name, version.ID} {
		if trimmed := strings.TrimSpace(sanitizeText(value)); trimmed != "" {
			return trimmed
		}
	}

	return "-"
}

func summarizeValues(values []string, max int) string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		value = strings.TrimSpace(sanitizeText(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}

	if len(cleaned) == 0 {
		return "-"
	}
	if max <= 0 || len(cleaned) <= max {
		return strings.Join(cleaned, ", ")
	}

	return fmt.Sprintf("%s (+%d)", strings.Join(cleaned[:max], ", "), len(cleaned)-max)
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(sanitizeText(value))
	if max <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max == 1 {
		return string(runes[:1])
	}

	return string(runes[:max-1]) + "…"
}

// sanitizeText removes control and bidirectional-override characters from
// untrusted provider data before it is written to a single-line terminal field.
func sanitizeText(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t' || r == 0x2028 || r == 0x2029:
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		case r >= 0x80 && r <= 0x9f:
			return -1
		case r == 0x061c || (r >= 0x200e && r <= 0x200f) ||
			(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			return -1
		default:
			return r
		}
	}, value)
}

// sanitizeBlock is the multiline variant of sanitizeText. It preserves line
// breaks and tabs for provider descriptions.
func sanitizeBlock(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r == 0x2028 || r == 0x2029:
			return '\n'
		case r < 0x20 || r == 0x7f:
			return -1
		case r >= 0x80 && r <= 0x9f:
			return -1
		case r == 0x061c || (r >= 0x200e && r <= 0x200f) ||
			(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			return -1
		default:
			return r
		}
	}, value)
}

func formatCount(value int64) string {
	if value < 0 {
		return "0"
	}

	switch {
	case value < 1_000:
		return strconv.FormatInt(value, 10)
	case value < 1_000_000:
		return compactCount(value, 1_000, "K")
	case value < 1_000_000_000:
		return compactCount(value, 1_000_000, "M")
	default:
		return compactCount(value, 1_000_000_000, "B")
	}
}

func compactCount(value, divisor int64, suffix string) string {
	formatted := strconv.FormatFloat(float64(value)/float64(divisor), 'f', 1, 64)
	formatted = strings.TrimSuffix(formatted, ".0")

	return formatted + suffix
}

func formatBytes(size int64) string {
	if size < 0 {
		return "0 B"
	}

	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	divisor, exponent := int64(unit), 0
	for value := size / unit; value >= unit && exponent < len("KMGTPE")-1; value /= unit {
		divisor *= unit
		exponent++
	}

	return fmt.Sprintf("%.1f %cB", float64(size)/float64(divisor), "KMGTPE"[exponent])
}
