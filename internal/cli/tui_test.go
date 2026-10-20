package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func newTUITestModel(t *testing.T, client modsource.Client, downloader *download.Client) *tuiModel {
	t.Helper()

	inst, _ := openTestCLIInstance(t)
	if err := inst.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	store := loadTestManifest(t, inst)

	return newTUIModel(context.Background(), client, downloader, inst, store)
}

func newTUITestModelWithContent(
	t *testing.T,
	client modsource.Client,
	downloader *download.Client,
	configure func(*instance.Instance, *manifest.Store),
) *tuiModel {
	t.Helper()

	inst, _ := openTestCLIInstance(t)
	if err := inst.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	store := loadTestManifest(t, inst)
	if configure != nil {
		configure(inst, store)
	}

	return newTUIModel(context.Background(), client, downloader, inst, store)
}

func executeTUICmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()

	if cmd == nil {
		return nil
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		if msg == nil {
			return nil
		}

		return []tea.Msg{msg}
	}

	msgs := make([]tea.Msg, 0, len(batch))
	for _, subcommand := range batch {
		if subcommand == nil {
			continue
		}
		if submsg := subcommand(); submsg != nil {
			msgs = append(msgs, submsg)
		}
	}

	return msgs
}

func findTUIInstallMsg(t *testing.T, msgs []tea.Msg) tuiInstallMsg {
	t.Helper()

	for _, msg := range msgs {
		if value, ok := msg.(tuiInstallMsg); ok {
			return value
		}
	}
	t.Fatalf("nenhuma tuiInstallMsg encontrada em %#v", msgs)

	return tuiInstallMsg{}
}

func findTUIUpdateMsg(t *testing.T, msgs []tea.Msg) tuiUpdateMsg {
	t.Helper()

	for _, msg := range msgs {
		if value, ok := msg.(tuiUpdateMsg); ok {
			return value
		}
	}
	t.Fatalf("nenhuma tuiUpdateMsg encontrada em %#v", msgs)

	return tuiUpdateMsg{}
}

func findTUIRemoveMsg(t *testing.T, msgs []tea.Msg) tuiRemoveMsg {
	t.Helper()

	for _, msg := range msgs {
		if value, ok := msg.(tuiRemoveMsg); ok {
			return value
		}
	}
	t.Fatalf("nenhuma tuiRemoveMsg encontrada em %#v", msgs)

	return tuiRemoveMsg{}
}

func TestTUISearchQueryAndFilters(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	model.query.SetValue(" sodium ")
	model.gameVersion.SetValue("1.21.1")
	model.loader.SetValue("fabric")
	model.typeIndex = 1
	model.channelIndex = 0

	query := model.searchQuery()
	if query.Text != "sodium" || query.Kind != modsource.KindMod {
		t.Fatalf("searchQuery() = %#v", query)
	}
	if query.GameVersion != "1.21.1" || query.Loader != "fabric" {
		t.Fatalf("searchQuery() = %#v", query)
	}
	if query.Limit != tuiSearchLimit {
		t.Fatalf("Limit = %d, want %d", query.Limit, tuiSearchLimit)
	}

	filter := model.versionFilter()
	if filter.GameVersion != "1.21.1" || filter.Loader != "fabric" {
		t.Fatalf("versionFilter() = %#v", filter)
	}
	if len(filter.Channels) != 1 || filter.Channels[0] != modsource.ChannelRelease {
		t.Fatalf("Channels = %#v, want release", filter.Channels)
	}
	if !model.channelExplicit() {
		t.Fatal("channelExplicit() = false, want true for release")
	}

	model.channelIndex = 3
	filter = model.versionFilter()
	if len(filter.Channels) != 0 {
		t.Fatalf("Channels = %#v, want all channels", filter.Channels)
	}
	if model.channelExplicit() {
		t.Fatal("channelExplicit() = true, want false for all channels")
	}
}

