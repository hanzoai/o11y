package zapingest

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/factory"
)

// The default must bind NOTHING. This is the regression that caused the
// incident: a linked-in library that seizes :4317-:4319 as a side effect of
// being imported races the host for the same sockets.
func TestDefaultConfigBindsNothing(t *testing.T) {
	c, ok := newConfig().(Config)
	if !ok {
		t.Fatalf("newConfig() is not a Config")
	}
	if got := c.signals(); len(got) != 0 {
		t.Fatalf("default config receives %v; a linked library must take no port", got)
	}
	for name, addr := range map[string]string{"spans": c.Spans, "logs": c.Logs, "metrics": c.Metrics} {
		if addr != "" {
			t.Errorf("default %s address is %q; want empty", name, addr)
		}
	}
	if err := c.Validate(); err != nil {
		t.Errorf("default config must validate, got %v", err)
	}
}

// A Service on the default config starts, binds nothing, and stops cleanly —
// the shape every process that merely links o11y gets.
func TestStartWithNoAddressBindsNothing(t *testing.T) {
	s := New(settings(), Config{}, nil) // nil store: never dereferenced when nothing binds
	done := make(chan error, 1)
	go func() { done <- s.Start(context.Background()) }()

	time.Sleep(50 * time.Millisecond)
	if len(s.started) != 0 {
		t.Fatalf("started %d receivers on an empty config", len(s.started))
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}

func TestSignalsOmitsUnconfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want []string
	}{
		{"none", Config{}, nil},
		{"logs only", Config{Logs: ":4318"}, []string{"logs"}},
		{"spans+metrics", Config{Spans: ":4317", Metrics: ":4319"}, []string{"spans", "metrics"}},
		{"all", Config{Spans: ":4317", Logs: ":4318", Metrics: ":4319"}, []string{"spans", "logs", "metrics"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, s := range tc.cfg.signals() {
				got = append(got, s.name)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("signals()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"empty ok", Config{}, ""},
		{"partial ok", Config{Logs: "0.0.0.0:4318"}, ""},
		{"bad addr", Config{Spans: "4317"}, "not host:port"},
		{"duplicate", Config{Spans: "0.0.0.0:4317", Logs: "0.0.0.0:4317"}, "both use"},
		// An unconfigured signal must not collide with another unconfigured
		// one: three empty strings are equal but none of them is an address.
		{"two empty are not a collision", Config{Spans: "0.0.0.0:4317"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// A bind collision must fail fast AND name the holder. Here the holder is this
// test process itself — the same shape as two ingest implementations linked
// into one binary, which is the case the message must call out by name.
func TestBindCollisionNamesTheHolder(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	s := New(settings(), Config{Logs: addr}, nil)
	sig := signal{name: "logs", addr: addr}
	err = s.bindError(fmt.Errorf("zaplogreceiver: zap.Node start: listen tcp %s: bind: %w", addr, syscall.EADDRINUSE), sig)
	if err == nil {
		t.Fatal("bindError returned nil for an in-use address")
	}
	msg := err.Error()
	for _, want := range []string{addr, "logs", "One ingest edge, not two"} {
		if !strings.Contains(msg, want) {
			t.Errorf("bind error missing %q:\n%s", want, msg)
		}
	}
	// procfs is how the holder is identified; where it exists, the holder is us.
	if _, statErr := os.Stat("/proc/net/tcp"); statErr == nil {
		if !strings.Contains(msg, fmt.Sprintf("pid %d", os.Getpid())) {
			t.Errorf("holder should be this process (pid %d):\n%s", os.Getpid(), msg)
		}
		if !strings.Contains(msg, "THIS SAME PROCESS") {
			t.Errorf("an in-process collision must say so:\n%s", msg)
		}
	}
}

// holderOf finds the process listening on an address, and reports nothing for
// an address nobody holds.
func TestHolderOf(t *testing.T) {
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		t.Skip("no procfs")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if got := holderOf(addr); !strings.Contains(got, fmt.Sprintf("pid %d", os.Getpid())) {
		t.Errorf("holderOf(%s)=%q, want this pid", addr, got)
	}
	ln.Close()
	// Give the socket a moment to leave LISTEN.
	time.Sleep(50 * time.Millisecond)
	if got := holderOf(addr); got != "" {
		t.Errorf("holderOf(%s) after close = %q, want empty", addr, got)
	}
}

func settings() factory.ProviderSettings {
	return factory.ProviderSettings{Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
}
