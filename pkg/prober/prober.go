// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package prober answers "is this service up?" — the one signal o11y could not
// produce for itself.
//
// o11y receives pushed telemetry: a service emits, o11y stores. That is the
// right model for traces, metrics and logs, and it has a blind spot. A service
// that has crashed pushes nothing, and "nothing" is indistinguishable from
// "idle". Availability is the one question that must be asked, not answered.
//
// This asks it. Each target is probed on an interval; the result becomes an
// up/down metric through the normal OTel meter, so it lands in the same store as
// every other metric and pkg/ruler evaluates alerts on it with no special path.
//
// It replaces the blackbox scrape jobs that lived outside o11y, in a separate
// metrics stack, purely because o11y had no prober. One system, one store, one
// place alert rules live.
package prober

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/hanzoai/o11y/pkg/prober"

// Target is one thing to watch.
type Target struct {
	// Name identifies the target in the emitted series (the `service` label).
	Name string

	// URL is probed with GET. A 2xx or 3xx response counts as up.
	URL string

	// Interval defaults to the Config's when zero.
	Interval time.Duration

	// Timeout defaults to the Config's when zero.
	Timeout time.Duration
}

// Config drives the prober.
type Config struct {
	Targets []Target

	// Interval between probes of a single target. Defaults to 30s.
	Interval time.Duration

	// Timeout for one probe. Defaults to 5s. A probe that outlives its interval
	// would stack, so this is clamped below Interval.
	Timeout time.Duration

	// Client is the HTTP client used for probes. Defaults to one with the
	// timeout applied and redirects followed.
	Client *http.Client
}

// Prober runs the probe loops.
type Prober struct {
	cfg Config

	up       metric.Int64Gauge
	duration metric.Float64Histogram

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New builds a Prober. It does not start probing; call Start.
//
// The instruments resolve here rather than at package init: the meter provider
// is installed by the composition root, and binding earlier would attach every
// measurement to the global no-op, which discards silently.
func New(cfg Config) (*Prober, error) {
	if len(cfg.Targets) == 0 {
		return nil, errors.New("prober: no targets")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Timeout >= cfg.Interval {
		// A probe outliving its interval would stack request on request against
		// a service that is already struggling.
		cfg.Timeout = cfg.Interval / 2
	}
	// Defaults land on the targets themselves, here, so a Target is always
	// complete once New returns. Applying them in Start instead left a zero
	// Timeout on any target reached by another path, and a zero timeout means an
	// already-expired context — every probe failing, reported as "down".
	for i := range cfg.Targets {
		if cfg.Targets[i].Name == "" || cfg.Targets[i].URL == "" {
			return nil, fmt.Errorf("prober: target %d needs both Name and URL", i)
		}
		if cfg.Targets[i].Interval <= 0 {
			cfg.Targets[i].Interval = cfg.Interval
		}
		if cfg.Targets[i].Timeout <= 0 {
			cfg.Targets[i].Timeout = cfg.Timeout
		}
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: cfg.Timeout}
	}

	m := otel.Meter(meterName)
	up, err := m.Int64Gauge("hanzo_service_up",
		metric.WithDescription("1 when the service answered its health probe, 0 when it did not."))
	if err != nil {
		return nil, fmt.Errorf("prober: up gauge: %w", err)
	}
	dur, err := m.Float64Histogram("hanzo_service_probe_duration_seconds",
		metric.WithDescription("Health probe round-trip in seconds."),
		metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("prober: duration histogram: %w", err)
	}
	return &Prober{cfg: cfg, up: up, duration: dur}, nil
}

// Start begins probing. Each target gets its own goroutine so a slow target
// cannot delay the others — the failure being watched for is exactly the one
// that would serialize them.
func (p *Prober) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	for _, t := range p.cfg.Targets {
		t := t
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			// Probe immediately: waiting a full interval means a service that is
			// down at boot reads as unknown rather than down for that window.
			p.probe(ctx, t)
			tk := time.NewTicker(t.Interval)
			defer tk.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tk.C:
					p.probe(ctx, t)
				}
			}
		}()
	}
}

// Stop ends probing and waits for the loops to finish.
func (p *Prober) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
		p.wg.Wait()
	}
}

// probe performs one check and records the result.
//
// Every outcome emits: an unreachable target records 0, not nothing. A gauge
// that stops being written looks identical to a prober that died, and the whole
// point is to tell those apart.
func (p *Prober) probe(ctx context.Context, t Target) {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	attrs := metric.WithAttributes(attribute.String("service", t.Name))
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		p.up.Record(ctx, 0, attrs)
		return
	}
	resp, err := p.cfg.Client.Do(req)
	elapsed := time.Since(start).Seconds()
	p.duration.Record(ctx, elapsed, attrs)

	if err != nil {
		p.up.Record(ctx, 0, attrs)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		p.up.Record(ctx, 1, attrs)
		return
	}
	p.up.Record(ctx, 0, attrs)
}
