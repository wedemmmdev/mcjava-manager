package cli

import (
	"context"
	"crypto/sha1" // #nosec G505 -- test helper mirrors provider checksums.
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func installHashes(content []byte) []modsource.Hash {
	sha512sum := sha512.Sum512(content)
	sha1sum := sha1.Sum(content) // #nosec G401 -- compatibility test checksum.

	return []modsource.Hash{
		{Algorithm: modsource.HashSHA512, Value: hex.EncodeToString(sha512sum[:])},
		{Algorithm: modsource.HashSHA1, Value: hex.EncodeToString(sha1sum[:])},
	}
}

func executeInstallCommand(
	t *testing.T,
	client modsource.Client,
	globalOpts *options,
	downloader *download.Client,
	args ...string,
) (string, string, error) {
	t.Helper()

	return executeCommand(t, newInstallCommand(client, globalOpts, downloader), args...)
}

func installTestDownloader(server *httptest.Server) *download.Client {
	return download.New(
		download.WithHTTPClient(server.Client()),
		download.WithUserAgent("mcjava-manager/test"),
	)
}

func assertNoInstallTempFiles(t *testing.T, directory string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".mcjava-manager-") {
			t.Fatalf("temporary file %q was not removed", entry.Name())
		}
	}
}

func TestInstallCommandInstallsLatestCompatibleVersion(t *testing.T) {
	content := []byte("conteúdo do sodium")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "mcjava-manager/test" {
			t.Errorf("User-Agent = %q", got)
		}
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	versionFile := modsource.File{
		Name:    "sodium.jar",
		URL:     server.URL,
		Primary: true,
		Hashes:  installHashes(content),
	}
	client := &fakeProviderClient{
		project: modsource.Project{
			ID:   "AANobbMI",
			Slug: "sodium",
			Name: "Sodium",
			Type: modsource.KindMod,
		},
		versions: []modsource.Version{
			{
				ID:            "old",
				ProjectID:     "AANobbMI",
				VersionNumber: "1.0",
				Channel:       modsource.ChannelRelease,
				GameVersions:  []string{"1.21.1"},
				Loaders:       []string{"fabric"},
				PublishedAt:   time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Files: []modsource.File{{
					Name:    "sodium-old.jar",
					URL:     server.URL,
					Primary: true,
					Hashes:  installHashes(content),
				}},
			},
			{
				ID:            "new",
				ProjectID:     "AANobbMI",
				VersionNumber: "2.0",
				Channel:       modsource.ChannelRelease,
				GameVersions:  []string{"1.21.1"},
				Loaders:       []string{"fabric"},
				PublishedAt:   time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
				Files:         []modsource.File{versionFile},
			},
		},
	}

	instancePath := t.TempDir()
	out, errOut, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		installTestDownloader(server),
		"sodium",
		"--game-version", "1.21.1",
		"--loader", "fabric",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}

	if client.projectID != "sodium" {
		t.Errorf("projectID = %q, want sodium", client.projectID)
	}
	if client.versionsID != "AANobbMI" {
		t.Errorf("versionsID = %q, want AANobbMI", client.versionsID)
	}
	if client.versionFilter.GameVersion != "1.21.1" {
		t.Errorf("GameVersion = %q", client.versionFilter.GameVersion)
	}
	if client.versionFilter.Loader != "fabric" {
		t.Errorf("Loader = %q", client.versionFilter.Loader)
	}
	if len(client.versionFilter.Channels) != 1 || client.versionFilter.Channels[0] != modsource.ChannelRelease {
		t.Errorf("Channels = %v, want release", client.versionFilter.Channels)
	}

	target := filepath.Join(instancePath, instance.ModsDirName, "sodium.jar")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
	for _, folder := range []string{instance.ModsDirName, instance.ShaderPacksDirName, instance.ResourcePacksDirName, instance.SavesDirName} {
		info, err := os.Stat(filepath.Join(instancePath, folder))
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", folder, err)
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", folder)
		}
	}
	if !strings.Contains(out, "Instalado Sodium 2.0") {
		t.Errorf("stdout = %q, missing installation message", out)
	}
	if !strings.Contains(out, target) {
		t.Errorf("stdout = %q, missing target path", out)
	}
	assertNoInstallTempFiles(t, filepath.Join(instancePath, instance.ModsDirName))
}

