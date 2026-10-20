package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wedemmmdev/modink/internal/download"
	"github.com/wedemmmdev/modink/internal/manifest"
	"github.com/wedemmmdev/modink/pkg/modsource"
)

func executeUpdateCommand(t *testing.T, client modsource.Client, globalOpts *options, downloader *download.Client, args ...string) (string, string, error) {
	t.Helper()

	return executeCommand(t, newUpdateCommand(client, globalOpts, downloader), args...)
}

func updateTestVersion(
	id, projectID, versionNumber, filename, downloadPath string,
	file modsource.File,
	dependencies ...modsource.Dependency,
) modsource.Version {
	return modsource.Version{
		ID:            id,
		ProjectID:     projectID,
		VersionNumber: versionNumber,
		Channel:       modsource.ChannelRelease,
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		Files:         []modsource.File{file},
		Dependencies:  dependencies,
	}
}

func setupUpdateRoot(version modsource.Version) (*fakeProviderClient, *manifest.Entry) {
	entry := sampleEntry(modsource.KindMod, "ROOT", "root-v1", "root-v1.jar")
	entry.ProjectSlug = "root"
	entry.ProjectName = "Root Mod"
	entry.VersionNumber = "1.0"

	client := &fakeProviderClient{
		projects: map[string]modsource.Project{
			"ROOT": {ID: "ROOT", Name: "Root Mod", Type: modsource.KindMod},
		},
		versionsByProject: map[string][]modsource.Version{
			"ROOT": {version},
		},
	}

	return client, &entry
}

func TestUpdateCommandUpdatesAllTrackedContent(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root-v2.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")

	out, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("server calls = %d, want 1", calls.Load())
	}
	if !strings.Contains(out, "Atualizado Root Mod 1.0 -> 2.0") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v2.jar")); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v1.jar")); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: %v", err)
	}
	entries := loadTestManifest(t, inst).FindByProject(modsource.ProviderModrinth, "ROOT")
	if len(entries) != 1 || entries[0].VersionID != "root-v2" {
		t.Fatalf("manifest entries = %+v", entries)
	}
}

func TestUpdateCommandSameFilenameOverwrites(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root.jar",
		"/root.jar",
		artifactFile(server, "/root.jar", "root.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	entry.FileName = "root.jar"
	entry.FilePath = "mods/root.jar"
	createInstalledEntry(t, inst, *entry, "old root")

	if _, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server)); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("server calls = %d, want 1", calls.Load())
	}
	content, err := os.ReadFile(filepath.Join(path, "mods", "root.jar"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "new root" {
		t.Fatalf("content = %q, want new root", content)
	}
}

func TestUpdateCommandCheckDoesNotDownload(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root-v2.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")

	out, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"--check",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
	if !strings.Contains(out, "Atualização disponível: Root Mod 1.0 -> 2.0") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v1.jar")); err != nil {
		t.Fatalf("old file removed during check: %v", err)
	}
	entries := loadTestManifest(t, inst).FindByProject(modsource.ProviderModrinth, "ROOT")
	if len(entries) != 1 || entries[0].VersionID != "root-v1" {
		t.Fatalf("manifest changed during check: %+v", entries)
	}
}

func TestUpdateCommandUpToDate(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root-v1.jar": "root"})
	version := updateTestVersion(
		"root-v1",
		"ROOT",
		"1.0",
		"root-v1.jar",
		"/root-v1.jar",
		artifactFile(server, "/root-v1.jar", "root-v1.jar", "root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "root")

	out, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
	if !strings.Contains(out, "Já atualizado: Root Mod 1.0") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestUpdateCommandReinstallsMissingFile(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root-v1.jar": "root"})
	version := updateTestVersion(
		"root-v1",
		"ROOT",
		"1.0",
		"root-v1.jar",
		"/root-v1.jar",
		artifactFile(server, "/root-v1.jar", "root-v1.jar", "root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	persistTestEntry(t, inst, *entry)

	out, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("server calls = %d, want 1", calls.Load())
	}
	if !strings.Contains(out, "Reinstalado Root Mod 1.0") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v1.jar")); err != nil {
		t.Fatalf("file was not reinstalled: %v", err)
	}
}

func TestUpdateCommandCheckReportsMissingFile(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root-v1.jar": "root"})
	version := updateTestVersion(
		"root-v1",
		"ROOT",
		"1.0",
		"root-v1.jar",
		"/root-v1.jar",
		artifactFile(server, "/root-v1.jar", "root-v1.jar", "root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	persistTestEntry(t, inst, *entry)

	out, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"--check",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
	if !strings.Contains(out, "Reinstalação necessária") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestUpdateCommandUsesStoredFilters(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{"/root-v1.jar": "root"})
	version := updateTestVersion(
		"root-v1",
		"ROOT",
		"1.0",
		"root-v1.jar",
		"/root-v1.jar",
		artifactFile(server, "/root-v1.jar", "root-v1.jar", "root"),
	)
	client, entry := setupUpdateRoot(version)
	client.versionFiltersByProject = make(map[string]modsource.VersionFilter)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "root")

	if _, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"--check",
	); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	filter := client.versionFiltersByProject["ROOT"]
	if filter.GameVersion != "1.21.1" || filter.Loader != "fabric" {
		t.Fatalf("filter = %+v", filter)
	}
	if len(filter.Channels) != 1 || filter.Channels[0] != modsource.ChannelRelease {
		t.Fatalf("Channels = %v", filter.Channels)
	}
}

func TestUpdateCommandOverridesFilters(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{"/root-v1.jar": "root"})
	version := updateTestVersion(
		"root-v1",
		"ROOT",
		"1.0",
		"root-v1.jar",
		"/root-v1.jar",
		artifactFile(server, "/root-v1.jar", "root-v1.jar", "root"),
	)
	client, entry := setupUpdateRoot(version)
	client.versionFiltersByProject = make(map[string]modsource.VersionFilter)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "root")

	if _, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"--check",
		"--game-version", "1.20.1",
		"--loader", "forge",
		"--channel", "beta",
	); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	filter := client.versionFiltersByProject["ROOT"]
	if filter.GameVersion != "1.20.1" || filter.Loader != "forge" {
		t.Fatalf("filter = %+v", filter)
	}
	if len(filter.Channels) != 1 || filter.Channels[0] != modsource.ChannelBeta {
		t.Fatalf("Channels = %v", filter.Channels)
	}
}

