// Package download writes HTTP artifacts atomically and verifies the SHA1 or
// SHA512 checksums published by content providers.
package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

const (
	// DefaultTimeout limits the total duration of a download when the caller
	// does not provide an HTTP client with a timeout.
	DefaultTimeout = 10 * time.Minute

	// DefaultMaxBytes is the maximum artifact size accepted by default.
	DefaultMaxBytes int64 = 1 << 30 // 1 GiB

	// DefaultUserAgent identifies this application to download servers.
	DefaultUserAgent = "mcjava-manager/dev (https://github.com/wedemmmdev/mcjava-manager)"

	tempPattern    = ".mcjava-manager-*.part"
	copyBufferSize = 32 * 1024
)

// ProgressFunc reports download progress. It is called once with zero bytes
// and then after each written chunk. Returning an error aborts the download.
type ProgressFunc func(downloaded, total int64) error

// Client downloads artifacts using a bounded HTTP client.
type Client struct {
	httpClient *http.Client
	userAgent  string
	maxBytes   int64
}

// Option configures a Client.
type Option func(*Client)

// New creates a downloader with safe defaults.
func New(opts ...Option) *Client {
	client := &Client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		userAgent:  DefaultUserAgent,
		maxBytes:   DefaultMaxBytes,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// WithHTTPClient provides a custom HTTP client. A zero timeout is replaced by
// DefaultTimeout so downloads cannot hang indefinitely.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient == nil {
			return
		}

		copy := &http.Client{
			Transport:     httpClient.Transport,
			CheckRedirect: httpClient.CheckRedirect,
			Jar:           httpClient.Jar,
			Timeout:       httpClient.Timeout,
		}
		if copy.Timeout <= 0 {
			copy.Timeout = DefaultTimeout
		}

		client.httpClient = copy
	}
}

// WithUserAgent overrides the User-Agent sent to download servers.
func WithUserAgent(userAgent string) Option {
	return func(client *Client) {
		if trimmed := strings.TrimSpace(userAgent); trimmed != "" {
			client.userAgent = trimmed
		}
	}
}

// WithMaxBytes overrides the maximum accepted artifact size.
func WithMaxBytes(maxBytes int64) Option {
	return func(client *Client) {
		if maxBytes > 0 {
			client.maxBytes = maxBytes
		}
	}
}

// Request describes one artifact download.
type Request struct {
	URL            string
	Target         string
	ExpectedHashes []modsource.Hash
	Overwrite      bool
	Progress       ProgressFunc
}

// Result describes a completed download.
type Result struct {
	Path   string
	Bytes  int64
	Hashes map[modsource.HashAlgorithm]string
}

// Download fetches the artifact, verifies its checksums, and atomically
// replaces Target.
func (c *Client) Download(ctx context.Context, request Request) (result Result, err error) {
	if err := validateRequest(&request); err != nil {
		return Result{}, err
	}

	specs, err := prepareHashes(request.ExpectedHashes)
	if err != nil {
		return Result{}, err
	}
	if err := ensureTarget(request.Target, request.Overwrite); err != nil {
		return Result{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("falha ao criar a requisição de download: %w", stripURLError(err))
	}
	httpRequest.Header.Set("Accept", "application/octet-stream")
	httpRequest.Header.Set("User-Agent", c.userAgent)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Result{}, fmt.Errorf("falha ao baixar o arquivo: %w", stripURLError(err))
	}
	bodyClosed := false
	defer func() {
		if bodyClosed {
			return
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			wrapped := fmt.Errorf("falha ao fechar a resposta do download: %w", closeErr)
			if err == nil {
				err = wrapped
			} else {
				err = errors.Join(err, wrapped)
			}
		}
	}()

	if response.StatusCode != http.StatusOK {
		return Result{}, &HTTPStatusError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
		}
	}

	total := response.ContentLength
	if total > c.maxBytes {
		return Result{}, fmt.Errorf("%w: %d bytes (limite de %d bytes)", ErrTooLarge, total, c.maxBytes)
	}
	if total < 0 {
		total = 0
	}

	tempPath, written, actualHashes, writeErr := writeTemp(
		filepath.Dir(request.Target),
		response.Body,
		total,
		specs,
		c.maxBytes,
		request.Progress,
	)
	if writeErr != nil {
		closeErr := response.Body.Close()
		bodyClosed = true
		if closeErr != nil {
			return Result{}, errors.Join(
				writeErr,
				fmt.Errorf("falha ao fechar a resposta do download: %w", closeErr),
			)
		}
		return Result{}, writeErr
	}

	removeTemp := true
	defer func() {
		if !removeTemp {
			return
		}
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			wrapped := instance.WrapFSError("remover o arquivo temporário", tempPath, removeErr)
			if err == nil {
				err = wrapped
			} else {
				err = errors.Join(err, wrapped)
			}
		}
	}()

	closeErr := response.Body.Close()
	bodyClosed = true
	if closeErr != nil {
		return Result{}, fmt.Errorf("falha ao fechar a resposta do download: %w", closeErr)
	}

	if err := os.Chmod(tempPath, 0o644); err != nil {
		return Result{}, instance.WrapFSError("ajustar as permissões do arquivo", tempPath, err)
	}
	if !request.Overwrite {
		if _, statErr := os.Stat(request.Target); statErr == nil {
			return Result{}, fmt.Errorf("%w: %q", ErrTargetExists, request.Target)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return Result{}, instance.WrapFSError("verificar o arquivo de destino", request.Target, statErr)
		}
	}
	if err := os.Rename(tempPath, request.Target); err != nil {
		return Result{}, instance.WrapFSError("mover o arquivo para o destino", request.Target, err)
	}
	removeTemp = false

	return Result{
		Path:   request.Target,
		Bytes:  written,
		Hashes: actualHashes,
	}, nil
}

func stripURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}

	return err
}

func validateRequest(request *Request) error {
	if strings.TrimSpace(request.URL) == "" {
		return ErrNoURL
	}
	if strings.TrimSpace(request.Target) == "" {
		return fmt.Errorf("o caminho de destino do download não foi informado")
	}

	target := filepath.Clean(request.Target)
	if !filepath.IsAbs(target) {
		return fmt.Errorf("o caminho de destino do download deve ser absoluto: %q", target)
	}

	base := filepath.Base(target)
	if safe, err := SafeFilename(base); err != nil {
		return err
	} else if safe != base {
		return fmt.Errorf("%w: %q", ErrInvalidFilename, base)
	}

	request.URL = strings.TrimSpace(request.URL)
	request.Target = target

	return nil
}

func ensureTarget(target string, overwrite bool) error {
	info, err := os.Lstat(target)
	switch {
	case err == nil:
		if info.IsDir() {
			return fmt.Errorf("o destino do download %q é um diretório", target)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if linkInfo, linkErr := os.Stat(target); linkErr == nil && linkInfo.IsDir() {
				return fmt.Errorf("o destino do download %q aponta para um diretório", target)
			}
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf(
				"o destino do download %q não é um arquivo regular ou link simbólico",
				target,
			)
		}
		if !overwrite {
			return fmt.Errorf("%w: %q", ErrTargetExists, target)
		}
	case errors.Is(err, fs.ErrNotExist):
	default:
		return instance.WrapFSError("verificar o arquivo de destino", target, err)
	}

	directory := filepath.Dir(target)
	info, err = os.Stat(directory)
	if err != nil {
		return instance.WrapFSError("acessar o diretório de destino", directory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("o caminho %q não é um diretório", directory)
	}

	return nil
}

func writeTemp(
	directory string,
	body io.Reader,
	total int64,
	specs []hashSpec,
	maxBytes int64,
	progress ProgressFunc,
) (resultPath string, written int64, hashes map[modsource.HashAlgorithm]string, err error) {
	file, err := os.CreateTemp(directory, tempPattern)
	if err != nil {
		return "", 0, nil, instance.WrapFSError("criar o arquivo temporário em", directory, err)
	}
	tempPath := file.Name()
	closed := false
	cleanup := true

	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				wrapped := instance.WrapFSError("fechar o arquivo temporário", tempPath, closeErr)
				if err == nil {
					err = wrapped
				} else {
					err = errors.Join(err, wrapped)
				}
			}
		}
		if cleanup {
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				wrapped := instance.WrapFSError("remover o arquivo temporário", tempPath, removeErr)
				if err == nil {
					err = wrapped
				} else {
					err = errors.Join(err, wrapped)
				}
			}
		}
	}()

	writers := make([]io.Writer, 0, len(specs)+1)
	writers = append(writers, file)
	for _, spec := range specs {
		writers = append(writers, spec.hasher)
	}

	written, err = copyWithProgress(io.MultiWriter(writers...), body, total, maxBytes, progress)
	if err != nil {
		return "", 0, nil, err
	}

	if syncErr := file.Sync(); syncErr != nil {
		return "", 0, nil, instance.WrapFSError("sincronizar o arquivo temporário", tempPath, syncErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		return "", 0, nil, instance.WrapFSError("fechar o arquivo temporário", tempPath, closeErr)
	}
	closed = true

	hashes, err = verifyHashes(specs)
	if err != nil {
		return "", 0, nil, err
	}

	cleanup = false
	resultPath = tempPath

	return resultPath, written, hashes, nil
}

func copyWithProgress(
	destination io.Writer,
	source io.Reader,
	total int64,
	maxBytes int64,
	progress ProgressFunc,
) (int64, error) {
	if progress != nil {
		if err := progress(0, total); err != nil {
			return 0, fmt.Errorf("falha ao reportar o progresso: %w", err)
		}
	}

	limited := io.LimitReader(source, maxBytes+1)
	buffer := make([]byte, copyBufferSize)
	var written int64

	for {
		n, readErr := limited.Read(buffer)
		if n > 0 {
			if written+int64(n) > maxBytes {
				return written, fmt.Errorf("%w: limite de %d bytes", ErrTooLarge, maxBytes)
			}
			if _, writeErr := destination.Write(buffer[:n]); writeErr != nil {
				return written, fmt.Errorf("falha ao gravar o arquivo baixado: %w", writeErr)
			}
			written += int64(n)
			if progress != nil {
				if err := progress(written, total); err != nil {
					return written, fmt.Errorf("falha ao reportar o progresso: %w", err)
				}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, fmt.Errorf("falha ao ler o download: %w", readErr)
		}
	}

	return written, nil
}
