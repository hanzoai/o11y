// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package datastoremetrics

import (
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/zapmetricreceiver"
)

func f64(v float64) *float64 { return &v }
func u64(v uint64) *uint64   { return &v }

// refFNV is an independent, byte-stream expression of FNV-1a (the ground-truth
// definition) used to cross-check the fingerprint primitive without importing
// the forked internal package.
func refFNV(seed uint64, data []byte) uint64 {
	const prime = 1099511628211
	h := seed
	for _, b := range data {
		h ^= uint64(b)
		h *= prime
	}
	return h
}

func TestFingerprintMatchesFNVReference(t *testing.T) {
	attrs := map[string]string{"b": "2", "a": "1"}
	// sorted key order: a,b — stream is key 0xFF value 0xFF per pair.
	stream := []byte("a")
	stream = append(stream, 255)
	stream = append(stream, []byte("1")...)
	stream = append(stream, 255)
	stream = append(stream, []byte("b")...)
	stream = append(stream, 255)
	stream = append(stream, []byte("2")...)
	stream = append(stream, 255)

	got := fingerprint(initialOffset, attrs)
	want := refFNV(initialOffset, stream)
	if got != want {
		t.Fatalf("fingerprint=%d want=%d", got, want)
	}

	// hashWithName appends __name__ 0xFF name (no trailing separator).
	name := "http_requests"
	nameStream := append([]byte("__name__"), 255)
	nameStream = append(nameStream, []byte(name)...)
	if hashWithName(got, name) != refFNV(got, nameStream) {
		t.Fatalf("hashWithName mismatch")
	}
}

func TestFingerprintOffsetPassthroughAndOrderIndependent(t *testing.T) {
	if fingerprint(initialOffset, map[string]string{}) != initialOffset {
		t.Fatalf("empty attrs must pass the offset through unchanged")
	}
	a := fingerprint(initialOffset, map[string]string{"x": "1", "y": "2", "z": "3"})
	b := fingerprint(initialOffset, map[string]string{"z": "3", "y": "2", "x": "1"})
	if a != b {
		t.Fatalf("fingerprint must be independent of map order: %d != %d", a, b)
	}
	if fingerprint(initialOffset, map[string]string{"x": "1"}) == fingerprint(initialOffset, map[string]string{"x": "2"}) {
		t.Fatalf("distinct values must hash differently")
	}
}

func indexTS(rows []tsRow) map[uint64]tsRow {
	m := make(map[uint64]tsRow, len(rows))
	for _, r := range rows {
		m[r.fingerprint] = r
	}
	return m
}

func TestBuildRowsCounter(t *testing.T) {
	batch := &zapmetricreceiver.MetricBatch{
		AppName:     "gateway",
		Version:     "1.2.3",
		Resource:    map[string]string{"deployment.environment": "prod"},
		TimestampNs: 1_700_000_000_000 * 1e6,
		Families: []zapmetricreceiver.MetricFamily{{
			Name: "http_requests_total", Help: "total reqs", Type: "counter",
			Metrics: []zapmetricreceiver.Metric{{
				Labels: map[string]string{"method": "GET"}, Value: f64(42),
			}},
		}},
	}
	ts, samples := buildRows(batch)
	if len(samples) != 1 || len(ts) != 1 {
		t.Fatalf("counter: got %d samples %d ts, want 1/1", len(samples), len(ts))
	}
	s := samples[0]
	if s.metricName != "http_requests_total" || s.value != 42 || s.temporality != temporalityCumulative {
		t.Fatalf("counter sample wrong: %+v", s)
	}
	if s.env != "prod" {
		t.Fatalf("env must come from deployment.environment: %q", s.env)
	}
	tr := ts[0]
	if tr.fingerprint != s.fingerprint {
		t.Fatalf("join key broken: ts fp %d != sample fp %d", tr.fingerprint, s.fingerprint)
	}
	if tr.typ != typeSum || !tr.isMonotonic {
		t.Fatalf("counter must be monotonic Sum: %+v", tr)
	}
	if tr.resourceAttrs["service.name"] != "gateway" || tr.resourceAttrs["service.version"] != "1.2.3" {
		t.Fatalf("AppName/Version must lift into service.*: %+v", tr.resourceAttrs)
	}
	for _, sub := range []string{`"__name__":"http_requests_total"`, `"method":"GET"`, `"service.name":"gateway"`, `"__temporality__":"Cumulative"`} {
		if !strings.Contains(tr.labels, sub) {
			t.Fatalf("labels %q missing %q", tr.labels, sub)
		}
	}
}

func TestBuildRowsGauge(t *testing.T) {
	batch := &zapmetricreceiver.MetricBatch{
		TimestampNs: 1_700_000_000_000 * 1e6,
		Families: []zapmetricreceiver.MetricFamily{{
			Name: "queue_depth", Type: "gauge",
			Metrics: []zapmetricreceiver.Metric{{Value: f64(7)}},
		}},
	}
	ts, samples := buildRows(batch)
	if len(samples) != 1 || samples[0].temporality != temporalityUnspecified {
		t.Fatalf("gauge temporality must be Unspecified: %+v", samples)
	}
	if ts[0].typ != typeGauge || ts[0].isMonotonic {
		t.Fatalf("gauge must be non-monotonic Gauge: %+v", ts[0])
	}
}

