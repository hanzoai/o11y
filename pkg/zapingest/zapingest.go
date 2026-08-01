// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package zapingest is the ONE place the ZAP receive side meets the event
// plane's write side.
//
// Before it, the two halves existed and never met: pkg/zapreceiver,
// pkg/zaplogreceiver and pkg/zapmetricreceiver decoded batches and handed them
// to a Handler nobody supplied, while pkg/datastoretraces, pkg/datastorelogs and
// pkg/datastoremetrics wrote event.span / event.log / event.metric for a caller
// that did not exist. Nothing in the server constructed either. The only live
// ingest path was cloud's embedded otelcol, which writes the OTLP-fork tables
// the event-plane readers do not read — so the plane the query builders read was
// only ever filled by hand.
//
// This service is that missing edge and nothing else: three transports, three
// writers, one factory.Service. It owns no decoding, no row building and no SQL
// — each of those belongs to exactly one package already.
//
// Each receiver runs its own zap.Node and therefore its own listener, so the
// three signals get three addresses. They are separate ports rather than one
// multiplexed node because a signal must be able to fail, saturate or be
// firewalled on its own.
package zapingest

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/pkg/datastorelogs"
	"github.com/hanzoai/o11y/pkg/datastoremetrics"
	"github.com/hanzoai/o11y/pkg/datastoretraces"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/zaplogreceiver"
	"github.com/hanzoai/o11y/pkg/zapmetricreceiver"
	"github.com/hanzoai/o11y/pkg/zapreceiver"
)

// Service is the running ZAP ingest plane.
type Service struct {
	config Config
	logger *slog.Logger
	store  telemetrystore.TelemetryStore

	spans   *zapreceiver.Receiver
	logs    *zaplogreceiver.Receiver
	metrics *zapmetricreceiver.Receiver

	stopped chan struct{}
}

var _ factory.Service = (*Service)(nil)

// New returns the ingest service. Nothing binds until Start.
func New(settings factory.ProviderSettings, config Config, store telemetrystore.TelemetryStore) *Service {
	return &Service{
		config:  config,
		logger:  settings.Logger.With(slog.String("pkg", "github.com/hanzoai/o11y/pkg/zapingest")),
		store:   store,
		stopped: make(chan struct{}),
	}
}

// Start binds the three receivers with the three event-plane writers as their
// handlers and blocks until Stop. A receiver's New returns as soon as its
// listener is up, so a bind failure surfaces here rather than in a goroutine.
func (s *Service) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.logger.InfoContext(ctx, "zap ingest disabled; the event plane has no write side in this process")
		<-s.stopped
		return nil
	}

	conn := s.store.Datastore()
	var err error

	if s.spans, err = zapreceiver.New(zapreceiver.Config{
		Listen:  s.config.Spans,
		OnBatch: datastoretraces.NewWriter(conn).WriteSpans,
		Logger:  s.logger,
	}); err != nil {
		return errors.WrapInternalf(err, errors.CodeInternal, "cannot start zap span receiver on %s", s.config.Spans)
	}

	if s.logs, err = zaplogreceiver.New(zaplogreceiver.Config{
		Listen:  s.config.Logs,
		OnBatch: datastorelogs.NewWriter(conn).WriteLogs,
		Logger:  s.logger,
	}); err != nil {
		return errors.WrapInternalf(err, errors.CodeInternal, "cannot start zap log receiver on %s", s.config.Logs)
	}

	if s.metrics, err = zapmetricreceiver.New(zapmetricreceiver.Config{
		Listen:  s.config.Metrics,
		OnBatch: datastoremetrics.NewWriter(conn).WriteMetrics,
		Logger:  s.logger,
	}); err != nil {
		return errors.WrapInternalf(err, errors.CodeInternal, "cannot start zap metric receiver on %s", s.config.Metrics)
	}

	s.logger.InfoContext(ctx, "zap ingest listening; receivers write the event plane",
		slog.String("spans", s.config.Spans),
		slog.String("logs", s.config.Logs),
		slog.String("metrics", s.config.Metrics),
	)

	<-s.stopped
	return nil
}

// Stop closes every listener that came up and releases Start.
func (s *Service) Stop(_ context.Context) error {
	if s.spans != nil {
		s.spans.Stop()
	}
	if s.logs != nil {
		s.logs.Stop()
	}
	if s.metrics != nil {
		s.metrics.Stop()
	}
	select {
	case <-s.stopped:
	default:
		close(s.stopped)
	}
	return nil
}
