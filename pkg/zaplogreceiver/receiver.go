// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package zaplogreceiver is the o11y-side endpoint of the ZAP-native log
// transport defined by luxfi/log.
//
// Wire: luxfi/zap envelope, MsgType=MsgLogBatch, payload is JSON-encoded
// LogBatch (see luxfi/log.LogBatch for the type definition). No protobuf, no
// OTLP, no gRPC — the ZAP envelope is the only framing.
//
// It is the third of three: pkg/zapreceiver takes spans, pkg/zapmetricreceiver
// takes metrics, this takes logs. Before it existed, a service that wanted its
// logs off the box had to reach for an OTLP exporter, which carries protobuf
// and gRPC on either flavour.
//
// Usage:
//
//	rcv, err := zaplogreceiver.New(zaplogreceiver.Config{
//	    Listen: ":4317",
//	    OnBatch: func(ctx context.Context, b *LogBatch) error {
//	        return datastoreWriter.WriteLogs(ctx, b)
//	    },
//	})
//	if err != nil { return err }
//	defer rcv.Stop()
package zaplogreceiver

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

// MsgLogBatch is the canonical ZAP MsgType for log batches.
// Must match luxfi/log.MsgLogBatch. Spans are 1, metrics are 2.
const MsgLogBatch uint16 = 3

// LogBatch is the JSON shape decoded out of the ZAP envelope.
// Field layout mirrors luxfi/log.LogBatch — keep these in lockstep.
type LogBatch struct {
	AppName  string            `json:"appName,omitempty"`
	Version  string            `json:"version,omitempty"`
	Resource map[string]string `json:"resource,omitempty"`
	Records  []LogRecord       `json:"records"`
}

type LogRecord struct {
	TimeUnixNs         int64          `json:"timeUnixNs"`
	ObservedTimeUnixNs int64          `json:"observedTimeUnixNs,omitempty"`
	Severity           int            `json:"severity,omitempty"`
	SeverityText       string         `json:"severityText,omitempty"`
	Body               string         `json:"body,omitempty"`
	Attributes         map[string]any `json:"attributes,omitempty"`
	TraceID            string         `json:"traceId,omitempty"`
	SpanID             string         `json:"spanId,omitempty"`
	EventName          string         `json:"eventName,omitempty"`
}

// Handler ingests one LogBatch. Implementations should write to the telemetry
// store; return error to log + drop (the sender doesn't wait on the response).
type Handler func(ctx context.Context, batch *LogBatch) error

// Config drives the receiver.
type Config struct {
	// Listen is where the ZAP server binds. A host:port binds TCP; a path —
	// anything starting /, ./, ../ or @ — binds a unix socket, because zap
	// reads the address's form to choose the network. Empty defaults to
	// ":4318", the canonical o11y ZAP port for logs.
	//
	// A socket is the right answer where sender and receiver share a pod: it
	// needs no port, cannot be reached from another pod, and lets the
	// filesystem's permissions say who may write telemetry. A TCP ear is
	// reachable by anything in the cluster.
	Listen string

	// NodeID is the server-side ZAP node identifier (sent in handshake).
	// Empty defaults to "o11y-zaplogreceiver".
	NodeID string

	// OnBatch is called for every decoded LogBatch. Must be set.
	OnBatch Handler

	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Receiver owns the ZAP node and the decode loop.
type Receiver struct {
	cfg  Config
	node *zap.Node

	batches atomic.Uint64
	dropped atomic.Uint64
}

// New constructs and starts a Receiver. Returns immediately once the
// listener is up; the receiver runs in background goroutines until Stop.
func New(cfg Config) (*Receiver, error) {
	if cfg.OnBatch == nil {
		return nil, errors.New("zaplogreceiver: Config.OnBatch is required")
	}
	if cfg.Listen == "" {
		cfg.Listen = ":4317"
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "o11y-zaplogreceiver"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Listen is passed to zap WHOLE. zap.Network reads its form to choose the
	// network — a leading /, ./, ../ or @ is a unix socket, anything else is
	// TCP — so a path here binds a socket with no other change. This used to
	// narrow the address through parsePort into NodeConfig.Port, which threw
	// the host away and made a socket path unrepresentable: the receiver could
	// only ever bind a TCP port, while both emit sides already spoke either.
	//
	// A socket is what the in-pod hop wants. ~110 plugin processes and the o11y
	// process share a pod and a run directory, so their telemetry crosses
	// loopback TCP today for no reason, unauthenticated on 0.0.0.0. Over a
	// socket the filesystem's own permissions decide who may write, which is
	// the discipline the rest of the plane already keeps (0700 dir, 0600 sock).
	if zap.Network(cfg.Listen) == "tcp" {
		if _, err := parsePort(cfg.Listen); err != nil {
			return nil, fmt.Errorf("zaplogreceiver: invalid Listen %q: %w", cfg.Listen, err)
		}
	}

	node := zap.NewNode(zap.NodeConfig{
		NodeID:      cfg.NodeID,
		ServiceType: "_o11y._tcp",
		Address:     cfg.Listen,
		Logger:      cfg.Logger,
		NoDiscovery: true,
	})

	r := &Receiver{cfg: cfg, node: node}
	node.Handle(MsgLogBatch, r.handle)

	if err := node.Start(); err != nil {
		return nil, fmt.Errorf("zaplogreceiver: zap.Node start: %w", err)
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

// handle is the ZAP-level callback. It decodes the JSON payload out of the
// envelope and hands the batch to the registered Handler. It never returns a
// non-nil error — that would shut down the connection — so a misbehaving client
// cannot take down the receive loop for everyone. Bad input is logged and
// counted instead.
func (r *Receiver) handle(ctx context.Context, from string, msg *zap.Message) (*zap.Message, error) {
	payload := msg.Root().Bytes(0)
	if len(payload) == 0 {
		r.cfg.Logger.Warn("zaplogreceiver: empty log batch payload", "from", from)
		r.dropped.Add(1)
		return nil, nil
	}

	var batch LogBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		r.cfg.Logger.Warn("zaplogreceiver: unmarshal log batch", "from", from, "err", err)
		r.dropped.Add(1)
		return nil, nil
	}

	if err := r.cfg.OnBatch(ctx, &batch); err != nil {
		r.cfg.Logger.Warn("zaplogreceiver: handler returned error",
			"from", from, "records", len(batch.Records), "err", err)
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
