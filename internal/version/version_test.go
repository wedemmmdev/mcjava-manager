package version

import (
	"strings"
	"testing"
)

func TestShort(t *testing.T) {
	original := Version
	t.Cleanup(func() {
		Version = original
	})

	Version = ""
	if got := Short(); got != devVersion {
		t.Fatalf("Short() = %q, want %q", got, devVersion)
	}

	Version = "v0.1.0"
	if got := Short(); got != "v0.1.0" {
		t.Fatalf("Short() = %q, want v0.1.0", got)
	}
}

func TestFull(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuildDate = originalBuildDate
	})

	Version = "v0.1.0"
	Commit = "0123456789abcdef"
	BuildDate = "2026-10-19"

	got := Full()
	for _, fragment := range []string{"v0.1.0", "0123456789ab", "2026-10-19", "go"} {
		if !strings.Contains(got, fragment) {
			t.Errorf("Full() = %q, missing %q", got, fragment)
		}
	}
	if strings.Contains(got, "0123456789abcdef") {
		t.Errorf("Full() = %q, commit should be truncated", got)
	}
}
