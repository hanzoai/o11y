// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package zapreceiver is the o11y-side endpoint of the ZAP-native trace
// transport defined by luxfi/trace.
//
// Wire: luxfi/zap envelope, MsgType=MsgSpanBatch, payload is JSON-encoded
// SpanBatch (see luxfi/trace.SpanBatch for the type definition). No
// protobuf, no OTLP, no gRPC — the ZAP envelope is the only framing.
//
// Usage:
//
//	rcv, err := zapreceiver.New(zapreceiver.Config{
//	    Listen: ":4317",
//	    OnBatch: func(ctx context.Context, b *SpanBatch) error {
//	        return clickhouseWriter.WriteBatch(ctx, b)
//	    },
//	})
//	if err != nil { return err }
//	defer rcv.Stop()
package zapreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/luxfi/zap"
)

// MsgSpanBatch is the canonical ZAP MsgType for trace span batches.
// Must match luxfi/trace.MsgSpanBatch.
const MsgSpanBatch uint16 = 1

// SpanBatch is the JSON shape decoded out of the ZAP envelope.
// Field layout mirrors luxfi/trace.SpanBatch — keep these in lockstep.
type SpanBatch struct {
	AppName  string            `json:"appName,omitempty"`
	Version  string            `json:"version,omitempty"`
	Resource map[string]string `json:"resource,omitempty"`
	Spans    []Span            `json:"spans"`
}

type Span struct {
	TraceID      string         `json:"traceId"`
	SpanID       string         `json:"spanId"`
	ParentSpanID string         `json:"parentSpanId,omitempty"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind,omitempty"`
	StartUnixNs  int64          `json:"startUnixNs"`
	EndUnixNs    int64          `json:"endUnixNs"`
	Attributes   map[string]any `json:"attributes,omitempty"`
	Events       []SpanEvent    `json:"events,omitempty"`
	StatusCode   string         `json:"statusCode,omitempty"`
	StatusMsg    string         `json:"statusMessage,omitempty"`
}

type SpanEvent struct {
	Name       string         `json:"name"`
	TimeUnixNs int64          `json:"timeUnixNs"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// Handler ingests one SpanBatch. Implementations should write to the
// telemetry store; return error to log + drop (the sender doesn't wait
// on the response).
type Handler func(ctx context.Context, batch *SpanBatch) error

// Config drives the receiver.
type Config struct {
	// Listen is the TCP address the ZAP server binds to. Empty defaults
	// to ":4317" — the canonical o11y ZAP port.
	Listen string
	// NodeID is the server-side ZAP node identifier (sent in handshake).
	// Empty defaults to "o11y-zapreceiver".
	NodeID string
	// OnBatch is called for every decoded SpanBatch. Must be set.
	OnBatch Handler
	// Logger is used for receive-side warnings. Empty defaults to slog.Default().
	Logger *slog.Logger
}

// Receiver is a running ZAP trace ingestion endpoint.
type Receiver struct {
	cfg     Config
	node    *zap.Node
	batches atomic.Uint64
	dropped atomic.Uint64
}

// New constructs and starts a Receiver. Returns immediately once the
// listener is up; the receiver runs in background goroutines until Stop.
func New(cfg Config) (*Receiver, error) {
	if cfg.OnBatch == nil {
		return nil, errors.New("zapreceiver: Config.OnBatch is required")
	}
	if cfg.Listen == "" {
		cfg.Listen = ":4317"
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "o11y-zapreceiver"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	port, err := parsePort(cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("zapreceiver: invalid Listen %q: %w", cfg.Listen, err)
	}

	node := zap.NewNode(zap.NodeConfig{
		NodeID:      cfg.NodeID,
		ServiceType: "_o11y._tcp",
		Port:        port,
		Logger:      cfg.Logger,
		NoDiscovery: true,
	})

	r := &Receiver{cfg: cfg, node: node}
	node.Handle(MsgSpanBatch, r.handle)

	if err := node.Start(); err != nil {
		return nil, fmt.Errorf("zapreceiver: zap.Node start: %w", err)
	}
	return r, nil
}

// Stop closes the listener and drains in-flight connections.
func (r *Receiver) Stop() {
	r.node.Stop()
}

// Stats returns lifetime counters. Useful for metrics endpoints.
func (r *Receiver) Stats() (batches uint64, dropped uint64) {
	return r.batches.Load(), r.dropped.Load()
}

// handle is the ZAP-level callback. It decodes the JSON payload out of
// the envelope and hands the batch to the registered Handler. We never
// return a non-nil error here — that would shut down the connection.
// Instead, we log and increment the dropped counter, so a misbehaving
// client doesn't take down the receive loop for everyone.
func (r *Receiver) handle(ctx context.Context, from string, msg *zap.Message) (*zap.Message, error) {
	payload := msg.Root().Bytes(0)
	if len(payload) == 0 {
		r.cfg.Logger.Warn("zapreceiver: empty span batch payload", "from", from)
		r.dropped.Add(1)
		return nil, nil
	}

	var batch SpanBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		r.cfg.Logger.Warn("zapreceiver: unmarshal span batch", "from", from, "err", err)
		r.dropped.Add(1)
		return nil, nil
	}

	if err := r.cfg.OnBatch(ctx, &batch); err != nil {
		r.cfg.Logger.Warn("zapreceiver: handler returned error", "from", from, "spans", len(batch.Spans), "err", err)
		r.dropped.Add(1)
		return nil, nil
	}
	r.batches.Add(1)
	return nil, nil
}

// parsePort accepts ":4317", "4317", "host:4317" and returns 4317.
func parsePort(addr string) (int, error) {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[i+1:]
	}
	if host == "" {
		return 0, errors.New("missing port")
	}
	return strconv.Atoi(host)
}
