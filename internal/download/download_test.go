package download

import (
	"context"
	"crypto/sha1" // #nosec G505 -- test helper mirrors provider checksums.
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func hashesFor(content []byte) []modsource.Hash {
	sha512sum := sha512.Sum512(content)
	sha1sum := sha1.Sum(content) // #nosec G401 -- compatibility test checksum.

	return []modsource.Hash{
		{Algorithm: modsource.HashSHA512, Value: hex.EncodeToString(sha512sum[:])},
		{Algorithm: modsource.HashSHA1, Value: hex.EncodeToString(sha1sum[:])},
	}
}

func testDownloader(server *httptest.Server, opts ...Option) *Client {
	options := []Option{
		WithHTTPClient(server.Client()),
		WithUserAgent("mcjava-manager/test"),
	}
	options = append(options, opts...)

	return New(options...)
}

func assertNoTempFiles(t *testing.T, directory string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".mcjava-manager-") {
			t.Fatalf("temporary file %q was not removed", entry.Name())
		}
	}
}

type progressRecorder struct {
	calls       int
	downloaded  int64
	total       int64
	initialCall bool
	finalCall   bool
	err         error
}

func (p *progressRecorder) update(downloaded, total int64) error {
	p.calls++
	p.downloaded = downloaded
	p.total = total
	if downloaded == 0 {
		p.initialCall = true
	}
	if total > 0 && downloaded == total {
		p.finalCall = true
	}

	return p.err
}

func TestDownloadVerifiesHashesAndWritesAtomically(t *testing.T) {
	content := []byte("conteúdo do mod")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.Header.Get("User-Agent"); got != "mcjava-manager/test" {
			t.Errorf("User-Agent = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/octet-stream" {
			t.Errorf("Accept = %q", got)
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		if _, err := w.Write(content); err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	progress := &progressRecorder{}

	result, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashesFor(content),
		Progress:       progress.update,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if result.Path != target {
		t.Errorf("Path = %q, want %q", result.Path, target)
	}
	if result.Bytes != int64(len(content)) {
		t.Errorf("Bytes = %d, want %d", result.Bytes, len(content))
	}
	if len(result.Hashes) != 2 {
		t.Errorf("Hashes = %#v, want SHA1 and SHA512", result.Hashes)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}

	if !progress.initialCall {
		t.Error("progress did not receive the initial 0-byte update")
	}
	if !progress.finalCall {
		t.Errorf("progress final update = %d/%d, want complete", progress.downloaded, progress.total)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Errorf("mode = %o, want 0644", got)
		}
	}

	assertNoTempFiles(t, directory)
}

func TestDownloadRejectsExistingTargetBeforeNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, "novo")
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	if err := os.WriteFile(target, []byte("antigo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashesFor([]byte("novo")),
	})
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Download() error = %v, want ErrTargetExists", err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want 0", calls)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "antigo" {
		t.Fatalf("content = %q, want old content", got)
	}
}

func TestDownloadRejectsBrokenSymlinkWithoutOverwrite(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, "novo")
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	if err := os.Symlink(filepath.Join(directory, "missing.jar"), target); err != nil {
		t.Skipf("não foi possível criar link simbólico: %v", err)
	}

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashesFor([]byte("novo")),
	})
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Download() error = %v, want ErrTargetExists", err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want 0", calls)
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target symlink was replaced: info=%v err=%v", info, err)
	}
}

func TestDownloadRejectsSymlinkToDirectory(t *testing.T) {
	directory := t.TempDir()
	targetDirectory := filepath.Join(directory, "mods")
	if err := os.Mkdir(targetDirectory, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	target := filepath.Join(directory, "sodium.jar")
	if err := os.Symlink(targetDirectory, target); err != nil {
		t.Skipf("não foi possível criar link simbólico: %v", err)
	}

	_, err := New().Download(context.Background(), Request{
		URL:            "https://example.com/a.jar",
		Target:         target,
		ExpectedHashes: hashesFor([]byte("x")),
		Overwrite:      true,
	})
	if err == nil || !strings.Contains(err.Error(), "aponta para um diretório") {
		t.Fatalf("Download() error = %v, want symlinked directory error", err)
	}
	info, statErr := os.Lstat(target)
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target symlink was replaced: info=%v err=%v", info, statErr)
	}
}

