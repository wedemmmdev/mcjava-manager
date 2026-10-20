package download

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

var (
	// ErrTargetExists indicates that the destination file already exists and
	// the caller did not allow overwriting.
	ErrTargetExists = errors.New("arquivo de destino já existe")

	// ErrNoChecksum indicates that the file has no SHA1 or SHA512 checksum.
	ErrNoChecksum = errors.New("arquivo sem checksum SHA1/SHA512")

	// ErrInvalidChecksum indicates a malformed or conflicting checksum.
	ErrInvalidChecksum = errors.New("checksum inválido")

	// ErrChecksumMismatch indicates that the downloaded bytes do not match the
	// checksum published by the provider.
	ErrChecksumMismatch = errors.New("checksum do arquivo não confere")

	// ErrTooLarge indicates that the download exceeds the configured limit.
	ErrTooLarge = errors.New("arquivo maior que o limite permitido")

	// ErrInvalidFilename indicates that the provider returned an unsafe name.
	ErrInvalidFilename = errors.New("nome de arquivo inválido")

	// ErrNoURL indicates that the request has no download URL.
	ErrNoURL = errors.New("URL de download não informada")

	// ErrHTTPStatus indicates that the download server returned a non-200
	// response.
	ErrHTTPStatus = errors.New("resposta HTTP inesperada")
)

// HTTPStatusError describes a non-successful download response.
type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "<nil>"
	}

	status := strings.TrimSpace(e.Status)
	if status == "" && e.StatusCode != 0 {
		status = fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	if status == "" {
		status = "resposta HTTP inválida"
	}

	return status
}

func (e *HTTPStatusError) Unwrap() error {
	return ErrHTTPStatus
}

// ChecksumError describes a checksum mismatch without exposing it as a
// security-sensitive value in normal logs.
type ChecksumError struct {
	Algorithm modsource.HashAlgorithm
	Expected  string
	Actual    string
}

func (e *ChecksumError) Error() string {
	if e == nil {
		return "<nil>"
	}

	return fmt.Sprintf("checksum %s não confere", strings.ToUpper(string(e.Algorithm)))
}

func (e *ChecksumError) Unwrap() error {
	return ErrChecksumMismatch
}
