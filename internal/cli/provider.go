package cli

import (
	"fmt"

	"github.com/wedemmmdev/mcjava-manager/internal/version"
	"github.com/wedemmmdev/mcjava-manager/pkg/modrinth"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

func resolveProviderClient(client modsource.Client, providerName string) (modsource.Client, error) {
	provider, err := modsource.ParseProvider(providerName)
	if err != nil {
		return nil, err
	}

	if client == nil {
		return newProviderClient(provider)
	}
	if client.Provider() != provider {
		return nil, fmt.Errorf("o cliente disponível é do provedor %q, mas --provider=%q foi solicitado", client.Provider(), provider)
	}

	return client, nil
}

func newProviderClient(provider modsource.Provider) (modsource.Client, error) {
	switch provider {
	case modsource.ProviderModrinth:
		return modrinth.New(modrinth.WithUserAgent(providerUserAgent())), nil
	case modsource.ProviderCurseForge:
		return nil, fmt.Errorf("o provedor CurseForge ainda não está implementado")
	default:
		return nil, fmt.Errorf("provedor %q não suportado", provider)
	}
}

func providerUserAgent() string {
	return fmt.Sprintf("mcjava-manager/%s (https://github.com/wedemmmdev/mcjava-manager)", version.Short())
}
