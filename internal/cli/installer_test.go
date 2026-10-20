package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func artifactServer(t *testing.T, contents map[string]string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		content, ok := contents[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, content)
	}))
	t.Cleanup(server.Close)

	return server, &calls
}

func artifactFile(server *httptest.Server, pathValue, name, content string) modsource.File {
	return modsource.File{
		Name:    name,
		URL:     server.URL + pathValue,
		Primary: true,
		Hashes:  installHashes([]byte(content)),
	}
}

func TestInstallCommandInstallsRequiredDependencies(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root.jar":   "root",
		"/fabric.jar": "fabric",
	})

	rootVersion := modsource.Version{
		ID:            "root-v1",
		ProjectID:     "ROOT",
		VersionNumber: "1.0",
		Channel:       modsource.ChannelRelease,
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{artifactFile(server, "/root.jar", "root.jar", "root")},
		Dependencies: []modsource.Dependency{{
			ProjectID: "FABRIC",
			VersionID: "fabric-v1",
			Kind:      modsource.DependencyRequired,
		}},
	}
	fabricVersion := modsource.Version{
		ID:            "fabric-v1",
		ProjectID:     "FABRIC",
		VersionNumber: "0.100.0",
		Channel:       modsource.ChannelRelease,
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{artifactFile(server, "/fabric.jar", "fabric.jar", "fabric")},
	}

	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v1": fabricVersion,
		},
	}

	inst, path := openTestCLIInstance(t)
	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
		"--game-version", "1.21.1",
		"--loader", "fabric",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"Instalado dependência Fabric API 0.100.0",
		"Instalado Root Mod 1.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Fabric API") > strings.Index(out, "Root Mod") {
		t.Fatalf("dependency was not installed before the root project:\n%s", out)
	}

	rootEntry := sampleEntry(modsource.KindMod, "ROOT", "root-v1", "root.jar")
	fabricEntry := sampleEntry(modsource.KindMod, "FABRIC", "fabric-v1", "fabric.jar")
	for _, item := range []struct {
		entry   manifest.Entry
		content string
	}{
		{entry: rootEntry, content: "root"},
		{entry: fabricEntry, content: "fabric"},
	} {
		target, targetErr := item.entry.TargetPath(path)
		if targetErr != nil {
			t.Fatalf("TargetPath() error = %v", targetErr)
		}
		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", target, readErr)
		}
		if string(got) != item.content {
			t.Errorf("content of %q = %q, want %q", target, got, item.content)
		}
	}

	store := loadTestManifest(t, inst)
	if entries := store.FindByProject(modsource.ProviderModrinth, "ROOT"); len(entries) != 1 {
		t.Fatalf("root manifest entries = %v", entries)
	}
	if entries := store.FindByProject(modsource.ProviderModrinth, "FABRIC"); len(entries) != 1 {
		t.Fatalf("dependency manifest entries = %v", entries)
	}
}

func TestInstallCommandResolvesUnpinnedDependency(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root.jar":   "root",
		"/fabric.jar": "fabric",
	})
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/root.jar", "root.jar", "root")},
		Dependencies: []modsource.Dependency{{
			ProjectID: "FABRIC",
			Kind:      modsource.DependencyRequired,
		}},
	}
	fabricVersion := modsource.Version{
		ID:            "fabric-v1",
		ProjectID:     "FABRIC",
		VersionNumber: "0.100.0",
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{artifactFile(server, "/fabric.jar", "fabric.jar", "fabric")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{"root-v1": rootVersion},
		versionsByProject: map[string][]modsource.Version{
			"FABRIC": {fabricVersion},
		},
	}

	_, path := openTestCLIInstance(t)
	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
		"--game-version", "1.21.1",
		"--loader", "fabric",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Instalado dependência Fabric API 0.100.0") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "fabric.jar")); err != nil {
		t.Fatalf("dependency file missing: %v", err)
	}
	if client.versionsID != "FABRIC" {
		t.Fatalf("ListVersions project = %q, want FABRIC", client.versionsID)
	}
	if client.versionFilter.GameVersion != "1.21.1" || client.versionFilter.Loader != "fabric" {
		t.Fatalf("dependency filter = %+v", client.versionFilter)
	}
}

