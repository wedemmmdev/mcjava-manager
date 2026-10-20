// Package netdns configures a resilient DNS resolver for restricted
// environments such as Android/Shizuku, where the system-provided resolver may
// point to an inaccessible loopback address.
package netdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	googlePublicDNS     = "8.8.8.8:53"
	cloudflarePublicDNS = "1.1.1.1:53"
	dialTimeout         = 5 * time.Second
)

type dialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Install replaces net.DefaultResolver with a resolver that tries the
// system-provided DNS server first and falls back to public resolvers when it
// is unreachable.
func Install() {
	dialer := &net.Dialer{Timeout: dialTimeout}
	net.DefaultResolver = newResolver(dialer.DialContext)
}

func newResolver(dial dialFunc) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialWithFallback(ctx, network, address, dial)
		},
	}
}

func dialWithFallback(ctx context.Context, network, address string, dial dialFunc) (net.Conn, error) {
	candidates := dnsCandidates(address)

	var lastErr error
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		conn, err := dial(ctx, network, candidate)
		if err == nil {
			if conn == nil {
				lastErr = errors.New("conexão DNS vazia")
				continue
			}

			return conn, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("nenhum servidor DNS disponível")
	}

	return nil, fmt.Errorf("falha ao contatar servidores DNS: %w", lastErr)
}

func dnsCandidates(systemAddress string) []string {
	candidates := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)

	for _, address := range []string{systemAddress, googlePublicDNS, cloudflarePublicDNS} {
		if address == "" {
			continue
		}
		if _, ok := seen[address]; ok {
			continue
		}

		seen[address] = struct{}{}
		candidates = append(candidates, address)
	}

	return candidates
}
