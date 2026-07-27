package prober

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// readUp returns the hanzo_service_up value recorded for a service, and whether
// any was recorded at all. The distinction matters: "no data" and "0" are
// different answers, and conflating them is the bug this package exists to
// avoid.
func readUp(t *testing.T, r *sdkmetric.ManualReader, service string) (int64, bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "hanzo_service_up" {
				continue
			}
			g, ok := m.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("hanzo_service_up is %T, want Gauge[int64]", m.Data)
			}
			for _, dp := range g.DataPoints {
				if v, found := dp.Attributes.Value("service"); found && v.AsString() == service {
					return dp.Value, true
				}
			}
		}
	}
	return 0, false
}

func withReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	r := sdkmetric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(r)))
	t.Cleanup(func() { otel.SetMeterProvider(prev) })
	return r
}

func TestProbeRecordsUpForHealthyTarget(t *testing.T) {
	r := withReader(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := New(Config{Targets: []Target{{Name: "healthy", URL: srv.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	p.probe(context.Background(), p.cfg.Targets[0])

	v, ok := readUp(t, r, "healthy")
	if !ok {
		t.Fatal("no hanzo_service_up recorded")
	}
	if v != 1 {
		t.Fatalf("up = %d, want 1", v)
	}
}

// A target that is down must record 0 — not silence. A gauge that simply stops
// being written is indistinguishable from a prober that died, and an alert on
// absent data fires for both.
func TestProbeRecordsDownForUnreachableTarget(t *testing.T) {
	r := withReader(t)
	p, err := New(Config{
		Targets: []Target{{Name: "gone", URL: "http://127.0.0.1:1"}},
		Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	p.probe(context.Background(), p.cfg.Targets[0])

	v, ok := readUp(t, r, "gone")
	if !ok {
		t.Fatal("unreachable target recorded nothing; it must record 0")
	}
	if v != 0 {
		t.Fatalf("up = %d, want 0", v)
	}
}

// A 5xx is reachable but not healthy.
func TestProbeRecordsDownForServerError(t *testing.T) {
	r := withReader(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, err := New(Config{Targets: []Target{{Name: "erroring", URL: srv.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	p.probe(context.Background(), p.cfg.Targets[0])

	if v, _ := readUp(t, r, "erroring"); v != 0 {
		t.Fatalf("up = %d for a 500, want 0", v)
	}
}

// Start probes immediately rather than after one interval, so a service that is
// already down at boot reads as down instead of unknown.
func TestStartProbesImmediately(t *testing.T) {
	r := withReader(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := New(Config{
		Targets:  []Target{{Name: "fast", URL: srv.URL}},
		Interval: time.Hour, // long enough that a tick cannot be what recorded
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	defer p.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if v, ok := readUp(t, r, "fast"); ok && v == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no probe recorded within 2s; Start must not wait a full interval")
}

// A probe outliving its interval would stack requests on a service already in
// trouble, so the timeout is clamped below it.
func TestTimeoutClampedBelowInterval(t *testing.T) {
	p, err := New(Config{
		Targets:  []Target{{Name: "x", URL: "http://example.invalid"}},
		Interval: time.Second,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.cfg.Timeout >= p.cfg.Interval {
		t.Fatalf("timeout %v not clamped below interval %v", p.cfg.Timeout, p.cfg.Interval)
	}
}

func TestNewRejectsIncompleteTargets(t *testing.T) {
	if _, err := New(Config{Targets: []Target{{Name: "no-url"}}}); err == nil {
		t.Fatal("expected an error for a target with no URL")
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected an error for no targets")
	}
}
