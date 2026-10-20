package instance

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open("   "); err == nil {
		t.Fatal("Open() error = nil, want an error")
	}
}

func TestOpenRejectsMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-existe")

	_, err := Open(path)
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("Open() error = %v, want ErrInstanceNotFound", err)
	}
}

func TestOpenRejectsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo.txt")
	if err := os.WriteFile(path, []byte("conteúdo"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Open(path)
	if !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("Open() error = %v, want ErrNotDirectory", err)
	}
}

func TestOpenAcceptsArbitraryDirectoryName(t *testing.T) {
	names := []string{
		"minha-instancia",
		"qualquer pasta 1.20.1",
		"fabric-nao-e-requisito",
		"servidor-de-amigos",
		"インスタンス",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name)
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("Mkdir() error = %v", err)
			}

			inst, err := Open(path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if inst.Path != path {
				t.Fatalf("Instance.Path = %q, want %q", inst.Path, path)
			}
		})
	}
}

func TestOpenResolvesRelativePath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	inst, err := Open(".")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if inst.Path != root {
		t.Fatalf("Instance.Path = %q, want %q", inst.Path, root)
	}
}

func TestDirectories(t *testing.T) {
	root := t.TempDir()
	inst := &Instance{Path: root}

	dirs := inst.Directories()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "mods", got: dirs.Mods, want: filepath.Join(root, ModsDirName)},
		{name: "shaderpacks", got: dirs.ShaderPacks, want: filepath.Join(root, ShaderPacksDirName)},
		{name: "resourcepacks", got: dirs.ResourcePacks, want: filepath.Join(root, ResourcePacksDirName)},
		{name: "saves", got: dirs.Saves, want: filepath.Join(root, SavesDirName)},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s path = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestDirectoriesOnNilInstance(t *testing.T) {
	var inst *Instance

	dirs := inst.Directories()
	if dirs != (Directories{}) {
		t.Fatalf("Directories() = %+v, want zero value", dirs)
	}
}

func TestRequiredDirectoryNamesReturnsCopy(t *testing.T) {
	want := []string{ModsDirName, ShaderPacksDirName, ResourcePacksDirName, SavesDirName}

	got := RequiredDirectoryNames()
	if len(got) != len(want) {
		t.Fatalf("RequiredDirectoryNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RequiredDirectoryNames() = %v, want %v", got, want)
		}
	}

	got[0] = "alterado"
	if RequiredDirectoryNames()[0] == "alterado" {
		t.Fatal("RequiredDirectoryNames() must return a new slice on every call")
	}
}

func TestPrepareCreatesLayout(t *testing.T) {
	root := t.TempDir()

	inst, err := Prepare(root)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if inst.Path != root {
		t.Fatalf("Instance.Path = %q, want %q", inst.Path, root)
	}

	for _, name := range RequiredDirectoryNames() {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", path)
		}
	}
}

func TestPrepareIsIdempotentAndPreservesContent(t *testing.T) {
	root := t.TempDir()
	mods := filepath.Join(root, ModsDirName)
	if err := os.MkdirAll(mods, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	marker := filepath.Join(mods, "meu-mod.jar")
	if err := os.WriteFile(marker, []byte("conteúdo"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	for range 2 {
		if _, err := Prepare(root); err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "conteúdo" {
		t.Fatalf("marker content = %q, want conteúdo", data)
	}
}

func TestPrepareRejectsFileInPlaceOfDirectory(t *testing.T) {
	root := t.TempDir()
	mods := filepath.Join(root, ModsDirName)
	if err := os.WriteFile(mods, []byte("não sou diretório"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Prepare(root)
	if err == nil {
		t.Fatal("Prepare() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "não é um diretório") {
		t.Fatalf("Prepare() error = %v, want a not-a-directory error", err)
	}
}

func TestEnsureLayoutRejectsInvalidInstance(t *testing.T) {
	var inst *Instance
	if err := inst.EnsureLayout(); err == nil {
		t.Fatal("EnsureLayout() error = nil, want an error")
	}

	inst = &Instance{}
	if err := inst.EnsureLayout(); err == nil {
		t.Fatal("EnsureLayout() error = nil, want an error")
	}
}

func TestPermissionError(t *testing.T) {
	err := &PermissionError{
		Operation: "criar a pasta",
		Path:      "/caminho/protegido/mods",
		Err:       fs.ErrPermission,
	}

	message := err.Error()
	for _, fragment := range []string{"permissão negada", "criar a pasta", err.Path} {
		if !strings.Contains(message, fragment) {
			t.Errorf("PermissionError = %q, missing %q", message, fragment)
		}
	}

	if !errors.Is(err, fs.ErrPermission) {
		t.Error("PermissionError must unwrap to fs.ErrPermission")
	}
	if !IsPermissionError(err) {
		t.Error("IsPermissionError() = false, want true")
	}
	if IsPermissionError(errors.New("outro erro")) {
		t.Error("IsPermissionError() = true for an unrelated error")
	}
}

func TestWrapFSError(t *testing.T) {
	permErr := wrapFSError("criar a pasta", "/tmp/mods", fs.ErrPermission)

	var target *PermissionError
	if !errors.As(permErr, &target) {
		t.Fatalf("wrapFSError() = %T, want *PermissionError", permErr)
	}
	if !errors.Is(permErr, fs.ErrPermission) {
		t.Error("wrapFSError() must preserve fs.ErrPermission")
	}

	otherErr := wrapFSError("criar a pasta", "/tmp/mods", fs.ErrExist)
	if errors.As(otherErr, &target) {
		t.Fatal("wrapFSError() should not return *PermissionError for a non-permission error")
	}
	if !errors.Is(otherErr, fs.ErrExist) {
		t.Error("wrapFSError() must preserve the original error")
	}
}

func TestPermissionGuidance(t *testing.T) {
	t.Run("termux", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Termux guidance is Unix-only")
		}

		t.Setenv("TERMUX_VERSION", "0.118.0")
		t.Setenv("PREFIX", "/data/data/com.termux/files/usr")

		guidance := permissionGuidance("/storage/emulated/0/mods")
		if !strings.Contains(guidance, "termux-setup-storage") {
			t.Fatalf("guidance = %q, want termux-setup-storage", guidance)
		}
	})

	t.Run("linux", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("Linux-specific guidance")
		}
		t.Setenv("TERMUX_VERSION", "")
		t.Setenv("PREFIX", "")

		guidance := permissionGuidance("/home/user/mods")
		if !strings.Contains(guidance, "chmod u+w") {
			t.Fatalf("guidance = %q, want chmod guidance", guidance)
		}
	})
}
