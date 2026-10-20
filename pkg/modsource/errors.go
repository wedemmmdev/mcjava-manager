package modsource

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrNotFound indicates that a project, version, or file does not exist.
	ErrNotFound = errors.New("recurso não encontrado")

	// ErrMissingCredentials indicates that a provider requires an API key that
	// was not configured.
	ErrMissingCredentials = errors.New("credenciais da API ausentes")

	// ErrUnauthorized indicates invalid or insufficient credentials.
	ErrUnauthorized = errors.New("credenciais inválidas ou sem permissão")

	// ErrRateLimited indicates that the provider throttled the request.
	ErrRateLimited = errors.New("limite de requisições da API atingido")

	// ErrInvalidResponse indicates that the provider returned malformed data.
	ErrInvalidResponse = errors.New("resposta inválida do provedor")

	// ErrDownloadUnavailable indicates that the provider does not expose a
	// direct download URL for a file.
	ErrDownloadUnavailable = errors.New("download não disponível no provedor")

	// ErrNoFiles indicates that a version has no attached artifacts.
	ErrNoFiles = errors.New("versão sem arquivos")

	// ErrUnsupportedKind indicates that a provider does not host a content kind.
	ErrUnsupportedKind = errors.New("tipo de projeto não suportado pelo provedor")
)

// APIError wraps a failed provider operation while preserving an underlying
// sentinel error for errors.Is/errors.As checks.
type APIError struct {
	Provider   Provider
	Operation  string
	StatusCode int
	Message    string
	RetryAfter time.Duration
	Err        error
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}

	var b strings.Builder
	if e.Provider != "" {
		b.WriteString(e.Provider.DisplayName())
		b.WriteString(": ")
	}
	if e.Operation != "" {
		b.WriteString(e.Operation)
		b.WriteString(": ")
	}
	if e.StatusCode != 0 {
		fmt.Fprintf(&b, "HTTP %d: ", e.StatusCode)
	}
	if e.Message != "" {
		b.WriteString(e.Message)
	} else if e.Err != nil {
		b.WriteString(e.Err.Error())
	}
	if e.RetryAfter > 0 {
		fmt.Fprintf(&b, " (tente novamente em %s)", e.RetryAfter)
	}

	message := strings.TrimSpace(b.String())
	if message == "" {
		return "erro na API do provedor"
	}

	return message
}

func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}
