package config

import (
	"fmt"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

// MissingAPIKeyError explains how to configure a required API key. It unwraps
// to modsource.ErrMissingCredentials so callers can react uniformly.
type MissingAPIKeyError struct {
	ConfigPath string
}

func (e *MissingAPIKeyError) Error() string {
	return fmt.Sprintf(`API Key do CurseForge não configurada.

Defina a variável de ambiente %s ou adicione a chave ao arquivo %s:

curseforge:
  api_key: "SUA_CHAVE"

Obtenha sua chave em %s.`, EnvCurseForgeAPIKey, e.ConfigPath, modsource.ProviderCurseForge.APIKeyURL())
}

func (e *MissingAPIKeyError) Unwrap() error {
	return modsource.ErrMissingCredentials
}