func TestInstallCommandInstallsSpecificVersion(t *testing.T) {
	content := []byte("versão específica")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	client := &fakeProviderClient{
		project: modsource.Project{
			ID:   "AANobbMI",
			Name: "Sodium",
			Type: modsource.KindMod,
		},
		version: modsource.Version{
			ID:            "v2",
			ProjectID:     "AANobbMI",
			VersionNumber: "2.0",
			Channel:       modsource.ChannelRelease,
			GameVersions:  []string{"1.21.1"},
			Loaders:       []string{"fabric"},
			Files: []modsource.File{{
				Name:    "sodium.jar",
				URL:     server.URL,
				Primary: true,
				Hashes:  installHashes(content),
			}},
		},
	}

	instancePath := t.TempDir()
	out, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		installTestDownloader(server),
		"sodium",
		"--version", "v2",
		"--game-version", "1.21.1",
		"--loader", "fabric",
		"--channel", "release",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if client.versionID != "v2" || client.versionCalls != 1 {
		t.Errorf("versionID = %q, versionCalls = %d", client.versionID, client.versionCalls)
	}
	if client.versionsCalls != 0 {
		t.Errorf("versionsCalls = %d, want 0", client.versionsCalls)
	}
	if !strings.Contains(out, "Instalado Sodium 2.0") {
		t.Errorf("stdout = %q", out)
	}

	target := filepath.Join(instancePath, instance.ModsDirName, "sodium.jar")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
}

func TestInstallCommandRejectsIncompatibleSpecificVersion(t *testing.T) {
	client := &fakeProviderClient{
		project: modsource.Project{ID: "AANobbMI", Name: "Sodium", Type: modsource.KindMod},
		version: modsource.Version{
			ID:           "v2",
			ProjectID:    "AANobbMI",
			GameVersions: []string{"1.20.1"},
			Loaders:      []string{"fabric"},
			Files: []modsource.File{{
				Name:    "sodium.jar",
				URL:     "https://example.com/sodium.jar",
				Primary: true,
				Hashes:  installHashes([]byte("x")),
			}},
		},
	}

	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		download.New(),
		"sodium",
		"--version", "v2",
		"--game-version", "1.21.1",
	)
	if err == nil || !strings.Contains(err.Error(), "não é compatível") {
		t.Fatalf("Execute() error = %v, want incompatibility error", err)
	}
	if _, statErr := os.Stat(filepath.Join(instancePath, instance.ModsDirName, "sodium.jar")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target exists after failure: %v", statErr)
	}
}

func TestInstallCommandMapsProjectKinds(t *testing.T) {
	content := []byte("pacote")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	tests := []struct {
		kind   modsource.ProjectKind
		folder string
		name   string
	}{
		{kind: modsource.KindShaderPack, folder: instance.ShaderPacksDirName, name: "shader.zip"},
		{kind: modsource.KindResourcePack, folder: instance.ResourcePacksDirName, name: "pack.zip"},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			client := &fakeProviderClient{
				project: modsource.Project{ID: "P1", Name: "Pacote", Type: test.kind},
				version: modsource.Version{
					ID:            "v1",
					ProjectID:     "P1",
					VersionNumber: "1.0",
					Files: []modsource.File{{
						Name:    test.name,
						URL:     server.URL,
						Primary: true,
						Hashes:  installHashes(content),
					}},
				},
			}

			instancePath := t.TempDir()
			if _, _, err := executeInstallCommand(
				t,
				client,
				&options{InstancePath: instancePath},
				installTestDownloader(server),
				"pacote",
				"--version", "v1",
			); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			target := filepath.Join(instancePath, test.folder, test.name)
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("Stat(%q) error = %v", target, err)
			}
		})
	}
}

func TestInstallCommandRejectsUnsupportedKinds(t *testing.T) {
	tests := []modsource.ProjectKind{
		modsource.KindWorld,
		modsource.KindModpack,
		modsource.KindPlugin,
		modsource.KindDatapack,
		"",
	}

	for _, kind := range tests {
		t.Run(string(kind), func(t *testing.T) {
			client := &fakeProviderClient{
				project: modsource.Project{ID: "P1", Name: "Projeto", Type: kind},
			}

			instancePath := t.TempDir()
			_, _, err := executeInstallCommand(
				t,
				client,
				&options{InstancePath: instancePath},
				download.New(),
				"projeto",
			)
			if !errors.Is(err, errInstallUnsupportedKind) {
				t.Fatalf("Execute() error = %v, want errInstallUnsupportedKind", err)
			}
			if _, statErr := os.Stat(filepath.Join(instancePath, instance.ModsDirName)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("mods folder created for unsupported kind: %v", statErr)
			}
		})
	}
}

