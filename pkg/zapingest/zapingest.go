// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package zapingest is the ONE place the ZAP receive side meets the event
// plane's write side.
//
// Before it, the two halves existed and never met: pkg/zapreceiver,
// pkg/zaplogreceiver and pkg/zapmetricreceiver decoded batches and handed them
// to a Handler nobody supplied, while pkg/datastoretraces, pkg/datastorelogs and
// pkg/datastoremetrics wrote event.span / event.log / event.metric for a caller
// that did not exist. Nothing in the server constructed either.
//
// This service is that missing edge and nothing else: three transports, three
// writers, one factory.Service. It owns no decoding, no row building and no SQL
// — each of those belongs to exactly one package already.
//
// It is also the ONLY ingest edge, and an embedding host must not carry a
// second one. What a consumer gets here is the WHOLE edge: three signals, and
// the support tables the read plane joins against — event.span_resource /
// _attribute / _key, event.trace, event.operation, event.log_resource /
// _attribute / _key / _resource_key, event.series. A partial re-implementation
// of the write half fills the two headline tables and leaves those empty, and
// an empty destination is indistinguishable from "no traffic". So a second
// implementation does not announce itself: it quietly wins a port and drops
// what it has nowhere to put.
//
// Each receiver runs its own zap.Node and therefore its own listener, so the
// three signals get three addresses. They are separate ports rather than one
// multiplexed node because a signal must be able to fail, saturate or be
// firewalled on its own.
//
// NOTHING BINDS UNLESS AN ADDRESS SAYS SO. See Config: the address is the
// switch, it defaults to empty, and linking this package takes no port.
package zapingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"syscall"

	"github.com/hanzoai/o11y/pkg/datastorelogs"
	"github.com/hanzoai/o11y/pkg/datastoremetrics"
	"github.com/hanzoai/o11y/pkg/datastoretraces"
	o11yerrors "github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/zaplogreceiver"
	"github.com/hanzoai/o11y/pkg/zapmetricreceiver"
	"github.com/hanzoai/o11y/pkg/zapreceiver"
)

// stopper is the only thing Service needs from a receiver once it is up.
// Holding the three behind it keeps Stop uniform and the signal list open.
type stopper interface{ Stop() }

// Service is the running ZAP ingest plane.
type Service struct {
	config Config
	logger *slog.Logger
	store  telemetrystore.TelemetryStore

	started []stopper
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

// Start binds each CONFIGURED receiver with its event-plane writer as the
// handler, then blocks until Stop. A receiver's New returns as soon as its
// listener is up, so a bind failure surfaces here rather than in a goroutine —
// and it is RETURNED, never swallowed. Losing a port means losing telemetry,
// which is exactly the failure that must not be quiet.
func (s *Service) Start(ctx context.Context) error {
	configured := s.config.signals()
	if len(configured) == 0 {
		s.logger.InfoContext(ctx, "zap ingest receives nothing in this process: no signal address is configured (set zapingest spans/logs/metrics to bind)")
		<-s.stopped
		return nil
	}

	conn := s.store.Datastore()
	// One binder per signal, so the three read identically and a new signal is
	// one entry rather than another copy of the same error handling.
	binders := map[string]func(addr string) (stopper, error){
		"spans": func(addr string) (stopper, error) {
			return zapreceiver.New(zapreceiver.Config{
				Listen:  addr,
				OnBatch: datastoretraces.NewWriter(conn).WriteSpans,
				Logger:  s.logger,
			})
		},
		"logs": func(addr string) (stopper, error) {
			return zaplogreceiver.New(zaplogreceiver.Config{
				Listen:  addr,
				OnBatch: datastorelogs.NewWriter(conn).WriteLogs,
				Logger:  s.logger,
			})
		},
		"metrics": func(addr string) (stopper, error) {
			return zapmetricreceiver.New(zapmetricreceiver.Config{
				Listen:  addr,
				OnBatch: datastoremetrics.NewWriter(conn).WriteMetrics,
				Logger:  s.logger,
			})
		},
	}

	for _, sig := range configured {
		rcv, err := binders[sig.name](sig.addr)
		if err != nil {
			// Release whatever already came up. A half-bound edge is worse than
			// none: it reports partial traffic with a healthy-looking surface.
			s.stopAll()
			bindErr := s.bindError(err, sig)
			// Logged as well as returned: an embedding host is free to ignore a
			// service error, and telemetry vanishing must not depend on whether
			// the caller chose to print it.
			s.logger.ErrorContext(ctx, "zap ingest cannot take its port",
				slog.String("signal", sig.name), slog.String("addr", sig.addr), slog.Any("error", bindErr))
			return bindErr
		}
		s.started = append(s.started, rcv)
		s.logger.InfoContext(ctx, "zap ingest receiving", slog.String("signal", sig.name), slog.String("addr", sig.addr))
	}

	<-s.stopped
	return nil
}

// bindError turns a failed listen into an error that names what it lost to.
// "address already in use" on its own sends you hunting for a stale process;
// the holder's identity is what reveals that the winner is a second ingest
// implementation inside this very binary.
//
// The message is built into the CAUSE, not into the wrapper. o11y's
// errors.base.Error() returns the wrapped cause's text and drops the wrapper's
// own message, so a message passed to Wrap*f is retained on the value but
// never printed by anything that renders err.Error() — a diagnostic that
// exists and is invisible. Folding it into the cause makes it print, and the
// %w keeps errors.Is(err, syscall.EADDRINUSE) working for callers.
func (s *Service) bindError(err error, sig signal) error {
	if !errors.Is(err, syscall.EADDRINUSE) {
		cause := fmt.Errorf("zapingest cannot start the %s receiver on %s: %w", sig.name, sig.addr, err)
		return o11yerrors.WrapInternalf(cause, o11yerrors.CodeInternal, "%s", cause)
	}
	held := holderOf(sig.addr)
	if held == "" {
		held = "an unidentified process"
	}
	cause := fmt.Errorf(
		"zapingest cannot bind %s on %s: already held by %s. One ingest edge, not two — remove the other listener rather than moving this one: %w",
		sig.name, sig.addr, held, err)
	return o11yerrors.WrapInternalf(cause, o11yerrors.CodeInternal, "%s", cause)
}

// Stop closes every listener that came up and releases Start.
func (s *Service) Stop(_ context.Context) error {
	s.stopAll()
	select {
	case <-s.stopped:
	default:
		close(s.stopped)
	}
	return nil
}

func (s *Service) stopAll() {
	for _, r := range s.started {
		r.Stop()
	}
	s.started = nil
}
