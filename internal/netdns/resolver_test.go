package netdns

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"
)

func TestDialWithFallbackUsesGoogleFirst(t *testing.T) {
	var calls []string
	conn := newTestConn(t)

	got, err := dialWithFallback(
		context.Background(),
		"udp",
		func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, network+" "+address)
			return conn, nil
		},
	)
	if err != nil {
		t.Fatalf("dialWithFallback() error = %v", err)
	}
	if got != conn {
		t.Fatalf("dialWithFallback() returned a different connection")
	}
	if want := []string{"udp " + googlePublicDNS}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackTriesCloudflareWhenGoogleFails(t *testing.T) {
	var calls []string
	conn := newTestConn(t)
	googleErr := errors.New("google unavailable")

	got, err := dialWithFallback(
		context.Background(),
		"udp",
		func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, network+" "+address)
			if address == googlePublicDNS {
				return nil, googleErr
			}

			return conn, nil
		},
	)
	if err != nil {
		t.Fatalf("dialWithFallback() error = %v", err)
	}
	if got != conn {
		t.Fatalf("dialWithFallback() returned a different connection")
	}
	if want := []string{"udp " + googlePublicDNS, "udp " + cloudflarePublicDNS}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackTriesAllCandidates(t *testing.T) {
	var calls []string
	dialErr := errors.New("network unreachable")

	_, err := dialWithFallback(
		context.Background(),
		"udp",
		func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, network+" "+address)
			return nil, dialErr
		},
	)
	if err == nil {
		t.Fatal("dialWithFallback() error = nil, want an error")
	}
	if !errors.Is(err, dialErr) {
		t.Fatalf("dialWithFallback() error = %v, want it to wrap %v", err, dialErr)
	}
	if want := []string{
		"udp " + googlePublicDNS,
		"udp " + cloudflarePublicDNS,
	}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := dialWithFallback(
		ctx,
		"udp",
		func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dial called after context cancellation")
			return nil, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dialWithFallback() error = %v, want %v", err, context.Canceled)
	}
}

func TestNewResolverUsesGoResolver(t *testing.T) {
	resolver := newResolver(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("not used")
	})
	if !resolver.PreferGo {
		t.Fatal("newResolver().PreferGo = false, want true")
	}
	if resolver.Dial == nil {
		t.Fatal("newResolver().Dial = nil, want a dial function")
	}
}

func TestNewDialerUsesFallbackResolver(t *testing.T) {
	dialer := newDialer()

	if dialer.Timeout != dialTimeout {
		t.Fatalf("newDialer().Timeout = %v, want %v", dialer.Timeout, dialTimeout)
	}
	if dialer.KeepAlive != keepAlive {
		t.Fatalf("newDialer().KeepAlive = %v, want %v", dialer.KeepAlive, keepAlive)
	}
	if dialer.Resolver == nil {
		t.Fatal("newDialer().Resolver = nil, want a resolver")
	}
	if !dialer.Resolver.PreferGo {
		t.Fatal("newDialer().Resolver.PreferGo = false, want true")
	}
	if dialer.Resolver.Dial == nil {
		t.Fatal("newDialer().Resolver.Dial = nil, want a dial function")
	}
}

func TestNewTransportConfiguresHTTPTransport(t *testing.T) {
	transport := NewTransport()

	if transport.DialContext == nil {
		t.Fatal("NewTransport().DialContext = nil, want a dial function")
	}
	if !transport.ForceAttemptHTTP2 {
		t.Fatal("NewTransport().ForceAttemptHTTP2 = false, want true")
	}
	if transport.MaxIdleConns != 100 {
		t.Fatalf("NewTransport().MaxIdleConns = %d, want 100", transport.MaxIdleConns)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Fatalf("NewTransport().IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("NewTransport().TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.ExpectContinueTimeout != time.Second {
		t.Fatalf("NewTransport().ExpectContinueTimeout = %v, want 1s", transport.ExpectContinueTimeout)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("NewTransport().TLSClientConfig = nil, want TLS configuration")
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("NewTransport().TLSClientConfig.RootCAs = nil, want CA roots")
	}
}

func TestNewTransportUsesProvidedDialer(t *testing.T) {
	var calls []string
	conn := newTestConn(t)

	transport := newTransport(func(_ context.Context, network, address string) (net.Conn, error) {
		calls = append(calls, network+" "+address)
		return conn, nil
	})

	got, err := transport.DialContext(context.Background(), "udp", "example.com:53")
	if err != nil {
		t.Fatalf("transport.DialContext() error = %v", err)
	}
	if got != conn {
		t.Fatal("transport.DialContext() returned a different connection")
	}
	if want := []string{"udp example.com:53"}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func newTestConn(t *testing.T) net.Conn {
	t.Helper()

	conn, peer := net.Pipe()
	t.Cleanup(func() {
		_ = conn.Close()
		_ = peer.Close()
	})

	return conn
}