func TestDownloadReplacesBrokenSymlinkWithOverwrite(t *testing.T) {
	content := []byte("novo")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	if err := os.Symlink(filepath.Join(directory, "missing.jar"), target); err != nil {
		t.Skipf("não foi possível criar link simbólico: %v", err)
	}

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashesFor(content),
		Overwrite:      true,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("target mode = %v, want regular file", info.Mode())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content = %q, want %q", got, content)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadOverwritesExistingTarget(t *testing.T) {
	content := []byte("novo")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	if err := os.WriteFile(target, []byte("antigo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashesFor(content),
		Overwrite:      true,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content = %q, want %q", got, content)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadChecksumMismatch(t *testing.T) {
	content := []byte("conteúdo")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	hashes := hashesFor(content)
	hashes[0].Value = strings.Repeat("0", 128)

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashes,
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("Download() error = %v, want ErrChecksumMismatch", err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target exists after checksum failure: %v", statErr)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadRejectsMissingChecksumBeforeNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:    server.URL,
		Target: filepath.Join(t.TempDir(), "sodium.jar"),
	})
	if !errors.Is(err, ErrNoChecksum) {
		t.Fatalf("Download() error = %v, want ErrNoChecksum", err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want 0", calls)
	}
}

func TestDownloadReturnsHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         filepath.Join(t.TempDir(), "sodium.jar"),
		ExpectedHashes: hashesFor([]byte("x")),
	})
	if !errors.Is(err, ErrHTTPStatus) {
		t.Fatalf("Download() error = %v, want ErrHTTPStatus", err)
	}

	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("Download() error = %T, want *HTTPStatusError", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404", statusErr.StatusCode)
	}
}

func TestDownloadRejectsTooLargeContentLength(t *testing.T) {
	content := []byte("0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	_, err := testDownloader(server, WithMaxBytes(5)).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         filepath.Join(directory, "sodium.jar"),
		ExpectedHashes: hashesFor(content),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Download() error = %v, want ErrTooLarge", err)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadRejectsTooLargeChunkedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter is not a Flusher")
			return
		}
		for i := 0; i < 4; i++ {
			if _, err := w.Write([]byte("0123456789")); err != nil {
				t.Errorf("Write() error = %v", err)
				return
			}
			flusher.Flush()
		}
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	_, err := testDownloader(server, WithMaxBytes(15)).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         filepath.Join(directory, "sodium.jar"),
		ExpectedHashes: hashesFor([]byte("0123456789")),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Download() error = %v, want ErrTooLarge", err)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadProgressErrorAborts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("conteúdo"))
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	progress := &progressRecorder{err: errors.New("stderr fechado")}

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         filepath.Join(directory, "sodium.jar"),
		ExpectedHashes: hashesFor([]byte("conteúdo")),
		Progress:       progress.update,
	})
	if err == nil || !strings.Contains(err.Error(), "reportar o progresso") {
		t.Fatalf("Download() error = %v, want progress error", err)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadValidatesRequest(t *testing.T) {
	directory := t.TempDir()
	validContent := []byte("x")

	tests := []struct {
		name    string
		request Request
		want    error
	}{
		{
			name: "URL vazia",
			request: Request{
				Target:         filepath.Join(directory, "a.jar"),
				ExpectedHashes: hashesFor(validContent),
			},
			want: ErrNoURL,
		},
		{
			name: "destino vazio",
			request: Request{
				URL:            "https://example.com/a.jar",
				ExpectedHashes: hashesFor(validContent),
			},
		},
		{
			name: "destino relativo",
			request: Request{
				URL:            "https://example.com/a.jar",
				Target:         "a.jar",
				ExpectedHashes: hashesFor(validContent),
			},
		},
		{
			name: "nome inseguro",
			request: Request{
				URL:            "https://example.com/a.jar",
				Target:         filepath.Join(directory, "evil\x1b.jar"),
				ExpectedHashes: hashesFor(validContent),
			},
			want: ErrInvalidFilename,
		},
		{
			name: "diretório pai ausente",
			request: Request{
				URL:            "https://example.com/a.jar",
				Target:         filepath.Join(directory, "ausente", "a.jar"),
				ExpectedHashes: hashesFor(validContent),
			},
		},
		{
			name: "checksum malformado",
			request: Request{
				URL:    "https://example.com/a.jar",
				Target: filepath.Join(directory, "b.jar"),
				ExpectedHashes: []modsource.Hash{
					{Algorithm: modsource.HashSHA512, Value: "xyz"},
				},
			},
			want: ErrInvalidChecksum,
		},
		{
			name: "checksum MD5 não suportado",
			request: Request{
				URL:    "https://example.com/a.jar",
				Target: filepath.Join(directory, "md5.jar"),
				ExpectedHashes: []modsource.Hash{
					{Algorithm: modsource.HashMD5, Value: strings.Repeat("a", 32)},
				},
			},
			want: ErrNoChecksum,
		},
		{
			name: "checksums conflitantes",
			request: Request{
				URL:    "https://example.com/a.jar",
				Target: filepath.Join(directory, "c.jar"),
				ExpectedHashes: []modsource.Hash{
					{Algorithm: modsource.HashSHA512, Value: strings.Repeat("a", 128)},
					{Algorithm: modsource.HashSHA512, Value: strings.Repeat("b", 128)},
				},
			},
			want: ErrInvalidChecksum,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Download(context.Background(), test.request)
			if err == nil {
				t.Fatal("Download() error = nil, want an error")
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("Download() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestDownloadRejectsDirectoryTarget(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "mods")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	_, err := New().Download(context.Background(), Request{
		URL:            "https://example.com/a.jar",
		Target:         target,
		ExpectedHashes: hashesFor([]byte("x")),
		Overwrite:      true,
	})
	if err == nil || !strings.Contains(err.Error(), "é um diretório") {
		t.Fatalf("Download() error = %v, want directory error", err)
	}
}

func TestDownloadContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	directory := t.TempDir()
	_, err := New().Download(ctx, Request{
		URL:            "https://example.com/a.jar",
		Target:         filepath.Join(directory, "sodium.jar"),
		ExpectedHashes: hashesFor([]byte("x")),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Download() error = %v, want context.Canceled", err)
	}
	assertNoTempFiles(t, directory)
}

func TestNewDefaults(t *testing.T) {
	client := New()

	if client.httpClient.Timeout != DefaultTimeout {
		t.Errorf("timeout = %s, want %s", client.httpClient.Timeout, DefaultTimeout)
	}
	if client.httpClient.Transport == nil {
		t.Error("transport = nil, want a transport with DNS fallback")
	}
	if client.userAgent != DefaultUserAgent {
		t.Errorf("userAgent = %q, want %q", client.userAgent, DefaultUserAgent)
	}
	if client.maxBytes != DefaultMaxBytes {
		t.Errorf("maxBytes = %d, want %d", client.maxBytes, DefaultMaxBytes)
	}

	custom := New(WithHTTPClient(nil), WithUserAgent("  custom/1.0  "), WithMaxBytes(10))
	if custom.httpClient.Timeout != DefaultTimeout {
		t.Errorf("nil client timeout = %s, want %s", custom.httpClient.Timeout, DefaultTimeout)
	}
	if custom.userAgent != "custom/1.0" {
		t.Errorf("userAgent = %q, want custom/1.0", custom.userAgent)
	}
	if custom.maxBytes != 10 {
		t.Errorf("maxBytes = %d, want 10", custom.maxBytes)
	}
}

func TestChecksumError(t *testing.T) {
	err := &ChecksumError{Algorithm: modsource.HashSHA512, Expected: "a", Actual: "b"}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatal("ChecksumError must unwrap to ErrChecksumMismatch")
	}
	if !strings.Contains(err.Error(), "SHA512") {
		t.Fatalf("Error() = %q, want SHA512", err.Error())
	}
}

func TestHTTPStatusError(t *testing.T) {
	err := &HTTPStatusError{StatusCode: 500, Status: "500 Internal Server Error"}
	if !errors.Is(err, ErrHTTPStatus) {
		t.Fatal("HTTPStatusError must unwrap to ErrHTTPStatus")
	}
	if err.Error() != "500 Internal Server Error" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestDownloadUsesInstancePermissionGuidance(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions are not meaningful on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	content := []byte("x")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o755); err != nil {
			t.Errorf("cleanup Chmod() error = %v", err)
		}
	})

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         filepath.Join(locked, "sodium.jar"),
		ExpectedHashes: hashesFor(content),
	})
	if err == nil {
		t.Fatal("Download() error = nil, want permission error")
	}

	var permissionErr *instance.PermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("Download() error = %T, want *instance.PermissionError", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Download() error = %v, want os.ErrPermission", err)
	}
}

