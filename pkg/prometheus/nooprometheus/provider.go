// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package nooprometheus is the default prometheus.Prometheus provider.
//
// o11y's canonical query path goes through Datastore SQL directly —
// PromQL support is signoz-inherited and not part of the default stack.
// This noop returns a working (empty) Engine + Queryable so o11y boots
// without the prometheus/prometheus/storage/remote chain (which pulls
// google api → s2a-go → google.golang.org/grpc).
//
// Real PromQL support is gated behind -tags signoz via
// pkg/prometheus/datastoreprometheus.
package nooprometheus

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

type provider struct {
	engine  *promql.Engine
	storage storage.Queryable
}

// NewFactory builds a no-op prometheus.Prometheus provider.
func NewFactory(_ telemetrystore.TelemetryStore) factory.ProviderFactory[prometheus.Prometheus, prometheus.Config] {
	return factory.NewProviderFactory(factory.MustNewName("noop"), func(ctx context.Context, ps factory.ProviderSettings, c prometheus.Config) (prometheus.Prometheus, error) {
		eng := promql.NewEngine(promql.EngineOpts{
			Logger:     slog.Default(),
			MaxSamples: 0,
		})
		return &provider{engine: eng, storage: noopQueryable{}}, nil
	})
}

func (p *provider) Engine() *prometheus.Engine { return p.engine }
func (p *provider) Storage() storage.Queryable  { return p.storage }

// noopQueryable satisfies storage.Queryable without holding any series.
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
