// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package zapmetricreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/zap"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// TestReceiver_DecodesBatch pins the contract: a client that ships a
// MsgMetricBatch ZAP envelope with a JSON MetricBatch payload triggers
// the OnBatch handler exactly once with the decoded batch.
func TestReceiver_DecodesBatch(t *testing.T) {
	var (
		mu    sync.Mutex
		got   *MetricBatch
		done  = make(chan struct{}, 1)
	)

	port := freePort(t)
	rcv, err := New(Config{
		Listen: fmt.Sprintf("127.0.0.1:%d", port),
		NodeID: "test-rcv",
		OnBatch: func(_ context.Context, b *MetricBatch) error {
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
		t.Fatalf("new receiver: %v", err)
	}
	defer rcv.Stop()

	// Build a client and ship a hand-rolled batch.
	cli := zap.NewNode(zap.NodeConfig{
		NodeID:      "test-cli",
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

	v := 7.0
	c := uint64(3)
	s := 1.5
	want := MetricBatch{
		AppName:     "test-app",
		Version:     "v1.2.3",
		Resource:    map[string]string{"env": "test"},
		TimestampNs: 12345,
		Families: []MetricFamily{
			{Name: "reqs", Help: "Total reqs", Type: "counter",
				Metrics: []Metric{{Value: &v, Labels: map[string]string{"path": "/"}}}},
			{Name: "lat", Help: "Latency", Type: "histogram",
				Metrics: []Metric{{
					SampleCount: &c, SampleSum: &s,
					Buckets: []Bucket{{UpperBound: 0.1, CumulativeCount: 2}},
				}}},
		},
	}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const envelopeSize = 16
	b := zap.NewBuilder(envelopeSize + 64 + len(payload))
	root := b.StartObject(envelopeSize)
	root.SetBytes(0, payload)
	root.FinishAsRoot()
	wire := b.FinishWithFlags(MsgMetricBatch << 8)
	msg, err := zap.Parse(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	peers := cli.Peers()
	if len(peers) == 0 {
		t.Fatal("no peers")
	}
	if err := cli.Send(context.Background(), peers[0], msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for batch")
	}

	mu.Lock()
	defer mu.Unlock()

	if got.AppName != want.AppName || got.TimestampNs != want.TimestampNs {
		t.Errorf("meta mismatch: got %+v want %+v", got, want)
	}
	if len(got.Families) != 2 {
		t.Fatalf("families: got %d want 2", len(got.Families))
	}
	if got.Families[0].Type != "counter" || got.Families[0].Metrics[0].Value == nil || *got.Families[0].Metrics[0].Value != 7 {
		t.Errorf("counter family wrong: %+v", got.Families[0])
	}
	if got.Families[1].Type != "histogram" || len(got.Families[1].Metrics[0].Buckets) != 1 {
		t.Errorf("histogram family wrong: %+v", got.Families[1])
	}

	batches, errs := rcv.Stats()
	if batches != 1 || errs != 0 {
		t.Errorf("stats: got batches=%d errors=%d want 1/0", batches, errs)
	}
}

// TestReceiver_DecodeError increments the error counter and skips the
// handler when the payload isn't valid JSON.
func TestReceiver_DecodeError(t *testing.T) {
	var called bool
	port := freePort(t)
	rcv, err := New(Config{
		Listen: fmt.Sprintf("127.0.0.1:%d", port),
		NodeID: "test-rcv-err",
		OnBatch: func(_ context.Context, _ *MetricBatch) error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new receiver: %v", err)
	}
	defer rcv.Stop()

	cli := zap.NewNode(zap.NodeConfig{
		NodeID:      "test-cli-err",
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

	const envelopeSize = 16
	payload := []byte("not-json")
	b := zap.NewBuilder(envelopeSize + 64 + len(payload))
	root := b.StartObject(envelopeSize)
	root.SetBytes(0, payload)
	root.FinishAsRoot()
	msg, _ := zap.Parse(b.FinishWithFlags(MsgMetricBatch << 8))
	peers := cli.Peers()
	if err := cli.Send(context.Background(), peers[0], msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Allow some time for the receiver loop to process.
	time.Sleep(300 * time.Millisecond)

	if called {
		t.Error("OnBatch fired despite decode error")
	}
	batches, errs := rcv.Stats()
	if batches != 0 || errs != 1 {
		t.Errorf("stats: got batches=%d errors=%d want 0/1", batches, errs)
	}
}

// TestReceiver_MsgTypeStable pins the wire ID with luxfi/metric.
func TestReceiver_MsgTypeStable(t *testing.T) {
	if MsgMetricBatch != 2 {
		t.Fatalf("MsgMetricBatch drift: got %d want 2", MsgMetricBatch)
	}
}