func TestInstallCommandResolvesDependencyWithVersionOnly(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root.jar":   "root",
		"/fabric.jar": "fabric",
	})
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/root.jar", "root.jar", "root")},
		Dependencies: []modsource.Dependency{{
			VersionID: "fabric-v1",
			Kind:      modsource.DependencyRequired,
		}},
	}
	fabricVersion := modsource.Version{
		ID:            "fabric-v1",
		ProjectID:     "FABRIC",
		VersionNumber: "0.100.0",
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{artifactFile(server, "/fabric.jar", "fabric.jar", "fabric")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v1": fabricVersion,
		},
	}

	_, path := openTestCLIInstance(t)
	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Instalado dependência Fabric API 0.100.0") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "fabric.jar")); err != nil {
		t.Fatalf("dependency file missing: %v", err)
	}
}

func TestInstallCommandRejectsDependencyVersionProjectMismatch(t *testing.T) {
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files: []modsource.File{{
			Name:    "root.jar",
			URL:     "https://example.com/root.jar",
			Primary: true,
			Hashes:  installHashes([]byte("root")),
		}},
		Dependencies: []modsource.Dependency{{
			ProjectID: "OTHER",
			VersionID: "fabric-v1",
			Kind:      modsource.DependencyRequired,
		}},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":  {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"OTHER": {ID: "OTHER", Name: "Other Mod", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v1": {ID: "fabric-v1", ProjectID: "FABRIC"},
		},
	}

	_, path := openTestCLIInstance(t)
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		download.New(),
		"ROOT",
		"--version", "root-v1",
	)
	if err == nil || !strings.Contains(err.Error(), "pertence ao projeto") {
		t.Fatalf("Execute() error = %v, want project mismatch", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "root.jar")); !os.IsNotExist(statErr) {
		t.Fatalf("root file was installed: %v", statErr)
	}
}

func TestInstallCommandRejectsDependencyWithoutProject(t *testing.T) {
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files: []modsource.File{{
			Name:    "root.jar",
			URL:     "https://example.com/root.jar",
			Primary: true,
			Hashes:  installHashes([]byte("root")),
		}},
		Dependencies: []modsource.Dependency{{
			VersionID: "orphan-v1",
			Kind:      modsource.DependencyRequired,
		}},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT": {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"orphan-v1": {ID: "orphan-v1"},
		},
	}

	_, path := openTestCLIInstance(t)
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		download.New(),
		"ROOT",
		"--version", "root-v1",
	)
	if err == nil || !strings.Contains(err.Error(), "não informa o projeto associado") {
		t.Fatalf("Execute() error = %v, want orphan dependency error", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "root.jar")); !os.IsNotExist(statErr) {
		t.Fatalf("root file was installed: %v", statErr)
	}
}

func TestInstallCommandCanSkipDependencies(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root.jar":   "root",
		"/fabric.jar": "fabric",
	})

	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/root.jar", "root.jar", "root")},
		Dependencies: []modsource.Dependency{{
			ProjectID: "FABRIC",
			VersionID: "fabric-v1",
			Kind:      modsource.DependencyRequired,
		}},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v1": {ID: "fabric-v1", ProjectID: "FABRIC", Files: []modsource.File{artifactFile(server, "/fabric.jar", "fabric.jar", "fabric")}},
		},
	}

	inst, path := openTestCLIInstance(t)
	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
		"--no-dependencies",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(out, "Fabric API") {
		t.Fatalf("stdout contains dependency:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root.jar")); err != nil {
		t.Fatalf("root file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "fabric.jar")); !os.IsNotExist(err) {
		t.Fatalf("dependency file exists: %v", err)
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 1 {
		t.Fatalf("manifest entries = %v, want one", entries)
	}
}

func TestInstallCommandRejectsDependencyVersionConflict(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root.jar":   "root",
		"/fabric.jar": "fabric",
	})

	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/root.jar", "root.jar", "root")},
		Dependencies: []modsource.Dependency{{
			ProjectID: "FABRIC",
			VersionID: "fabric-v2",
			Kind:      modsource.DependencyRequired,
		}},
	}
	fabricVersion := modsource.Version{
		ID:           "fabric-v2",
		ProjectID:    "FABRIC",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/fabric.jar", "fabric-v2.jar", "fabric v2")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v2": fabricVersion,
		},
	}

	inst, path := openTestCLIInstance(t)
	oldEntry := sampleEntry(modsource.KindMod, "FABRIC", "fabric-v1", "fabric-v1.jar")
	createInstalledEntry(t, inst, oldEntry, "fabric v1")

	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
	)
	if err == nil || !strings.Contains(err.Error(), "já está instalada") || !strings.Contains(err.Error(), "fabric-v1") {
		t.Fatalf("Execute() error = %v, want dependency conflict", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "root.jar")); !os.IsNotExist(statErr) {
		t.Fatalf("root file was installed despite conflict: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "fabric-v1.jar")); statErr != nil {
		t.Fatalf("old dependency file was removed: %v", statErr)
	}
}

