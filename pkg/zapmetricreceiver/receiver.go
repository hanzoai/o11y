// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package zapmetricreceiver is the o11y-side endpoint of the ZAP-native
// metric transport defined by luxfi/metric.
//
// Wire: luxfi/zap envelope, MsgType=MsgMetricBatch (=2), payload is
// JSON-encoded MetricBatch (see luxfi/metric.MetricBatch for the type
// definition). No protobuf, no OTLP, no gRPC — the ZAP envelope is the
// only framing.
//
// This sits alongside pkg/zapreceiver (which serves MsgSpanBatch=1).
// Both bind the same canonical o11y ZAP port (:4317) but a single
// luxfi/zap node multiplexes by MsgType in the envelope flags upper
// byte, so one zap.Node instance can host BOTH handlers concurrently.
//
// Usage:
//
//	rcv, err := zapmetricreceiver.New(zapmetricreceiver.Config{
//	    Listen: ":4317",
//	    OnBatch: func(ctx context.Context, b *MetricBatch) error {
//	        return datastoreWriter.WriteMetrics(ctx, b)
//	    },
//	})
//	if err != nil { return err }
//	defer rcv.Stop()
package zapmetricreceiver

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

// MsgMetricBatch is the canonical ZAP MsgType for metric batches.
// Must match luxfi/metric.MsgMetricBatch.
const MsgMetricBatch uint16 = 2

// MetricBatch is the JSON shape decoded out of the ZAP envelope.
// Field layout mirrors luxfi/metric.MetricBatch — keep in lockstep.
type MetricBatch struct {
	AppName     string            `json:"appName,omitempty"`
	Version     string            `json:"version,omitempty"`
	Resource    map[string]string `json:"resource,omitempty"`
	TimestampNs int64             `json:"timestampNs"`
	Families    []MetricFamily    `json:"families"`
}

// MetricFamily is the JSON-stable wire shape of a metric family.
type MetricFamily struct {
	Name    string   `json:"name"`
	Help    string   `json:"help,omitempty"`
	Type    string   `json:"type"` // counter | gauge | histogram | summary
	Metrics []Metric `json:"metrics"`
}

// Metric is the JSON-stable wire shape of a single metric sample.
type Metric struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Value       *float64          `json:"value,omitempty"`
	SampleCount *uint64           `json:"sampleCount,omitempty"`
	SampleSum   *float64          `json:"sampleSum,omitempty"`
	Buckets     []Bucket          `json:"buckets,omitempty"`
	Quantiles   []Quantile        `json:"quantiles,omitempty"`
}

type Bucket struct {
	UpperBound      float64 `json:"upperBound"`
	CumulativeCount uint64  `json:"cumulativeCount"`
}

type Quantile struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

// Handler ingests one MetricBatch. Implementations should write to the
// telemetry store; return error to log + drop (the sender doesn't wait
// on the response).
type Handler func(ctx context.Context, batch *MetricBatch) error

// Config drives the receiver.
type Config struct {
	// Listen is the TCP address the ZAP server binds to. Empty defaults
	// to ":4317" — the canonical o11y ZAP port. Shared with
	// pkg/zapreceiver (span transport); a single zap.Node serves both
	// MsgTypes when wired with both handlers.
	Listen string

	// NodeID is the server-side ZAP node identifier (sent in handshake).
	// Empty defaults to "o11y-zapmetricreceiver".
	NodeID string

	// OnBatch is the metric-ingest callback. Required.
	OnBatch Handler

	// Logger — defaults to slog.Default().
	Logger *slog.Logger
}

// Receiver hosts a luxfi/zap node that accepts MsgMetricBatch messages
// and dispatches each one through the configured Handler.
type Receiver struct {
	cfg     Config
	node    *zap.Node
	logger  *slog.Logger
	closed  atomic.Bool
	batches atomic.Uint64
	errors  atomic.Uint64
}

// New constructs and starts the receiver. Returns when the listener is
// bound; failure to bind returns an error and no goroutine is started.
func New(cfg Config) (*Receiver, error) {
	if cfg.OnBatch == nil {
		return nil, errors.New("zapmetricreceiver: OnBatch handler is required")
	}
	if cfg.Listen == "" {
		cfg.Listen = ":4317"
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "o11y-zapmetricreceiver"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	port, err := portOf(cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("zapmetricreceiver: parse Listen %q: %w", cfg.Listen, err)
	}

	r := &Receiver{
		cfg:    cfg,
		logger: cfg.Logger,
	}
	r.node = zap.NewNode(zap.NodeConfig{
		NodeID:      cfg.NodeID,
		ServiceType: "_o11y._tcp",
		Port:        port,
		Logger:      cfg.Logger,
		NoDiscovery: true,
	})
	r.node.Handle(MsgMetricBatch, r.handle)
	if err := r.node.Start(); err != nil {
		return nil, fmt.Errorf("zapmetricreceiver: start node: %w", err)
	}
	cfg.Logger.Info("o11y zap metric receiver listening",
		"listen", cfg.Listen, "nodeID", cfg.NodeID, "msgType", MsgMetricBatch)
	return r, nil
}

// Stop closes the listener and drains in-flight handlers. Safe to call
// multiple times.
func (r *Receiver) Stop() {
	if r.closed.Swap(true) {
		return
	}
	if r.node != nil {
		r.node.Stop()
	}
}

// Stats returns the running counters (batches processed, decode errors).
// Cheap; intended for /healthz and Prometheus scrape.
func (r *Receiver) Stats() (batches, errors uint64) {
	return r.batches.Load(), r.errors.Load()
}

func (r *Receiver) handle(ctx context.Context, peerID string, msg *zap.Message) (*zap.Message, error) {
	payload := append([]byte(nil), msg.Root().Bytes(0)...)
	var batch MetricBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		r.errors.Add(1)
		r.logger.Warn("zapmetricreceiver: decode batch",
			"peerID", peerID, "size", len(payload), "err", err)
		return nil, nil
	}
	if err := r.cfg.OnBatch(ctx, &batch); err != nil {
		r.errors.Add(1)
		r.logger.Warn("zapmetricreceiver: handler returned error",
			"peerID", peerID, "appName", batch.AppName, "err", err)
		return nil, nil
	}
	r.batches.Add(1)
	return nil, nil
}

// portOf parses a host:port string into the numeric port (host part
// ignored — luxfi/zap binds 0.0.0.0 by default).
func portOf(listen string) (int, error) {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return 4317, nil
	}
	// Accept ":4317", "0.0.0.0:4317", "[::]:4317"
	idx := strings.LastIndex(listen, ":")
	if idx < 0 {
		// Bare port like "4317"
		return strconv.Atoi(listen)
	}
	return strconv.Atoi(listen[idx+1:])
}
