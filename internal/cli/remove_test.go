package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

func executeRemoveCommand(t *testing.T, globalOpts *options, args ...string) (string, string, error) {
	t.Helper()

	return executeCommand(t, newRemoveCommand(globalOpts), args...)
}

func TestRemoveCommandRemovesTrackedProject(t *testing.T) {
	inst, path := openTestCLIInstance(t)
	entry := sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
	createInstalledEntry(t, inst, entry, "sodium")

	out, errOut, err := executeRemoveCommand(t, &options{InstancePath: path}, "sodium")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
	if !strings.Contains(out, "Removido") || !strings.Contains(out, "mods/sodium.jar") {
		t.Fatalf("stdout = %q", out)
	}

	target, _ := entry.TargetPath(path)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 0 {
		t.Fatalf("manifest entries = %v, want empty", entries)
	}
}

func TestRemoveCommandRemovesTrackedFileByAbsolutePath(t *testing.T) {
	inst, path := openTestCLIInstance(t)
	entry := sampleEntry(modsource.KindResourcePack, "faithful", "v1", "faithful.zip")
	createInstalledEntry(t, inst, entry, "pack")
	target, _ := entry.TargetPath(path)

	out, _, err := executeRemoveCommand(t, &options{InstancePath: path}, target)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "faithful.zip") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 0 {
		t.Fatalf("manifest entries = %v, want empty", entries)
	}
}

func TestRemoveCommandRemovesUntrackedFile(t *testing.T) {
	_, path := openTestCLIInstance(t)
	target := filepath.Join(path, "mods", "manual.jar")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(target, []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	out, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "manual.jar")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "não rastreado") || !strings.Contains(out, "mods/manual.jar") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestRemoveCommandRemovesManifestEntryWhenFileIsMissing(t *testing.T) {
	inst, path := openTestCLIInstance(t)
	entry := sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
	if err := loadTestManifest(t, inst).ReplaceProject(entry); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	out, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "sodium")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "já não existia") {
		t.Fatalf("stdout = %q", out)
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 0 {
		t.Fatalf("manifest entries = %v, want empty", entries)
	}
}

func TestRemoveCommandRejectsAmbiguousTrackedContent(t *testing.T) {
	inst, path := openTestCLIInstance(t)
	modEntry := sampleEntry(modsource.KindMod, "MOD", "mod-v1", "same.zip")
	createInstalledEntry(t, inst, modEntry, "mod")
	packEntry := sampleEntry(modsource.KindResourcePack, "PACK", "pack-v1", "same.zip")
	createInstalledEntry(t, inst, packEntry, "pack")

	_, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "same.zip")
	if err == nil || !strings.Contains(err.Error(), "mais de um conteúdo instalado") {
		t.Fatalf("Execute() error = %v, want tracked ambiguity", err)
	}
	if !strings.Contains(err.Error(), "mods/same.zip") || !strings.Contains(err.Error(), "resourcepacks/same.zip") {
		t.Fatalf("error does not list candidates: %v", err)
	}

	out, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "same.zip", "--type", "mod")
	if err != nil {
		t.Fatalf("Execute() with --type error = %v", err)
	}
	if !strings.Contains(out, "mods/same.zip") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "same.zip")); !os.IsNotExist(err) {
		t.Fatalf("mods/same.zip still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "resourcepacks", "same.zip")); err != nil {
		t.Fatalf("resourcepacks/same.zip was removed: %v", err)
	}
	if entries := loadTestManifest(t, inst).Entries(); len(entries) != 1 || entries[0].ProjectID != "PACK" {
		t.Fatalf("manifest entries = %+v", entries)
	}
}

func TestRemoveCommandRejectsAmbiguousFileName(t *testing.T) {
	_, path := openTestCLIInstance(t)
	for _, folder := range []string{"mods", "resourcepacks"} {
		target := filepath.Join(path, folder, "same.zip")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(target, []byte(folder), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	_, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "same.zip")
	if err == nil || !strings.Contains(err.Error(), "mais de um arquivo") {
		t.Fatalf("Execute() error = %v, want ambiguity error", err)
	}

	out, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "same.zip", "--type", "mod")
	if err != nil {
		t.Fatalf("Execute() with --type error = %v", err)
	}
	if !strings.Contains(out, "mods/same.zip") {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(filepath.Join(path, "mods", "same.zip")); !os.IsNotExist(err) {
		t.Fatalf("mods/same.zip still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "resourcepacks", "same.zip")); err != nil {
		t.Fatalf("resourcepacks/same.zip was removed: %v", err)
	}
}

func TestRemoveCommandRejectsPathsOutsideInstance(t *testing.T) {
	_, path := openTestCLIInstance(t)
	outside := filepath.Join(filepath.Dir(path), "outside.jar")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	for _, query := range []string{"../outside.jar", outside} {
		_, _, err := executeRemoveCommand(t, &options{InstancePath: path}, query)
		if err == nil || !strings.Contains(err.Error(), "fora da instância") {
			t.Fatalf("Execute(%q) error = %v, want outside-instance error", query, err)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
}

func TestRemoveCommandRejectsDirectory(t *testing.T) {
	_, path := openTestCLIInstance(t)
	directory := filepath.Join(path, "mods", "unpacked")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	_, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "unpacked")
	if err == nil || !strings.Contains(err.Error(), "diretório") {
		t.Fatalf("Execute() error = %v, want directory error", err)
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("directory was removed: %v", err)
	}
}

func TestRemoveCommandUnknownContent(t *testing.T) {
	_, path := openTestCLIInstance(t)

	_, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "missing")
	if err == nil || !strings.Contains(err.Error(), "não encontrado") {
		t.Fatalf("Execute() error = %v, want not found", err)
	}
}

func TestRemoveCommandRejectsUnsupportedType(t *testing.T) {
	_, path := openTestCLIInstance(t)

	_, _, err := executeRemoveCommand(t, &options{InstancePath: path}, "sodium", "--type", "world")
	if err == nil || !strings.Contains(err.Error(), "use mod, shader ou resourcepack") {
		t.Fatalf("Execute() error = %v, want type guidance", err)
	}
}
