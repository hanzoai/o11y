// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package zapreceiver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/zapreceiver"
	"github.com/luxfi/zap"
)

// TestReceiverIngestsZAPSpanBatch sends a hand-built SpanBatch envelope
// over ZAP and asserts the receiver's Handler is called with the same
// content. End-to-end smoke for the trace transport.
func TestReceiverIngestsZAPSpanBatch(t *testing.T) {
	var (
		mu   sync.Mutex
		got  *zapreceiver.SpanBatch
		done = make(chan struct{}, 1)
	)

	port := freePort(t)
	rcv, err := zapreceiver.New(zapreceiver.Config{
		Listen: fmt.Sprintf(":%d", port),
		OnBatch: func(_ context.Context, b *zapreceiver.SpanBatch) error {
			mu.Lock()
			got = b
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("receiver: %v", err)
	}
	defer rcv.Stop()

	// Build a synthetic SpanBatch + ZAP envelope by hand — no luxfi/trace
	// dependency required for the test.
	batch := zapreceiver.SpanBatch{
		AppName: "test-service",
		Version: "v0.1.0",
		Resource: map[string]string{
			"service.name": "test-service",
		},
		Spans: []zapreceiver.Span{
			{
				TraceID:     "0102030405060708090a0b0c0d0e0f10",
				SpanID:      "1112131415161718",
				Name:        "test-span",
				Kind:        "internal",
				StartUnixNs: 1700000000000000000,
				EndUnixNs:   1700000000000001000,
			},
		},
	}
	payload, err := json.Marshal(&batch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	b := zap.NewBuilder(128 + len(payload))
	root := b.StartObject(16)
	root.SetBytes(0, payload)
	root.FinishAsRoot()
	wire := b.FinishWithFlags(zapreceiver.MsgSpanBatch << 8)

	// Client side: connect a one-shot ZAP node, send the envelope.
	cli := zap.NewNode(zap.NodeConfig{
		NodeID:      "test-client",
		ServiceType: "_o11y._tcp",
		Port:        0,
		NoDiscovery: true,
	})
	if err := cli.Start(); err != nil {
		t.Fatalf("client start: %v", err)
	}
	defer cli.Stop()

	if err := cli.ConnectDirect(fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		t.Fatalf("connect: %v", err)
	}
	peers := cli.Peers()
	if len(peers) == 0 {
		t.Fatal("no peer after connect")
	}
	msg, err := zap.Parse(wire)
	if err != nil {
		t.Fatalf("parse outgoing: %v", err)
	}
	if err := cli.Send(context.Background(), peers[0], msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OnBatch")
	}

	mu.Lock()
	defer mu.Unlock()
	if got == nil {
		t.Fatal("OnBatch never called")
	}
	if got.AppName != batch.AppName || got.Version != batch.Version {
		t.Fatalf("metadata mismatch: got %+v want %+v", got, batch)
	}
	if len(got.Spans) != 1 || got.Spans[0].TraceID != batch.Spans[0].TraceID {
		t.Fatalf("span mismatch: got %+v", got.Spans)
	}

	batches, dropped := rcv.Stats()
	if batches != 1 || dropped != 0 {
		t.Fatalf("stats: batches=%d dropped=%d", batches, dropped)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return port
}