func TestInstallCommandOverwritePolicy(t *testing.T) {
	content := []byte("novo")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	client := &fakeProviderClient{
		project: modsource.Project{ID: "P1", Name: "Sodium", Type: modsource.KindMod},
		version: modsource.Version{
			ID:            "v1",
			ProjectID:     "P1",
			VersionNumber: "1.0",
			Files: []modsource.File{{
				Name:    "sodium.jar",
				URL:     server.URL,
				Primary: true,
				Hashes:  installHashes(content),
			}},
		},
	}

	instancePath := t.TempDir()
	mods := filepath.Join(instancePath, instance.ModsDirName)
	if err := os.Mkdir(mods, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	target := filepath.Join(mods, "sodium.jar")
	if err := os.WriteFile(target, []byte("antigo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		installTestDownloader(server),
		"sodium",
		"--version", "v1",
	)
	if err == nil || !strings.Contains(err.Error(), "já existe") || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("Execute() error = %v, want overwrite guidance", err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want 0 before overwrite", calls)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "antigo" {
		t.Fatalf("content = %q, want old content", got)
	}

	if _, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		installTestDownloader(server),
		"sodium",
		"--version", "v1",
		"--overwrite",
	); err != nil {
		t.Fatalf("Execute() with --overwrite error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("server calls = %d, want 1", calls)
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content = %q, want new content", got)
	}
	assertNoInstallTempFiles(t, mods)
}

func TestInstallCommandRejectsChecksumMismatch(t *testing.T) {
	content := []byte("conteúdo")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	hashes := installHashes(content)
	hashes[0].Value = strings.Repeat("0", 128)
	client := &fakeProviderClient{
		project: modsource.Project{ID: "P1", Name: "Sodium", Type: modsource.KindMod},
		version: modsource.Version{
			ID:        "v1",
			ProjectID: "P1",
			Files: []modsource.File{{
				Name:    "sodium.jar",
				URL:     server.URL,
				Primary: true,
				Hashes:  hashes,
			}},
		},
	}

	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		installTestDownloader(server),
		"sodium",
		"--version", "v1",
	)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Execute() error = %v, want checksum error", err)
	}
	mods := filepath.Join(instancePath, instance.ModsDirName)
	if _, statErr := os.Stat(filepath.Join(mods, "sodium.jar")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target exists after checksum failure: %v", statErr)
	}
	assertNoInstallTempFiles(t, mods)
}

func TestInstallCommandRejectsMissingChecksum(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	client := &fakeProviderClient{
		project: modsource.Project{ID: "P1", Name: "Sodium", Type: modsource.KindMod},
		version: modsource.Version{
			ID:        "v1",
			ProjectID: "P1",
			Files: []modsource.File{{
				Name:    "sodium.jar",
				URL:     server.URL,
				Primary: true,
			}},
		},
	}

	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		installTestDownloader(server),
		"sodium",
		"--version", "v1",
	)
	if err == nil || !strings.Contains(err.Error(), "checksum SHA1/SHA512") {
		t.Fatalf("Execute() error = %v, want missing checksum error", err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want 0", calls)
	}
}

func TestInstallCommandNoCompatibleVersion(t *testing.T) {
	client := &fakeProviderClient{
		project: modsource.Project{ID: "P1", Name: "Sodium", Type: modsource.KindMod},
		versions: []modsource.Version{
			{
				ID:        "v1",
				ProjectID: "P1",
				Files: []modsource.File{{
					Name: "sodium.jar",
					URL:  "",
				}},
			},
		},
	}

	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		download.New(),
		"sodium",
	)
	if err == nil || !strings.Contains(err.Error(), "nenhuma versão instalável") {
		t.Fatalf("Execute() error = %v, want no version error", err)
	}
	if len(client.versionFilter.Channels) != 1 || client.versionFilter.Channels[0] != modsource.ChannelRelease {
		t.Errorf("Channels = %v, want default release", client.versionFilter.Channels)
	}
}