func TestTUIInstalledItemsIncludeTrackedAndUntracked(t *testing.T) {
	var entry manifest.Entry
	model := newTUITestModelWithContent(t, &fakeProviderClient{}, nil, func(inst *instance.Instance, store *manifest.Store) {
		entry = sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
		if err := store.ReplaceProject(entry); err != nil {
			t.Fatalf("ReplaceProject() error = %v", err)
		}
		writeTestFile(t, inst, entry, "tracked")
		manual := filepath.Join(inst.Path, instance.ModsDirName, "manual.jar")
		if err := os.WriteFile(manual, []byte("manual"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	})

	if len(model.installed) != 2 {
		t.Fatalf("installed items = %d, want 2", len(model.installed))
	}
	if !model.installed[0].tracked || model.installed[0].Key() != "mods/sodium.jar" {
		t.Fatalf("first installed item = %#v", model.installed[0])
	}
	if model.installed[0].status != "ok" {
		t.Fatalf("tracked status = %q, want ok", model.installed[0].status)
	}
	if model.installed[1].tracked || model.installed[1].Key() != "mods/manual.jar" {
		t.Fatalf("second installed item = %#v", model.installed[1])
	}
	if model.installed[1].status != "não rastreado" {
		t.Fatalf("untracked status = %q", model.installed[1].status)
	}
}

func TestTUISearchMessageSelectsFirstResultAndIgnoresStaleMessages(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	model.busy = true
	model.seq = 5

	project := modsource.Project{
		ID:   "AANobbMI",
		Slug: "sodium",
		Name: "Sodium",
		Type: modsource.KindMod,
	}
	updated, _ := model.Update(tuiSearchMsg{
		seq:   5,
		query: modsource.SearchQuery{Text: "sodium"},
		result: modsource.SearchResult{
			Total:    1,
			Projects: []modsource.Project{project},
		},
	})
	got := updated.(tuiModel)

	if got.busy {
		t.Fatal("busy = true, want false")
	}
	if got.focus != tuiFocusResults {
		t.Fatalf("focus = %d, want results", got.focus)
	}
	if len(got.searchProjects) != 1 || got.searchTotal != 1 {
		t.Fatalf("search state = %#v", got.searchProjects)
	}
	item, ok := got.searchList.SelectedItem().(tuiProjectItem)
	if !ok || item.project.ID != project.ID {
		t.Fatalf("selected item = %#v", got.searchList.SelectedItem())
	}

	updated, _ = got.Update(tuiSearchMsg{
		seq:    4,
		query:  modsource.SearchQuery{Text: "outro"},
		result: modsource.SearchResult{Projects: []modsource.Project{{ID: "outro"}}},
	})
	stale := updated.(tuiModel)
	if len(stale.searchProjects) != 1 || stale.searchProjects[0].ID != project.ID {
		t.Fatalf("stale message changed search result = %#v", stale.searchProjects)
	}
}

func TestTUIProjectMessagePopulatesVersions(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	model.busy = true
	model.seq = 7

	project := modsource.Project{
		ID:   "AANobbMI",
		Slug: "sodium",
		Name: "Sodium",
		Type: modsource.KindMod,
	}
	version := modsource.Version{
		ID:            "v1",
		ProjectID:     project.ID,
		VersionNumber: "1.0.0",
		Channel:       modsource.ChannelRelease,
	}
	updated, _ := model.Update(tuiProjectMsg{
		seq:             7,
		project:         project,
		versions:        []modsource.Version{version},
		filter:          modsource.VersionFilter{GameVersion: "1.21.1", Loader: "fabric"},
		channelExplicit: true,
	})
	got := updated.(tuiModel)

	if got.project == nil || got.project.ID != project.ID {
		t.Fatalf("project = %#v", got.project)
	}
	if len(got.versions) != 1 {
		t.Fatalf("versions = %#v", got.versions)
	}
	item, ok := got.versionList.SelectedItem().(tuiVersionItem)
	if !ok || item.version.ID != version.ID {
		t.Fatalf("selected version = %#v", got.versionList.SelectedItem())
	}
	if got.busy {
		t.Fatal("busy = true, want false")
	}
}

func TestTUIMergeProjectFallback(t *testing.T) {
	details := modsource.Project{ID: "AANobbMI", Name: "Sodium"}
	fallback := modsource.Project{
		Slug:      "sodium",
		Type:      modsource.KindMod,
		Authors:   []string{"CaffeineMC"},
		Downloads: 1000,
	}

	merged := mergeTUIProjectFallback(details, fallback)
	if merged.ID != details.ID || merged.Name != details.Name {
		t.Fatalf("dados do projeto foram sobrescritos: %#v", merged)
	}
	if merged.Slug != "sodium" || merged.Type != modsource.KindMod {
		t.Fatalf("fallback de slug/tipo não aplicado: %#v", merged)
	}
	if len(merged.Authors) != 1 || merged.Authors[0] != "CaffeineMC" || merged.Downloads != 1000 {
		t.Fatalf("fallback de autores/downloads não aplicado: %#v", merged)
	}
}

func TestTUIOutputOverwriteShortcut(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	model.openOutput("Erro", "o arquivo já existe")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(tuiModel)
	if !got.overwrite {
		t.Fatal("overwrite = false, want true after o")
	}
	if got.overlay != tuiOverlayNone {
		t.Fatalf("overlay = %d, want none", got.overlay)
	}
	if !strings.Contains(got.status, "Sobrescrita ativada") {
		t.Fatalf("status = %q, want overwrite confirmation", got.status)
	}
}

func TestTUIErrorGuidanceForOverwrite(t *testing.T) {
	if guidance := tuiErrorGuidance(download.ErrTargetExists, false); !strings.Contains(guidance, "Pressione o") {
		t.Fatalf("guidance = %q, want overwrite hint", guidance)
	}
	if guidance := tuiErrorGuidance(download.ErrTargetExists, true); guidance != "" {
		t.Fatalf("guidance = %q, want empty when overwrite is enabled", guidance)
	}
	if guidance := tuiErrorGuidance(errors.New("outro erro"), false); guidance != "" {
		t.Fatalf("guidance = %q, want empty for unrelated errors", guidance)
	}
}

func TestTUIInstallRejectsUnsupportedKind(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	project := modsource.Project{
		ID:   "modpack",
		Name: "Modpack",
		Type: modsource.KindModpack,
	}
	version := modsource.Version{
		ID:        "v1",
		ProjectID: project.ID,
		Files: []modsource.File{{
			Name:    "pack.zip",
			URL:     "https://example.invalid/pack.zip",
			Primary: true,
		}},
	}
	model.setProject(project, []modsource.Version{version}, modsource.VersionFilter{}, false)

	if cmd := model.startInstall(); cmd != nil {
		t.Fatal("startInstall() returned a command for an unsupported kind")
	}
	if !strings.Contains(model.status, "não suportado") {
		t.Fatalf("status = %q, want unsupported kind error", model.status)
	}
}

func TestTUIConfirmRemoveCancels(t *testing.T) {
	var entry manifest.Entry
	model := newTUITestModelWithContent(t, &fakeProviderClient{}, nil, func(inst *instance.Instance, store *manifest.Store) {
		entry = sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
		if err := store.ReplaceProject(entry); err != nil {
			t.Fatalf("ReplaceProject() error = %v", err)
		}
		writeTestFile(t, inst, entry, "tracked")
	})

	model.openRemoveConfirm()
	if model.overlay != tuiOverlayConfirm {
		t.Fatal("overlay = none, want confirm")
	}
	if model.confirm.query != "mods/sodium.jar" || model.confirm.entry != modsource.KindMod {
		t.Fatalf("confirm = %#v", model.confirm)
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(tuiModel)
	if got.overlay != tuiOverlayNone {
		t.Fatalf("overlay = %d, want none", got.overlay)
	}
	if !strings.Contains(got.status, "cancelada") {
		t.Fatalf("status = %q, want cancellation", got.status)
	}
	if _, err := os.Stat(filepath.Join(model.inst.Path, "mods", "sodium.jar")); err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
}

func TestTUIConfirmRemoveDeletesTrackedFile(t *testing.T) {
	var entry manifest.Entry
	model := newTUITestModelWithContent(t, &fakeProviderClient{}, nil, func(inst *instance.Instance, store *manifest.Store) {
		entry = sampleEntry(modsource.KindMod, "sodium", "v1", "sodium.jar")
		if err := store.ReplaceProject(entry); err != nil {
			t.Fatalf("ReplaceProject() error = %v", err)
		}
		writeTestFile(t, inst, entry, "tracked")
	})

	model.openRemoveConfirm()
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(tuiModel)
	if !got.busy {
		t.Fatal("busy = false, want true")
	}

	msg := findTUIRemoveMsg(t, executeTUICmd(t, cmd))
	if msg.err != nil {
		t.Fatalf("remove error = %v", msg.err)
	}
	updated, _ = got.Update(msg)
	got = updated.(tuiModel)

	if len(got.installed) != 0 {
		t.Fatalf("installed items = %d, want 0", len(got.installed))
	}
	if _, err := os.Stat(filepath.Join(got.inst.Path, "mods", "sodium.jar")); !os.IsNotExist(err) {
		t.Fatalf("file still exists or stat failed: %v", err)
	}
	if !strings.Contains(got.status, "removido") {
		t.Fatalf("status = %q, want success", got.status)
	}
}

func TestTUIInstallSelectedVersion(t *testing.T) {
	content := []byte("conteúdo do sodium")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	project := modsource.Project{
		ID:   "AANobbMI",
		Slug: "sodium",
		Name: "Sodium",
		Type: modsource.KindMod,
	}
	version := modsource.Version{
		ID:            "v1",
		ProjectID:     project.ID,
		VersionNumber: "1.0.0",
		Channel:       modsource.ChannelRelease,
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		PublishedAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Files: []modsource.File{{
			Name:    "sodium.jar",
			URL:     server.URL,
			Primary: true,
			Hashes:  installHashes(content),
		}},
	}
	client := &fakeProviderClient{
		project: project,
		version: version,
		versions: []modsource.Version{
			version,
		},
	}
	model := newTUITestModel(t, client, installTestDownloader(server))
	model.installDependencies = false
	model.setProject(
		project,
		[]modsource.Version{version},
		modsource.VersionFilter{GameVersion: "1.21.1", Loader: "fabric"},
		true,
	)

	cmd := model.startInstall()
	if cmd == nil || !model.busy {
		t.Fatalf("startInstall() cmd = %v, busy = %v", cmd, model.busy)
	}

	msg := findTUIInstallMsg(t, executeTUICmd(t, cmd))
	if msg.err != nil {
		t.Fatalf("install error = %v\n%s", msg.err, msg.output)
	}
	if len(msg.outcomes) != 1 {
		t.Fatalf("outcomes = %d, want 1", len(msg.outcomes))
	}

	updated, _ := model.Update(msg)
	got := updated.(tuiModel)
	if len(got.installed) != 1 || !got.installed[0].tracked {
		t.Fatalf("installed items = %#v", got.installed)
	}

	target := filepath.Join(got.inst.Path, "mods", "sodium.jar")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("installed file not found: %v", err)
	}
	if entries := got.store.Entries(); len(entries) != 1 || entries[0].VersionID != version.ID {
		t.Fatalf("manifest entries = %#v", entries)
	}
}

func TestTUIUpdateSelectedCheck(t *testing.T) {
	var entry manifest.Entry
	project := modsource.Project{
		ID:   "AANobbMI",
		Slug: "sodium",
		Name: "Sodium",
		Type: modsource.KindMod,
	}
	version := modsource.Version{
		ID:            "v1",
		ProjectID:     project.ID,
		VersionNumber: "1.0.0",
		Channel:       modsource.ChannelRelease,
		GameVersions:  []string{"1.21.1"},
		Loaders:       []string{"fabric"},
		PublishedAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Files: []modsource.File{{
			Name:    "sodium.jar",
			URL:     "https://example.invalid/sodium.jar",
			Primary: true,
		}},
	}

	client := &fakeProviderClient{
		project:  project,
		version:  version,
		versions: []modsource.Version{version},
	}
	model := newTUITestModelWithContent(t, client, nil, func(inst *instance.Instance, store *manifest.Store) {
		entry = sampleEntry(modsource.KindMod, project.ID, version.ID, "sodium.jar")
		if err := store.ReplaceProject(entry); err != nil {
			t.Fatalf("ReplaceProject() error = %v", err)
		}
		writeTestFile(t, inst, entry, "tracked")
	})

	cmd := model.startUpdate(true, false)
	if cmd == nil || !model.busy {
		t.Fatalf("startUpdate() cmd = %v, busy = %v", cmd, model.busy)
	}

	msg := findTUIUpdateMsg(t, executeTUICmd(t, cmd))
	if msg.err != nil {
		t.Fatalf("update check error = %v\n%s", msg.err, msg.output)
	}
	if !strings.Contains(msg.output, "Já atualizado") {
		t.Fatalf("update output = %q, want up-to-date status", msg.output)
	}
	if !msg.check {
		t.Fatal("check = false, want true")
	}
}

func TestTUIInstalledTogglesOverwriteAndDependencies(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	model.tab = tuiTabInstalled

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := updated.(tuiModel)
	if got.installDependencies {
		t.Fatal("installDependencies = true, want false after d")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got = updated.(tuiModel)
	if !got.overwrite {
		t.Fatal("overwrite = false, want true after o")
	}
}

func TestTUITabKeysWorkWhileTyping(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)
	model.focus = tuiFocusQuery

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyF2})
	got := updated.(tuiModel)
	if got.tab != tuiTabInstalled {
		t.Fatalf("tab = %d, want installed", got.tab)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyF1})
	got = updated.(tuiModel)
	if got.tab != tuiTabSearch {
		t.Fatalf("tab = %d, want search", got.tab)
	}
	if got.focus != tuiFocusQuery {
		t.Fatalf("focus = %d, want query", got.focus)
	}
}

