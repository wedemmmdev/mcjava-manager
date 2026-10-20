package netdns

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
)

func TestDialWithFallbackUsesSystemResolverFirst(t *testing.T) {
	var calls []string
	conn := newTestConn(t)

	got, err := dialWithFallback(
		context.Background(),
		"udp",
		"[::1]:53",
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
	if want := []string{"udp [::1]:53"}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackTriesPublicDNSWhenSystemFails(t *testing.T) {
	var calls []string
	conn := newTestConn(t)
	systemErr := errors.New("connection refused")

	got, err := dialWithFallback(
		context.Background(),
		"udp",
		"[::1]:53",
		func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, network+" "+address)
			if address == "[::1]:53" {
				return nil, systemErr
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
	if want := []string{"udp [::1]:53", "udp " + googlePublicDNS}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackTriesAllCandidates(t *testing.T) {
	var calls []string
	dialErr := errors.New("network unreachable")

	_, err := dialWithFallback(
		context.Background(),
		"udp",
		"[::1]:53",
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
		"udp [::1]:53",
		"udp " + googlePublicDNS,
		"udp " + cloudflarePublicDNS,
	}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackSkipsDuplicateAddress(t *testing.T) {
	var calls []string
	conn := newTestConn(t)

	_, err := dialWithFallback(
		context.Background(),
		"udp",
		googlePublicDNS,
		func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, network+" "+address)
			return conn, nil
		},
	)
	if err != nil {
		t.Fatalf("dialWithFallback() error = %v", err)
	}
	if want := []string{"udp " + googlePublicDNS}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackWithEmptySystemAddress(t *testing.T) {
	var calls []string
	conn := newTestConn(t)

	_, err := dialWithFallback(
		context.Background(),
		"udp",
		"",
		func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, network+" "+address)
			return conn, nil
		},
	)
	if err != nil {
		t.Fatalf("dialWithFallback() error = %v", err)
	}
	if want := []string{"udp " + googlePublicDNS}; !slices.Equal(calls, want) {
		t.Fatalf("dial calls = %v, want %v", calls, want)
	}
}

func TestDialWithFallbackStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := dialWithFallback(
		ctx,
		"udp",
		"[::1]:53",
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

func TestInstallReplacesDefaultResolver(t *testing.T) {
	previous := net.DefaultResolver
	t.Cleanup(func() {
		net.DefaultResolver = previous
	})

	Install()

	if net.DefaultResolver == previous {
		t.Fatal("Install() did not replace net.DefaultResolver")
	}
	if !net.DefaultResolver.PreferGo {
		t.Fatal("net.DefaultResolver.PreferGo = false, want true")
	}
	if net.DefaultResolver.Dial == nil {
		t.Fatal("net.DefaultResolver.Dial = nil, want a dial function")
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