func TestInstallCommandRequiresInstance(t *testing.T) {
	t.Setenv("MCJAVA_MANAGER_INSTANCE", "")

	client := &fakeProviderClient{}
	_, _, err := executeInstallCommand(t, client, &options{}, download.New(), "sodium")
	if err == nil || !strings.Contains(err.Error(), "informe o caminho da instância") {
		t.Fatalf("Execute() error = %v, want instance guidance", err)
	}
	if client.projectCalls != 0 {
		t.Fatalf("projectCalls = %d, want 0", client.projectCalls)
	}
}

func TestInstallCommandUsesInstanceEnvironment(t *testing.T) {
	content := []byte("env")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	instancePath := t.TempDir()
	t.Setenv("MCJAVA_MANAGER_INSTANCE", instancePath)

	client := &fakeProviderClient{
		project: modsource.Project{ID: "P1", Name: "Sodium", Type: modsource.KindMod},
		version: modsource.Version{
			ID:        "v1",
			ProjectID: "P1",
			Files: []modsource.File{{
				Name:    "sodium.jar",
				URL:     server.URL,
				Primary: true,
				Hashes:  installHashes(content),
			}},
		},
	}

	if _, _, err := executeInstallCommand(
		t,
		client,
		&options{},
		installTestDownloader(server),
		"sodium",
		"--version", "v1",
	); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	target := filepath.Join(instancePath, instance.ModsDirName, "sodium.jar")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
}

func TestInstallCommandProviderUnavailable(t *testing.T) {
	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		nil,
		&options{InstancePath: instancePath},
		download.New(),
		"sodium",
		"--provider", "curseforge",
	)
	if err == nil || !strings.Contains(err.Error(), "CurseForge ainda não está implementado") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestInstallCommandRejectsInvalidChannel(t *testing.T) {
	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		&fakeProviderClient{},
		&options{InstancePath: instancePath},
		download.New(),
		"sodium",
		"--channel", "stable",
	)
	if err == nil || !strings.Contains(err.Error(), "canal inválido") {
		t.Fatalf("Execute() error = %v, want invalid channel error", err)
	}
}

func TestInstallCommandReturnsProjectAPIError(t *testing.T) {
	client := &fakeProviderClient{
		projectErr: &modsource.APIError{
			Provider:   modsource.ProviderModrinth,
			Operation:  "obter projeto",
			StatusCode: 404,
			Err:        modsource.ErrNotFound,
		},
	}

	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		download.New(),
		"ausente",
	)
	if !errors.Is(err, modsource.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "falha ao obter o projeto") {
		t.Fatalf("Execute() error = %v, want context", err)
	}
}

func TestInstallCommandReturnsVersionAPIError(t *testing.T) {
	client := &fakeProviderClient{
		project:    modsource.Project{ID: "P1", Name: "Sodium", Type: modsource.KindMod},
		versionErr: modsource.ErrNotFound,
	}

	instancePath := t.TempDir()
	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: instancePath},
		download.New(),
		"sodium",
		"--version", "v1",
	)
	if !errors.Is(err, modsource.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "falha ao obter a versão") {
		t.Fatalf("Execute() error = %v, want context", err)
	}
}

func TestInstallDirectoryName(t *testing.T) {
	tests := []struct {
		kind   modsource.ProjectKind
		folder string
	}{
		{kind: modsource.KindMod, folder: instance.ModsDirName},
		{kind: modsource.KindShaderPack, folder: instance.ShaderPacksDirName},
		{kind: modsource.KindResourcePack, folder: instance.ResourcePacksDirName},
	}

	for _, test := range tests {
		folder, err := installDirectoryName(test.kind)
		if err != nil {
			t.Fatalf("installDirectoryName(%q) error = %v", test.kind, err)
		}
		if folder != test.folder {
			t.Errorf("installDirectoryName(%q) = %q, want %q", test.kind, folder, test.folder)
		}
	}

	if _, err := installDirectoryName(modsource.KindWorld); !errors.Is(err, errInstallUnsupportedKind) {
		t.Fatalf("installDirectoryName(world) error = %v, want errInstallUnsupportedKind", err)
	}
}