func TestTUIViewFitsTerminalSizes(t *testing.T) {
	sizes := [][2]int{{80, 24}, {40, 20}, {120, 40}}

	for _, size := range sizes {
		width, height := size[0], size[1]
		model := newTUITestModel(t, &fakeProviderClient{}, nil)
		model.resize(width, height)

		views := map[string]string{"busca": model.View()}
		model.openHelp()
		views["ajuda"] = model.View()
		model.openOutput("Saída", "linha curta\n"+strings.Repeat("conteúdo longo ", 12))
		views["saida"] = model.View()
		model.overlay = tuiOverlayNone
		model.confirm = tuiConfirm{title: "Confirmar", body: "Remover um arquivo muito importante da instância?"}
		model.overlay = tuiOverlayConfirm
		views["confirmacao"] = model.View()

		for name, view := range views {
			lines := strings.Split(view, "\n")
			if len(lines) > height {
				t.Fatalf("%dx%d %s: %d linhas, máximo %d", width, height, name, len(lines), height)
			}
			for index, line := range lines {
				if lipgloss.Width(line) > width {
					t.Fatalf("%dx%d %s: linha %d tem largura %d: %q", width, height, name, index+1, lipgloss.Width(line), line)
				}
			}
		}
	}
}

func TestTUIViewRendersTabsAndHelp(t *testing.T) {
	model := newTUITestModel(t, &fakeProviderClient{}, nil)

	view := model.View()
	for _, fragment := range []string{"mcjava-manager", "Buscar", "Instalados", "Resultados"} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("view is missing %q:\n%s", fragment, view)
		}
	}

	model.openHelp()
	help := model.View()
	if !strings.Contains(help, "Ajuda") {
		t.Fatalf("help view = %q", help)
	}
}
