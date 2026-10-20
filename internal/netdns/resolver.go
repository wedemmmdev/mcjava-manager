// Package netdns provides HTTP transports with a public DNS resolver for
// restricted environments such as Android/Shizuku, where the system-provided
// resolver may point to an inaccessible loopback address.
package netdns

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	googlePublicDNS     = "8.8.8.8:53"
	cloudflarePublicDNS = "1.1.1.1:53"

	dnsDialTimeout = 5 * time.Second
	dialTimeout    = 30 * time.Second
	keepAlive      = 30 * time.Second
)

var (
	androidCertDirs = []string{
		"/system/etc/security/cacerts",
		"/apex/com.android.conscrypt/cacerts",
	}

	rootCAsOnce sync.Once
	rootCAsPool *x509.CertPool
)

type dialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// NewTransport returns an HTTP transport based on http.DefaultTransport that
// resolves names through public DNS servers, bypassing the system resolver.
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
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialWithFallback(ctx, network, dial)
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
	transport.TLSClientConfig = &tls.Config{RootCAs: rootCAs()}

	return transport
}

func rootCAs() *x509.CertPool {
	rootCAsOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}

		// Go's Linux certificate loader does not know about Android's hashed
		// CA directory. Load it explicitly when running under Android.
		for _, directory := range androidCertDirs {
			appendCertsFromDirectory(pool, directory)
		}

		rootCAsPool = pool
	})

	return rootCAsPool
}

func appendCertsFromDirectory(pool *x509.CertPool, directory string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			continue
		}

		pool.AppendCertsFromPEM(data)
	}
}

func dialWithFallback(ctx context.Context, network string, dial dialFunc) (net.Conn, error) {
	var lastErr error
	for _, address := range []string{googlePublicDNS, cloudflarePublicDNS} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		conn, err := dial(ctx, network, address)
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
		lastErr = errors.New("nenhum servidor DNS público disponível")
	}

	return nil, fmt.Errorf("falha ao contatar servidores DNS públicos: %w", lastErr)
}