func TestNewestDownloadableVersion(t *testing.T) {
	older := modsource.Version{
		ID:          "older",
		PublishedAt: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		Files:       []modsource.File{{Name: "older.jar", URL: "https://example.com/older.jar", Primary: true}},
	}
	newerWithoutFile := modsource.Version{
		ID:          "newer",
		PublishedAt: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	newest := modsource.Version{
		ID:          "newest",
		PublishedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Files:       []modsource.File{{Name: "newest.jar", URL: "https://example.com/newest.jar", Primary: true}},
	}

	got, ok, err := newestDownloadableVersion([]modsource.Version{older, newerWithoutFile, newest})
	if err != nil {
		t.Fatalf("newestDownloadableVersion() error = %v", err)
	}
	if !ok || got.ID != "newest" {
		t.Fatalf("newestDownloadableVersion() = %q, %v, want newest", got.ID, ok)
	}

	_, ok, err = newestDownloadableVersion([]modsource.Version{newerWithoutFile})
	if err != nil {
		t.Fatalf("newestDownloadableVersion() error = %v", err)
	}
	if ok {
		t.Fatal("newestDownloadableVersion() found a version without a download")
	}
}

func TestValidateVersionSelection(t *testing.T) {
	version := modsource.Version{
		ID:           "v1",
		Channel:      modsource.ChannelBeta,
		GameVersions: []string{"1.21.1"},
		Loaders:      []string{"fabric"},
	}

	if err := validateVersionSelection(version, modsource.VersionFilter{GameVersion: "1.21.1", Loader: "Fabric"}, false); err != nil {
		t.Fatalf("validateVersionSelection() error = %v", err)
	}
	if err := validateVersionSelection(version, modsource.VersionFilter{GameVersion: "1.20.1"}, false); err == nil {
		t.Fatal("validateVersionSelection() accepted incompatible game version")
	}
	if err := validateVersionSelection(version, modsource.VersionFilter{Loader: "forge"}, false); err == nil {
		t.Fatal("validateVersionSelection() accepted incompatible loader")
	}
	if err := validateVersionSelection(version, modsource.VersionFilter{Channels: []modsource.ReleaseChannel{modsource.ChannelRelease}}, true); err == nil {
		t.Fatal("validateVersionSelection() accepted wrong channel")
	}
	if err := validateVersionSelection(version, modsource.VersionFilter{Channels: []modsource.ReleaseChannel{modsource.ChannelBeta}}, true); err != nil {
		t.Fatalf("validateVersionSelection() error = %v", err)
	}
}

func TestSelectionFilterDoesNotDefaultChannelForSpecificVersion(t *testing.T) {
	cmd := newInstallCommand(nil, &options{}, nil)
	opts := &installOptions{provider: string(modsource.ProviderModrinth), versionID: "v1"}

	filter, explicit, err := opts.selectionFilter(cmd)
	if err != nil {
		t.Fatalf("selectionFilter() error = %v", err)
	}
	if explicit {
		t.Fatal("selectionFilter() marked channel explicit")
	}
	if len(filter.Channels) != 0 {
		t.Fatalf("Channels = %v, want none for a specific version", filter.Channels)
	}
}

func TestWrapInstallDownloadError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{err: download.ErrTargetExists, want: "--overwrite"},
		{err: download.ErrNoChecksum, want: "checksum SHA1/SHA512"},
		{err: download.ErrChecksumMismatch, want: "checksum"},
		{err: download.ErrTooLarge, want: "limite de download"},
		{err: download.ErrInvalidFilename, want: "nome de arquivo inválido"},
		{err: fmt.Errorf("wrapped: %w", context.Canceled), want: "falha ao baixar"},
	}

	for _, test := range tests {
		err := wrapInstallDownloadError(test.err, "file.jar", "/tmp/mods")
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("wrapInstallDownloadError(%v) = %v, want %q", test.err, err, test.want)
		}
	}
}

func TestInstallCommandValidatesInstanceBeforeProject(t *testing.T) {
	client := &fakeProviderClient{}
	missing := filepath.Join(t.TempDir(), "nao-existe")

	_, _, err := executeInstallCommand(
		t,
		client,
		&options{InstancePath: missing},
		download.New(),
		"sodium",
	)
	if !errors.Is(err, instance.ErrInstanceNotFound) {
		t.Fatalf("Execute() error = %v, want ErrInstanceNotFound", err)
	}
	if client.projectCalls != 0 {
		t.Fatalf("projectCalls = %d, want 0", client.projectCalls)
	}
}
