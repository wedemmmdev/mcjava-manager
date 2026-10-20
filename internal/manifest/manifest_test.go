package manifest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func openTestInstance(t *testing.T) *instance.Instance {
	t.Helper()

	inst, err := instance.Open(t.TempDir())
	if err != nil {
		t.Fatalf("instance.Open() error = %v", err)
	}

	return inst
}

func testEntry() Entry {
	return Entry{
		Provider:      modsource.ProviderModrinth,
		ProjectID:     "AANobbMI",
		ProjectSlug:   "sodium",
		ProjectName:   "Sodium",
		Type:          modsource.KindMod,
		VersionID:     "v1",
		VersionNumber: "1.0",
		Channel:       modsource.ChannelRelease,
		GameVersion:   "1.21.1",
		Loader:        "fabric",
		GameVersions:  []string{"1.21.1", "1.21"},
		Loaders:       []string{"fabric"},
		FileName:      "sodium.jar",
		FilePath:      "mods/sodium.jar",
		FileSize:      123,
		Hashes:        map[string]string{"sha512": "abc"},
		InstalledAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestLoadMissingManifestReturnsEmptyStore(t *testing.T) {
	inst := openTestInstance(t)

	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(store.Entries()) != 0 {
		t.Fatalf("Entries() = %v, want empty", store.Entries())
	}
	if _, err := os.Stat(filepath.Join(inst.Path, StateDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state directory was created: %v", err)
	}
}

func TestReplaceProjectWritesAndReloads(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	entry := testEntry()
	entry.Hashes = map[string]string{"sha1": "def", "sha512": "abc", "": "ignored"}
	if err := store.ReplaceProject(entry); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	reloaded, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() after save error = %v", err)
	}
	entries := reloaded.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() = %v, want one entry", entries)
	}
	got := entries[0]
	if got.ProjectID != entry.ProjectID || got.FilePath != entry.FilePath || got.VersionID != entry.VersionID {
		t.Fatalf("entry = %+v, want %+v", got, entry)
	}
	if got.Hashes["sha512"] != "abc" || got.Hashes["sha1"] != "def" {
		t.Fatalf("Hashes = %v", got.Hashes)
	}
	if _, ok := got.Hashes[""]; ok {
		t.Fatalf("Hashes contains an empty algorithm: %v", got.Hashes)
	}

	assertNoManifestTempFiles(t, filepath.Join(inst.Path, StateDirName))
}

func TestReplaceProjectReplacesPreviousVersion(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	old := testEntry()
	if err := store.ReplaceProject(old); err != nil {
		t.Fatalf("ReplaceProject(old) error = %v", err)
	}

	updated := testEntry()
	updated.VersionID = "v2"
	updated.VersionNumber = "2.0"
	updated.FileName = "sodium-2.jar"
	updated.FilePath = "mods/sodium-2.jar"
	if err := store.ReplaceProject(updated); err != nil {
		t.Fatalf("ReplaceProject(updated) error = %v", err)
	}

	entries := store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() = %v, want one entry", entries)
	}
	if entries[0].VersionID != "v2" || entries[0].FilePath != "mods/sodium-2.jar" {
		t.Fatalf("entry = %+v, want updated", entries[0])
	}
}

func TestReplaceProjectRejectsFilePathOwnedByAnotherProject(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	first := testEntry()
	if err := store.ReplaceProject(first); err != nil {
		t.Fatalf("ReplaceProject(first) error = %v", err)
	}

	second := testEntry()
	second.ProjectID = "OTHER"
	second.ProjectSlug = "other"
	second.ProjectName = "Other"
	if err := store.ReplaceProject(second); err == nil || !strings.Contains(err.Error(), "já está registrado") {
		t.Fatalf("ReplaceProject(second) error = %v, want conflict", err)
	}
}

func TestEntriesReturnsCopies(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.ReplaceProject(testEntry()); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	entries := store.Entries()
	entries[0].Hashes["sha512"] = "mutated"
	entries[0].GameVersions[0] = "mutated"
	entries[0].ProjectName = "mutated"

	reloaded := store.Entries()
	if reloaded[0].Hashes["sha512"] != "abc" {
		t.Fatalf("Hashes were mutated: %v", reloaded[0].Hashes)
	}
	if reloaded[0].GameVersions[0] != "1.21.1" {
		t.Fatalf("GameVersions were mutated: %v", reloaded[0].GameVersions)
	}
	if reloaded[0].ProjectName != "Sodium" {
		t.Fatalf("ProjectName was mutated: %q", reloaded[0].ProjectName)
	}
}