func TestInstallCommandHandlesDependencyCycle(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/a.jar": "a",
		"/b.jar": "b",
	})

	versionA := modsource.Version{
		ID:           "a-v1",
		ProjectID:    "A",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/a.jar", "a.jar", "a")},
		Dependencies: []modsource.Dependency{{ProjectID: "B", VersionID: "b-v1", Kind: modsource.DependencyRequired}},
	}
	versionB := modsource.Version{
		ID:           "b-v1",
		ProjectID:    "B",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/b.jar", "b.jar", "b")},
		Dependencies: []modsource.Dependency{{ProjectID: "A", VersionID: "a-v1", Kind: modsource.DependencyRequired}},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"A": {ID: "A", Name: "Mod A", Type: modsource.KindMod},
			"B": {ID: "B", Name: "Mod B", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"a-v1": versionA,
			"b-v1": versionB,
		},
	}

	inst, path := openTestCLIInstance(t)
	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"A",
		"--version", "a-v1",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Instalado dependência Mod B") || !strings.Contains(out, "Instalado Mod A") {
		t.Fatalf("stdout = %q", out)
	}
	for _, name := range []string{"a.jar", "b.jar"} {
		if _, err := os.Stat(filepath.Join(path, "mods", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 2 {
		t.Fatalf("manifest entries = %v, want two", entries)
	}
}

func TestInstallCommandRejectsDependencyWithoutReference(t *testing.T) {
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files: []modsource.File{{
			Name:    "root.jar",
			URL:     "https://example.com/root.jar",
			Primary: true,
			Hashes:  installHashes([]byte("root")),
		}},
		Dependencies: []modsource.Dependency{{Kind: modsource.DependencyRequired}},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT": {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{"root-v1": rootVersion},
	}

	_, path := openTestCLIInstance(t)
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		download.New(),
		"ROOT",
		"--version", "root-v1",
	)
	if err == nil || !strings.Contains(err.Error(), "não informa projeto nem versão") {
		t.Fatalf("Execute() error = %v, want missing reference error", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "root.jar")); !os.IsNotExist(statErr) {
		t.Fatalf("root file was installed: %v", statErr)
	}
}

func TestInstallCommandRejectsIncompatibleDependency(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/fabric.jar": "fabric",
	})
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files: []modsource.File{{
			Name:    "root.jar",
			URL:     "https://example.com/root.jar",
			Primary: true,
			Hashes:  installHashes([]byte("root")),
		}},
		Dependencies: []modsource.Dependency{{ProjectID: "FABRIC", VersionID: "fabric-v1", Kind: modsource.DependencyRequired}},
	}
	fabricVersion := modsource.Version{
		ID:           "fabric-v1",
		ProjectID:    "FABRIC",
		GameVersions: []string{"1.20.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/fabric.jar", "fabric.jar", "fabric")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v1": fabricVersion,
		},
	}

	_, path := openTestCLIInstance(t)
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		download.New(),
		"ROOT",
		"--version", "root-v1",
		"--game-version", "1.21.1",
	)
	if err == nil || !strings.Contains(err.Error(), "não é compatível") {
		t.Fatalf("Execute() error = %v, want incompatibility", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "root.jar")); !os.IsNotExist(statErr) {
		t.Fatalf("root file was installed: %v", statErr)
	}
}

