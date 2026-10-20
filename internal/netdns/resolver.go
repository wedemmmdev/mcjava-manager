// Package netdns provides HTTP transports with a resilient DNS resolver for
// restricted environments such as Android/Shizuku, where the system-provided
// resolver may point to an inaccessible loopback address.
package netdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	googlePublicDNS     = "8.8.8.8:53"
	cloudflarePublicDNS = "1.1.1.1:53"

	dnsDialTimeout = 5 * time.Second
	dialTimeout    = 30 * time.Second
	keepAlive      = 30 * time.Second
)

type dialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// NewTransport returns an HTTP transport based on http.DefaultTransport that
// tries the system-provided DNS server first and falls back to public
// resolvers when it is unreachable.
func NewTransport() *http.Transport {
	return newTransport(newDialer().DialContext)
}

func newDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlive,
		Resolver:  newResolver((&net.Dialer{Timeout: dnsDialTimeout}).DialContext),
	}
}

func newResolver(dial dialFunc) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialWithFallback(ctx, network, address, dial)
		},
	}
}

func newTransport(dial dialFunc) *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	} else {
		transport = transport.Clone()
	}

	transport.DialContext = dial
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConns = 100
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ExpectContinueTimeout = 1 * time.Second

	return transport
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