func TestDownloadChecksumMismatchKeepsExistingTarget(t *testing.T) {
	content := []byte("novo")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	target := filepath.Join(directory, "sodium.jar")
	if err := os.WriteFile(target, []byte("antigo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	hashes := hashesFor(content)
	hashes[0].Value = strings.Repeat("0", 128)

	_, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         target,
		ExpectedHashes: hashes,
		Overwrite:      true,
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("Download() error = %v, want ErrChecksumMismatch", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "antigo" {
		t.Fatalf("content = %q, want the previous file to remain intact", got)
	}
	assertNoTempFiles(t, directory)
}

func TestDownloadErrorDoesNotLeakURL(t *testing.T) {
	const secret = "token-secreto"

	_, err := New().Download(context.Background(), Request{
		URL:            "ftp://user:" + secret + "@example.com/file.jar",
		Target:         filepath.Join(t.TempDir(), "file.jar"),
		ExpectedHashes: hashesFor([]byte("x")),
	})
	if err == nil {
		t.Fatal("Download() error = nil, want an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("Download() error leaked the URL secret: %v", err)
	}
}

func TestDownloadVerifiesSHA1Only(t *testing.T) {
	content := []byte("somente sha1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	sum := sha1.Sum(content) // #nosec G401 -- compatibility test checksum.
	hashes := []modsource.Hash{{Algorithm: modsource.HashSHA1, Value: hex.EncodeToString(sum[:])}}

	directory := t.TempDir()
	result, err := testDownloader(server).Download(context.Background(), Request{
		URL:            server.URL,
		Target:         filepath.Join(directory, "mod.jar"),
		ExpectedHashes: hashes,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if len(result.Hashes) != 1 || result.Hashes[modsource.HashSHA1] == "" {
		t.Fatalf("Hashes = %#v, want only SHA1", result.Hashes)
	}
}
