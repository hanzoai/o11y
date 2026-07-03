// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package iamauthz

import (
	"context"
	"testing"
	"time"
)

// TestProviderStartBlocksUntilStop guards the supervisor contract: factory.Registry
// treats ANY Start() return as a service exit and tears the process down. The IAM
// authz provider has no background loop, so Start MUST block until Stop — a bare
// `return nil` crashed the whole server at boot ("caught service error, exiting").
func TestProviderStartBlocksUntilStop(t *testing.T) {
	p := &provider{stopC: make(chan struct{})}

	done := make(chan error, 1)
	go func() { done <- p.Start(context.Background()) }()

	select {
	case <-done:
		t.Fatal("Start returned before Stop — supervisor would tear down the server at boot")
	case <-time.After(50 * time.Millisecond):
		// expected: still blocking
	}

	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error after Stop: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after Stop")
	}
}

// TestProviderStartReturnsOnContextCancel verifies Start also unblocks on context
// cancellation, so shutdown does not hang if Stop is never called.
func TestProviderStartReturnsOnContextCancel(t *testing.T) {
	p := &provider{stopC: make(chan struct{})}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Start(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error on ctx cancel: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return on context cancellation")
	}
}

// TestProviderHealthyImmediately verifies the provider reports healthy right away
// (IAM is external; readiness is not gated on a live IAM round-trip at boot).
func TestProviderHealthyImmediately(t *testing.T) {
	healthy := make(chan struct{})
	close(healthy)
	p := &provider{healthy: healthy, stopC: make(chan struct{})}

	select {
	case <-p.Healthy():
		// expected
	case <-time.After(50 * time.Millisecond):
		t.Fatal("provider not healthy immediately")
	}
}