func TestInstallCommandSkipsAlreadyInstalledVersion(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root.jar": "new root"})
	rootVersion := modsource.Version{
		ID:            "root-v1",
		ProjectID:     "ROOT",
		VersionNumber: "1.0",
		Files:         []modsource.File{artifactFile(server, "/root.jar", "root.jar", "new root")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT": {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{"root-v1": rootVersion},
	}

	inst, path := openTestCLIInstance(t)
	entry := sampleEntry(modsource.KindMod, "ROOT", "root-v1", "root.jar")
	entry.ProjectName = "Root Mod"
	createInstalledEntry(t, inst, entry, "old root")

	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
	if !strings.Contains(out, "Já instalado Root Mod 1.0") {
		t.Fatalf("stdout = %q", out)
	}
	content, err := os.ReadFile(filepath.Join(path, "mods", "root.jar"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "old root" {
		t.Fatalf("content = %q, want unchanged", content)
	}
}

func TestInstallCommandRepairsMissingDependencyOfInstalledRoot(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/fabric.jar": "fabric"})
	rootVersion := modsource.Version{
		ID:            "root-v1",
		ProjectID:     "ROOT",
		VersionNumber: "1.0",
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files: []modsource.File{{
			Name:    "root.jar",
			URL:     "https://example.com/root.jar",
			Primary: true,
			Hashes:  installHashes([]byte("root")),
		}},
		Dependencies: []modsource.Dependency{{ProjectID: "FABRIC", VersionID: "fabric-v1", Kind: modsource.DependencyRequired}},
	}
	fabricVersion := modsource.Version{
		ID:            "fabric-v1",
		ProjectID:     "FABRIC",
		VersionNumber: "0.100.0",
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{artifactFile(server, "/fabric.jar", "fabric.jar", "fabric")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT":   {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
			"FABRIC": {ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{
			"root-v1":   rootVersion,
			"fabric-v1": fabricVersion,
		},
	}

	inst, path := openTestCLIInstance(t)
	entry := sampleEntry(modsource.KindMod, "ROOT", "root-v1", "root.jar")
	entry.ProjectName = "Root Mod"
	entry.VersionNumber = "1.0"
	createInstalledEntry(t, inst, entry, "root")

	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("server calls = %d, want 1", calls.Load())
	}
	if !strings.Contains(out, "Instalado dependência Fabric API") || !strings.Contains(out, "Já instalado Root Mod") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "fabric.jar")); err != nil {
		t.Fatalf("dependency was not installed: %v", err)
	}
}

func TestInstallCommandReplacesPreviousVersionFile(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{"/root-new.jar": "new root"})
	rootVersion := modsource.Version{
		ID:            "root-v2",
		ProjectID:     "ROOT",
		VersionNumber: "2.0",
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{artifactFile(server, "/root-new.jar", "root-new.jar", "new root")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT": {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{"root-v2": rootVersion},
	}

	inst, path := openTestCLIInstance(t)
	oldEntry := sampleEntry(modsource.KindMod, "ROOT", "root-v1", "root-old.jar")
	oldEntry.ProjectName = "Root Mod"
	oldEntry.ProjectSlug = "root"
	oldEntry.VersionNumber = "1.0"
	createInstalledEntry(t, inst, oldEntry, "old root")

	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v2",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Atualizado Root Mod 1.0 -> 2.0") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-new.jar")); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-old.jar")); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: %v", err)
	}
	entries := loadTestManifest(t, inst).FindByProject(modsource.ProviderModrinth, "ROOT")
	if len(entries) != 1 || entries[0].VersionID != "root-v2" {
		t.Fatalf("manifest entries = %+v", entries)
	}
}

func TestInstallCommandRejectsTargetOwnedByAnotherProject(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/shared.jar": "root"})
	rootVersion := modsource.Version{
		ID:           "root-v1",
		ProjectID:    "ROOT",
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
		Files:        []modsource.File{artifactFile(server, "/shared.jar", "shared.jar", "root")},
	}
	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT": {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
		},
		versionsByID: map[string]modsource.Version{"root-v1": rootVersion},
	}

	inst, path := openTestCLIInstance(t)
	other := sampleEntry(modsource.KindMod, "OTHER", "other-v1", "shared.jar")
	createInstalledEntry(t, inst, other, "other")

	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"ROOT",
		"--version", "root-v1",
	)
	if err == nil || !strings.Contains(err.Error(), "já está registrado") {
		t.Fatalf("Execute() error = %v, want target conflict", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
	content, err := os.ReadFile(filepath.Join(path, "mods", "shared.jar"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "other" {
		t.Fatalf("content = %q, want other", content)
	}
}
