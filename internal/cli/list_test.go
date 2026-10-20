package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func executeListCommand(t *testing.T, globalOpts *options, args ...string) (string, string, error) {
	t.Helper()

	return executeCommand(t, newListCommand(globalOpts), args...)
}

func TestListCommandShowsTrackedAndUntrackedContent(t *testing.T) {
	inst, path := openTestCLIInstance(t)

	sodium := sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
	createInstalledEntry(t, inst, sodium, "sodium")
	pack := sampleEntry(modsource.KindResourcePack, "faithful", "v2", "faithful.zip")
	createInstalledEntry(t, inst, pack, "pack")

	if err := os.WriteFile(filepath.Join(path, "mods", "manual.jar"), []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	out, errOut, err := executeListCommand(t, &options{InstancePath: path})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}

	for _, want := range []string{
		"Conteúdo rastreado: 2",
		"sodium",
		"v1",
		"mods/sodium.jar",
		"faithful",
		"resourcepacks/faithful.zip",
		"Arquivos não rastreados: 1",
		"mods/manual.jar",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestListCommandFiltersByType(t *testing.T) {
	inst, path := openTestCLIInstance(t)

	createInstalledEntry(t, inst, sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar"), "sodium")
	createInstalledEntry(t, inst, sampleEntry(modsource.KindShaderPack, "complementary", "v2", "complementary.zip"), "shader")
	if err := os.WriteFile(filepath.Join(path, "mods", "manual.jar"), []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "shaderpacks", "manual.zip"), []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	out, _, err := executeListCommand(t, &options{InstancePath: path}, "--type", "mod")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out, "Conteúdo rastreado: 1") || !strings.Contains(out, "mods/sodium.jar") {
		t.Fatalf("stdout missing mod entry:\n%s", out)
	}
	if strings.Contains(out, "complementary") || strings.Contains(out, "shaderpacks/manual.zip") {
		t.Fatalf("stdout contains filtered content:\n%s", out)
	}
	if !strings.Contains(out, "mods/manual.jar") {
		t.Fatalf("stdout missing untracked mod:\n%s", out)
	}
}

func TestListCommandReportsMissingFile(t *testing.T) {
	inst, path := openTestCLIInstance(t)

	entry := sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
	if err := loadTestManifest(t, inst).ReplaceProject(entry); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	out, _, err := executeListCommand(t, &options{InstancePath: path})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "ausente") {
		t.Fatalf("stdout = %q, want missing status", out)
	}
}

func TestListCommandEmptyInstance(t *testing.T) {
	_, path := openTestCLIInstance(t)

	out, _, err := executeListCommand(t, &options{InstancePath: path})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"Conteúdo rastreado: 0",
		"Nenhum conteúdo rastreado.",
		"Arquivos não rastreados: 0",
		"Nenhum arquivo não rastreado",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(path, manifest.StateDirName)); !os.IsNotExist(err) {
		t.Fatalf("list created the manifest state directory: %v", err)
	}
}

func TestListCommandSkipsSymlinkedAndBrokenEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a criação de links simbólicos exige privilégios no Windows")
	}

	_, path := openTestCLIInstance(t)
	mods := filepath.Join(path, "mods")
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(mods, "linked-dir")); err != nil {
		t.Skipf("não foi possível criar link simbólico: %v", err)
	}
	if err := os.Symlink(filepath.Join(target, "missing.jar"), filepath.Join(mods, "broken.jar")); err != nil {
		t.Skipf("não foi possível criar link simbólico: %v", err)
	}

	out, _, err := executeListCommand(t, &options{InstancePath: path})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(out, "linked-dir") || strings.Contains(out, "broken.jar") {
		t.Fatalf("stdout contains non-file entries:\n%s", out)
	}
	if !strings.Contains(out, "Arquivos não rastreados: 0") {
		t.Fatalf("stdout = %q, want no untracked files", out)
	}
}

func TestListCommandRejectsUnsupportedType(t *testing.T) {
	_, path := openTestCLIInstance(t)

	_, _, err := executeListCommand(t, &options{InstancePath: path}, "--type", "world")
	if err == nil || !strings.Contains(err.Error(), "use mod, shader ou resourcepack") {
		t.Fatalf("Execute() error = %v, want type guidance", err)
	}
}

func TestListCommandRejectsInvalidManifest(t *testing.T) {
	inst, path := openTestCLIInstance(t)
	stateDir := filepath.Join(inst.Path, manifest.StateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, manifest.FileName), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, err := executeListCommand(t, &options{InstancePath: path})
	if err == nil || !strings.Contains(err.Error(), "manifesto") {
		t.Fatalf("Execute() error = %v, want manifest error", err)
	}
}
