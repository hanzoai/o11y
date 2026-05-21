// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package prometheustest is the test-only Prometheus provider used by
// the query-service rules + querier test suites. Mirrors nooprometheus
// (no remote-read chain → no s2a-go → no google.golang.org/grpc) so the
// default build stays grpc-free. The signoz-tagged build supplies a
// real PromQL provider via pkg/prometheus/datastoreprometheus.
package prometheustest

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/prometheus/prometheus/model/labels"
	promql "github.com/prometheus/prometheus/promql"
	storage "github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/annotations"
)

// Provider is the test prometheus.Prometheus implementation.
type Provider struct {
	engine  *promql.Engine
	storage storage.Queryable
}

// New constructs a Provider directly (mirrors the test signature used
// across pkg/query-service/{app/querier,rules}/*_test.go).
func New(_ context.Context, _ factory.ProviderSettings, _ prometheus.Config, _ telemetrystore.TelemetryStore) *Provider {
	return &Provider{
		engine: promql.NewEngine(promql.EngineOpts{
			Logger:     slog.Default(),
			MaxSamples: 0,
		}),
		storage: noopQueryable{},
	}
}

// Engine satisfies prometheus.Prometheus.
func (p *Provider) Engine() *prometheus.Engine { return p.engine }

// Storage satisfies prometheus.Prometheus.
func (p *Provider) Storage() storage.Queryable { return p.storage }

// Close releases resources held by the provider. The noop provider has
// none, so this is a sink that lets tests call defer-style cleanup.
func (p *Provider) Close() error { return nil }

type noopQueryable struct{}

func (noopQueryable) Querier(_, _ int64) (storage.Querier, error) {
	return noopQuerier{}, nil
}

type noopQuerier struct{}

func (noopQuerier) Select(_ context.Context, _ bool, _ *storage.SelectHints, _ ...*labels.Matcher) storage.SeriesSet {
	return storage.EmptySeriesSet()
}

func (noopQuerier) LabelValues(_ context.Context, _ string, _ *storage.LabelHints, _ ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	return nil, nil, nil
}

func (noopQuerier) LabelNames(_ context.Context, _ *storage.LabelHints, _ ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	return nil, nil, nil
}

func (noopQuerier) Close() error { return nil }