func TestUpdateCommandSelectsSpecificEntry(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{"/root-v2.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")
	other := sampleEntry(modsource.KindResourcePack, "FAITHFUL", "faithful-v1", "faithful.zip")
	createInstalledEntry(t, inst, other, "pack")

	if _, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"root",
	); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v2.jar")); err != nil {
		t.Fatalf("root was not updated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "resourcepacks", "faithful.zip")); err != nil {
		t.Fatalf("unselected content was removed: %v", err)
	}
	entries := loadTestManifest(t, inst).FindByProject(modsource.ProviderModrinth, "FAITHFUL")
	if len(entries) != 1 || entries[0].VersionID != "faithful-v1" {
		t.Fatalf("unselected manifest entry changed: %+v", entries)
	}
}

func TestUpdateCommandUnknownEntryDoesNotModifyInstance(t *testing.T) {
	server, calls := artifactServer(t, map[string]string{"/root-v2.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")

	_, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"missing",
	)
	if err == nil || !strings.Contains(err.Error(), "não encontrado") {
		t.Fatalf("Execute() error = %v, want not found", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v1.jar")); err != nil {
		t.Fatalf("old file was removed: %v", err)
	}
}

func TestUpdateCommandInstallsNewDependencies(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root-v2.jar":   "new root",
		"/fabric-v1.jar": "fabric",
	})
	rootVersion := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
		modsource.Dependency{ProjectID: "FABRIC", VersionID: "fabric-v1", Kind: modsource.DependencyRequired},
	)
	fabricVersion := updateTestVersion(
		"fabric-v1",
		"FABRIC",
		"0.100.0",
		"fabric-v1.jar",
		"/fabric-v1.jar",
		artifactFile(server, "/fabric-v1.jar", "fabric-v1.jar", "fabric"),
	)

	client, entry := setupUpdateRoot(rootVersion)
	client.projects["FABRIC"] = modsource.Project{ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod}
	client.versionsByID = map[string]modsource.Version{"fabric-v1": fabricVersion}

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")

	out, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Instalado dependência Fabric API 0.100.0") || !strings.Contains(out, "Atualizado Root Mod 1.0 -> 2.0") {
		t.Fatalf("stdout = %q", out)
	}
	for _, name := range []string{"root-v2.jar", "fabric-v1.jar"} {
		if _, err := os.Stat(filepath.Join(path, "mods", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 2 {
		t.Fatalf("manifest entries = %v, want two", entries)
	}
}

func TestUpdateCommandReplacesDependencyVersion(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{
		"/root-v2.jar":   "new root",
		"/fabric-v2.jar": "fabric v2",
	})
	rootVersion := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
		modsource.Dependency{ProjectID: "FABRIC", VersionID: "fabric-v2", Kind: modsource.DependencyRequired},
	)
	fabricVersion := updateTestVersion(
		"fabric-v2",
		"FABRIC",
		"0.100.0",
		"fabric-v2.jar",
		"/fabric-v2.jar",
		artifactFile(server, "/fabric-v2.jar", "fabric-v2.jar", "fabric v2"),
	)
	client, entry := setupUpdateRoot(rootVersion)
	client.projects["FABRIC"] = modsource.Project{ID: "FABRIC", Name: "Fabric API", Type: modsource.KindMod}
	client.versionsByID = map[string]modsource.Version{"fabric-v2": fabricVersion}

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")
	oldFabric := sampleEntry(modsource.KindMod, "FABRIC", "fabric-v1", "fabric-v1.jar")
	createInstalledEntry(t, inst, oldFabric, "fabric v1")

	out, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server), "root")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Atualizado dependência Fabric API fabric-v1 -> 0.100.0") ||
		!strings.Contains(out, "Atualizado Root Mod 1.0 -> 2.0") {
		t.Fatalf("stdout = %q", out)
	}
	for _, name := range []string{"root-v2.jar", "fabric-v2.jar"} {
		if _, err := os.Stat(filepath.Join(path, "mods", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	for _, name := range []string{"root-v1.jar", "fabric-v1.jar"} {
		if _, err := os.Stat(filepath.Join(path, "mods", name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
	entries := loadTestManifest(t, inst).FindByProject(modsource.ProviderModrinth, "FABRIC")
	if len(entries) != 1 || entries[0].VersionID != "fabric-v2" {
		t.Fatalf("dependency manifest = %+v", entries)
	}
}

func TestUpdateCommandPreservesDirectoryAtOldPath(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{"/root-v2.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")
	oldPath := filepath.Join(path, "mods", "root-v1.jar")
	if err := os.Remove(oldPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	out, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server), "root")
	if err == nil || !strings.Contains(err.Error(), "diretório") {
		t.Fatalf("Execute() error = %v, want directory cleanup warning", err)
	}
	if !strings.Contains(out, "Atualizado Root Mod 1.0 -> 2.0") {
		t.Fatalf("stdout = %q", out)
	}
	info, statErr := os.Stat(oldPath)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("old directory was removed: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(path, "mods", "root-v2.jar")); statErr != nil {
		t.Fatalf("new root file missing: %v", statErr)
	}
	entries := loadTestManifest(t, inst).FindByProject(modsource.ProviderModrinth, "ROOT")
	if len(entries) != 1 || entries[0].VersionID != "root-v2" {
		t.Fatalf("manifest = %+v", entries)
	}
}

func TestUpdateCommandOverwriteUntrackedTarget(t *testing.T) {
	server, _ := artifactServer(t, map[string]string{"/root-v2.jar": "new root"})
	version := updateTestVersion(
		"root-v2",
		"ROOT",
		"2.0",
		"root-v2.jar",
		"/root-v2.jar",
		artifactFile(server, "/root-v2.jar", "root-v2.jar", "new root"),
	)
	client, entry := setupUpdateRoot(version)

	inst, path := openTestCLIInstance(t)
	createInstalledEntry(t, inst, *entry, "old root")
	manualTarget := filepath.Join(path, "mods", "root-v2.jar")
	if err := os.WriteFile(manualTarget, []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, err := executeUpdateCommand(t, client, &options{InstancePath: path}, installTestDownloader(server))
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("Execute() error = %v, want overwrite guidance", err)
	}
	content, err := os.ReadFile(manualTarget)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "manual" {
		t.Fatalf("untracked file was modified: %q", content)
	}

	if _, _, err := executeUpdateCommand(
		t,
		client,
		&options{InstancePath: path},
		installTestDownloader(server),
		"--overwrite",
	); err != nil {
		t.Fatalf("Execute() with --overwrite error = %v", err)
	}
	content, err = os.ReadFile(manualTarget)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "new root" {
		t.Fatalf("content = %q, want new root", content)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "root-v1.jar")); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: %v", err)
	}
}

func TestUpdateCommandRejectsOverwriteWithCheck(t *testing.T) {
	_, path := openTestCLIInstance(t)

	_, _, err := executeUpdateCommand(
		t,
		&fakeProviderClient{},
		&options{InstancePath: path},
		nil,
		"--check",
		"--overwrite",
	)
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("Execute() error = %v, want flag conflict", err)
	}
}

func TestUpdateCommandEmptyManifest(t *testing.T) {
	_, path := openTestCLIInstance(t)

	out, _, err := executeUpdateCommand(t, &fakeProviderClient{}, &options{InstancePath: path}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Nenhum conteúdo rastreado") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, manifest.StateDirName)); !os.IsNotExist(err) {
		t.Fatalf("update created the state directory: %v", err)
	}
}

func TestUpdateCommandProviderUnavailable(t *testing.T) {
	inst, path := openTestCLIInstance(t)
	entry := sampleEntry(modsource.KindMod, "CF1", "cf-v1", "cf.jar")
	entry.Provider = modsource.ProviderCurseForge
	createInstalledEntry(t, inst, entry, "cf")

	_, _, err := executeUpdateCommand(t, nil, &options{InstancePath: path}, nil)
	if err == nil || !strings.Contains(err.Error(), "CurseForge ainda não está implementado") {
		t.Fatalf("Execute() error = %v, want provider unavailable", err)
	}
}