func TestBuildRowsHistogram(t *testing.T) {
	batch := &zapmetricreceiver.MetricBatch{
		TimestampNs: 1_700_000_000_000 * 1e6,
		Families: []zapmetricreceiver.MetricFamily{{
			Name: "req_duration", Type: "histogram",
			Metrics: []zapmetricreceiver.Metric{{
				Labels:      map[string]string{"route": "/v1/chat"},
				SampleCount: u64(10), SampleSum: f64(3.5),
				Buckets: []zapmetricreceiver.Bucket{
					{UpperBound: 0.1, CumulativeCount: 3},
					{UpperBound: 0.5, CumulativeCount: 8},
				},
			}},
		}},
	}
	ts, samples := buildRows(batch)
	tsByFP := indexTS(ts)

	// Expect .count, .sum, and 3 .bucket samples (0.1, 0.5, +Inf).
	var count, sum int
	les := map[string]float64{} // le -> sample value
	for _, s := range samples {
		tr := tsByFP[s.fingerprint]
		switch s.metricName {
		case "req_duration.count":
			count++
			if s.value != 10 {
				t.Fatalf(".count value=%v want 10", s.value)
			}
			if tr.unit != unitCount || tr.typ != typeSum {
				t.Fatalf(".count series wrong: %+v", tr)
			}
		case "req_duration.sum":
			sum++
			if s.value != 3.5 {
				t.Fatalf(".sum value=%v want 3.5", s.value)
			}
		case "req_duration.bucket":
			if tr.typ != typeHistogram {
				t.Fatalf(".bucket type must be Histogram: %+v", tr)
			}
			les[tr.attrs["le"]] = s.value
		default:
			t.Fatalf("unexpected series %q", s.metricName)
		}
	}
	if count != 1 || sum != 1 {
		t.Fatalf("want one .count and one .sum, got %d/%d", count, sum)
	}
	if les["0.1"] != 3 || les["0.5"] != 8 || les["+Inf"] != 10 {
		t.Fatalf("cumulative bucket values wrong: %+v", les)
	}
	// Every bucket le must produce a distinct fingerprint.
	seen := map[uint64]bool{}
	for _, s := range samples {
		if s.metricName == "req_duration.bucket" {
			if seen[s.fingerprint] {
				t.Fatalf("duplicate bucket fingerprint %d", s.fingerprint)
			}
			seen[s.fingerprint] = true
		}
	}
}

func TestBuildRowsSummary(t *testing.T) {
	batch := &zapmetricreceiver.MetricBatch{
		TimestampNs: 1_700_000_000_000 * 1e6,
		Families: []zapmetricreceiver.MetricFamily{{
			Name: "rpc_latency", Type: "summary",
			Metrics: []zapmetricreceiver.Metric{{
				SampleCount: u64(5), SampleSum: f64(1.2),
				Quantiles: []zapmetricreceiver.Quantile{
					{Quantile: 0.5, Value: 0.2},
					{Quantile: 0.99, Value: 0.9},
				},
			}},
		}},
	}
	ts, samples := buildRows(batch)
	tsByFP := indexTS(ts)
	quantiles := map[string]float64{}
	for _, s := range samples {
		tr := tsByFP[s.fingerprint]
		if s.metricName == "rpc_latency.quantile" {
			if tr.typ != typeSummary {
				t.Fatalf(".quantile type must be Summary: %+v", tr)
			}
			quantiles[tr.attrs["quantile"]] = s.value
		}
	}
	if quantiles["0.5"] != 0.2 || quantiles["0.99"] != 0.9 {
		t.Fatalf("quantile samples wrong: %+v", quantiles)
	}
}

func TestTimeSeriesHourFloor(t *testing.T) {
	// 1_700_000_123_456 ms is mid-hour; series unix_milli must floor to the hour.
	batch := &zapmetricreceiver.MetricBatch{
		TimestampNs: 1_700_000_123_456 * 1e6,
		Families: []zapmetricreceiver.MetricFamily{{
			Name: "c", Type: "counter",
			Metrics: []zapmetricreceiver.Metric{{Value: f64(1)}},
		}},
	}
	ts, samples := buildRows(batch)
	if samples[0].unixMilli != 1_700_000_123_456 {
		t.Fatalf("sample keeps exact ms: %d", samples[0].unixMilli)
	}
	if ts[0].unixMilli != 1_700_000_123_456/3600000*3600000 {
		t.Fatalf("series must floor to hour: %d", ts[0].unixMilli)
	}
}

func TestNilValueSkipped(t *testing.T) {
	batch := &zapmetricreceiver.MetricBatch{
		TimestampNs: 1,
		Families: []zapmetricreceiver.MetricFamily{{
			Name: "c", Type: "counter",
			Metrics: []zapmetricreceiver.Metric{{Value: nil}},
		}},
	}
	ts, samples := buildRows(batch)
	if len(ts) != 0 || len(samples) != 0 {
		t.Fatalf("nil counter value must be skipped, got %d/%d", len(ts), len(samples))
	}
}

func TestDeterministicAcrossCalls(t *testing.T) {
	mk := func() *zapmetricreceiver.MetricBatch {
		return &zapmetricreceiver.MetricBatch{
			AppName: "svc", TimestampNs: 1_700_000_000_000 * 1e6,
			Families: []zapmetricreceiver.MetricFamily{{
				Name: "m", Type: "counter",
				Metrics: []zapmetricreceiver.Metric{{
					Labels: map[string]string{"a": "1", "b": "2", "c": "3"}, Value: f64(1),
				}},
			}},
		}
	}
	_, s1 := buildRows(mk())
	_, s2 := buildRows(mk())
	if s1[0].fingerprint != s2[0].fingerprint {
		t.Fatalf("fingerprints must be stable across calls: %d != %d", s1[0].fingerprint, s2[0].fingerprint)
	}
}