func TestRemoveByPath(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.ReplaceProject(testEntry()); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	removed, ok, err := store.RemoveByPath("mods/sodium.jar")
	if err != nil {
		t.Fatalf("RemoveByPath() error = %v", err)
	}
	if !ok || removed.ProjectID != "AANobbMI" {
		t.Fatalf("RemoveByPath() = %+v, %v", removed, ok)
	}
	if len(store.Entries()) != 0 {
		t.Fatalf("Entries() = %v, want empty", store.Entries())
	}

	_, ok, err = store.RemoveByPath("mods/sodium.jar")
	if err != nil {
		t.Fatalf("RemoveByPath() second error = %v", err)
	}
	if ok {
		t.Fatal("RemoveByPath() reported a second removal")
	}
}

func TestRemoveProject(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.ReplaceProject(testEntry()); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	removed, err := store.RemoveProject(modsource.ProviderModrinth, "AANobbMI")
	if err != nil {
		t.Fatalf("RemoveProject() error = %v", err)
	}
	if len(removed) != 1 || removed[0].FilePath != "mods/sodium.jar" {
		t.Fatalf("removed = %+v", removed)
	}
	if len(store.Entries()) != 0 {
		t.Fatalf("Entries() = %v, want empty", store.Entries())
	}
}

func TestNormalizeEntryRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Entry)
		want   string
	}{
		{name: "provider", mutate: func(e *Entry) { e.Provider = "invalid" }, want: "provedor inválido"},
		{name: "project", mutate: func(e *Entry) { e.ProjectID = " " }, want: "ID do projeto"},
		{name: "kind", mutate: func(e *Entry) { e.Type = "invalid" }, want: "tipo de conteúdo inválido"},
		{name: "unsupported kind", mutate: func(e *Entry) { e.Type = modsource.KindWorld }, want: "não suportado"},
		{name: "version", mutate: func(e *Entry) { e.VersionID = "" }, want: "ID da versão"},
		{name: "channel", mutate: func(e *Entry) { e.Channel = "stable" }, want: "canal inválido"},
		{name: "empty path", mutate: func(e *Entry) { e.FilePath = "" }, want: "caminho do arquivo"},
		{name: "absolute path", mutate: func(e *Entry) { e.FilePath = "/etc/passwd" }, want: "relativo"},
		{name: "traversal", mutate: func(e *Entry) { e.FilePath = "mods/../../etc/passwd" }, want: "escapa"},
		{name: "subdirectory", mutate: func(e *Entry) { e.FilePath = "mods/sub/sodium.jar" }, want: "diretamente"},
		{name: "wrong folder", mutate: func(e *Entry) { e.FilePath = "shaderpacks/sodium.jar" }, want: "não pertence"},
		{name: "mismatched file name", mutate: func(e *Entry) { e.FileName = "other.jar" }, want: "não corresponde"},
		{name: "unsafe file name", mutate: func(e *Entry) { e.FilePath = "mods/bad:name.jar"; e.FileName = "bad:name.jar" }, want: "caractere inválido"},
		{name: "negative size", mutate: func(e *Entry) { e.FileSize = -1 }, want: "negativo"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := testEntry()
			test.mutate(&entry)

			_, err := normalizeEntry(entry)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("normalizeEntry() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsInvalidManifest(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "invalid json", content: "{", want: "inválido"},
		{name: "wrong schema", content: `{"schema":2,"entries":[]}`, want: "esquema 2"},
		{
			name: "duplicate path",
			content: `{"schema":1,"entries":[
				{"provider":"modrinth","project_id":"A","type":"mod","version_id":"1","file_path":"mods/a.jar"},
				{"provider":"modrinth","project_id":"B","type":"mod","version_id":"1","file_path":"mods/a.jar"}
			]}`,
			want: "mais de uma vez",
		},
		{
			name: "duplicate project",
			content: `{"schema":1,"entries":[
				{"provider":"modrinth","project_id":"A","type":"mod","version_id":"1","file_path":"mods/a.jar"},
				{"provider":"modrinth","project_id":"A","type":"mod","version_id":"2","file_path":"mods/b.jar"}
			]}`,
			want: "projeto",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inst := openTestInstance(t)
			stateDir := filepath.Join(inst.Path, StateDirName)
			if err := os.MkdirAll(stateDir, 0o755); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.WriteFile(filepath.Join(stateDir, FileName), []byte(test.content), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			_, err := Load(inst)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsOversizedManifest(t *testing.T) {
	inst := openTestInstance(t)
	stateDir := filepath.Join(inst.Path, StateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := make([]byte, maxFileBytes+1)
	for index := range content {
		content[index] = ' '
	}
	if err := os.WriteFile(filepath.Join(stateDir, FileName), content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(inst)
	if err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("Load() error = %v, want size limit", err)
	}
}

func TestEntryLabelsSanitizeControlCharacters(t *testing.T) {
	entry := Entry{
		ProjectName:   "Sodium\nInjected\u2028Again\u200e",
		VersionNumber: "1.0\x1b[31m",
	}

	if got := entry.ProjectLabel(); got != "Sodium Injected Again" {
		t.Fatalf("ProjectLabel() = %q", got)
	}
	if got := entry.VersionLabel(); got != "1.0[31m" {
		t.Fatalf("VersionLabel() = %q", got)
	}
}

func TestPathsEqual(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		a        string
		b        string
		foldCase bool
		want     bool
	}{
		{name: "exact", a: "mods/sodium.jar", b: "mods/sodium.jar", want: true},
		{name: "different", a: "mods/sodium.jar", b: "mods/iris.jar", want: false},
		{name: "case sensitive", a: "mods/sodium.jar", b: "mods/Sodium.jar", want: false},
		{name: "case folded", a: "mods/sodium.jar", b: "mods/Sodium.jar", foldCase: true, want: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := pathsEqual(testCase.a, testCase.b, testCase.foldCase); got != testCase.want {
				t.Fatalf("pathsEqual(%q, %q, %v) = %v, want %v", testCase.a, testCase.b, testCase.foldCase, got, testCase.want)
			}
		})
	}
}

func TestRelativePathAndTargetPath(t *testing.T) {
	pathValue, err := RelativePath(modsource.KindMod, "sodium.jar")
	if err != nil {
		t.Fatalf("RelativePath() error = %v", err)
	}
	if pathValue != "mods/sodium.jar" {
		t.Fatalf("RelativePath() = %q", pathValue)
	}

	entry := testEntry()
	target, err := entry.TargetPath("/tmp/instance")
	if err != nil {
		t.Fatalf("TargetPath() error = %v", err)
	}
	if target != filepath.Join("/tmp/instance", "mods", "sodium.jar") {
		t.Fatalf("TargetPath() = %q", target)
	}

	if _, err := RelativePath(modsource.KindWorld, "world.zip"); err == nil {
		t.Fatal("RelativePath() accepted world")
	}
	if _, err := RelativePath(modsource.KindMod, "../sodium.jar"); err == nil {
		t.Fatal("RelativePath() accepted a path separator")
	}
}

func TestFindByPathAndProject(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.ReplaceProject(testEntry()); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	entry, ok := store.FindByPath(`mods\sodium.jar`)
	if !ok || entry.ProjectID != "AANobbMI" {
		t.Fatalf("FindByPath() = %+v, %v", entry, ok)
	}
	if entries := store.FindByProject(modsource.ProviderModrinth, "AANobbMI"); len(entries) != 1 {
		t.Fatalf("FindByProject() = %v", entries)
	}
	if entries := store.FindByProject(modsource.ProviderCurseForge, "AANobbMI"); len(entries) != 0 {
		t.Fatalf("FindByProject() = %v, want empty", entries)
	}
	if _, ok := store.FindByPath("mods/../sodium.jar"); ok {
		t.Fatal("FindByPath() matched a traversal path")
	}
}

func TestManifestIsValidJSON(t *testing.T) {
	inst := openTestInstance(t)
	store, err := Load(inst)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.ReplaceProject(testEntry()); err != nil {
		t.Fatalf("ReplaceProject() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(inst.Path, StateDirName, FileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if doc["schema"].(float64) != SchemaVersion {
		t.Fatalf("schema = %v", doc["schema"])
	}
}

func assertNoManifestTempFiles(t *testing.T, directory string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".installed-") {
			t.Fatalf("temporary manifest file %q was not removed", entry.Name())
		}
	}
}
