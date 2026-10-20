package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func openTestCLIInstance(t *testing.T) (*instance.Instance, string) {
	t.Helper()

	path := t.TempDir()
	inst, err := instance.Open(path)
	if err != nil {
		t.Fatalf("instance.Open() error = %v", err)
	}

	return inst, path
}

func loadTestManifest(t *testing.T, inst *instance.Instance) *manifest.Store {
	t.Helper()

	store, err := manifest.Load(inst)
	if err != nil {
		t.Fatalf("manifest.Load() error = %v", err)
	}

	return store
}

func sampleEntry(kind modsource.ProjectKind, projectID, versionID, filename string) manifest.Entry {
	directory, _ := manifest.ContentDirectory(kind)

	return manifest.Entry{
		Provider:      modsource.ProviderModrinth,
		ProjectID:     projectID,
		ProjectSlug:   strings.ToLower(projectID),
		ProjectName:   projectID,
		Type:          kind,
		VersionID:     versionID,
		VersionNumber: versionID,
		Channel:       modsource.ChannelRelease,
		GameVersion:   "1.21.1",
		Loader:        "fabric",
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		FileName:      filename,
		FilePath:      directory + "/" + filename,
		InstalledAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
}

func persistTestEntry(t *testing.T, inst *instance.Instance, entry manifest.Entry) *manifest.Store {
	t.Helper()

	store := loadTestManifest(t, inst)
	if err := store.ReplaceProject(entry); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	return store
}

func writeTestFile(t *testing.T, inst *instance.Instance, entry manifest.Entry, content string) {
	t.Helper()

	target, err := entry.TargetPath(inst.Path)
	if err != nil {
		t.Fatalf("TargetPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func createInstalledEntry(t *testing.T, inst *instance.Instance, entry manifest.Entry, content string) *manifest.Store {
	t.Helper()

	writeTestFile(t, inst, entry, content)
	return persistTestEntry(t, inst, entry)
}
