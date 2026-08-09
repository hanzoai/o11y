// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package datastoremetrics is the o11y-native datastore metrics driver: it
// writes metric samples + series metadata to the datastore (the columnar
// telemetry backend) over UPSTREAM ch-go / datastore-go v2, with NO
// histogram-fork dependency.
//
// It is the write-side companion to pkg/zapmetricreceiver: WriteMetrics
// satisfies zapmetricreceiver.Handler, so a receiver dispatches each decoded
// batch straight into the datastore:
//
//	w := datastoremetrics.NewWriter(store.Datastore())
//	rcv, _ := zapmetricreceiver.New(zapmetricreceiver.Config{OnBatch: w.WriteMetrics})
//
// WHY THIS EXISTS — the unblock. The stock histogram exporter serialises OTLP
// exponential histograms as a DDSketch into the `exp_hist.sketch` column, which
// needs a FORKED ch-go exposing proto.DD / proto.Store / proto.IndexMapping.
// That fork conflicts with the upstream ch-go the query plane pins, which is why
// metrics ingest could not move in-process. This driver sidesteps the fork
// entirely: the ZAP wire (luxfi/metric.MetricBatch) already carries CLASSIC
// Prometheus shapes — explicit histogram buckets, summary quantiles — so every
// series decomposes into plain event.metric rows (`<name>.bucket{le=…}`,
// `<name>.count`, `<name>.sum`, `<name>.quantile{quantile=…}`). No sketch, no
// exp_hist, no fork — just the two event-plane tables the query plane already
// reads (event.series + event.metric), keyed by the identical labels fingerprint.
package datastoremetrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hanzo-ds/go"
	"github.com/hanzo-ds/go/lib/driver"
	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/hanzoai/o11y/pkg/zapmetricreceiver"
)

// Temporality + type column values — the exact strings the query plane filters
// on (metrictypes.Temporality.Value() / OTLP MetricType.String()).
const (
	temporalityCumulative  = "Cumulative"
	temporalityUnspecified = "Unspecified"

	typeSum       = "Sum"
	typeGauge     = "Gauge"
	typeHistogram = "Histogram"
	typeSummary   = "Summary"

	// Series-name suffixes for decomposed complex metrics — the datastore's
	// dot-suffix convention (NOT Prometheus `_bucket`/`_count`).
	suffixCount    = ".count"
	suffixSum      = ".sum"
	suffixBucket   = ".bucket"
	suffixQuantile = ".quantile"

	// unitCount is the unit stamped on synthesised `.count` series.
	unitCount = "1"
)

// INSERT templates — identical column order to the datastore metrics schema.
// exp_hist is intentionally absent: the native path never writes a sketch.
const (
	samplesSQLTmpl    = "INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, value, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	timeSeriesSQLTmpl = "INSERT INTO %s.%s (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels, attrs, scope_attrs, resource_attrs, __normalized, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
)

// sampleRow maps 1:1 to an event.metric row (minus the write-time inserted_at).
type sampleRow struct {
	env         string
	temporality string
	metricName  string
	fingerprint uint64
	unixMilli   int64
	value       float64
	flags       uint32
}

// tsRow maps 1:1 to an event.series row (minus __normalized / inserted_at).
type tsRow struct {
	env           string
	temporality   string
	metricName    string
	description   string
	unit          string
	typ           string
	isMonotonic   bool
	fingerprint   uint64
	unixMilli     int64 // hour-floored, matching the schema's series cadence
	labels        string
	attrs         map[string]string
	scopeAttrs    map[string]string
	resourceAttrs map[string]string
}

// Writer ingests decoded metric batches into the datastore over upstream
// datastore-go v2. It is safe for concurrent use (datastore.Conn is).
type Writer struct {
	conn         datastore.Conn
	db           string
	tsTable      string
	samplesTable string
	nowMilli     func() int64
}

// Option configures a Writer.
type Option func(*Writer)

// WithDatabase overrides the target datastore database (default: the query
// plane's canonical metrics DB).
func WithDatabase(db string) Option { return func(w *Writer) { w.db = db } }

// WithTables overrides the distributed time-series / samples table names.
func WithTables(timeSeries, samples string) Option {
	return func(w *Writer) { w.tsTable, w.samplesTable = timeSeries, samples }
}

// WithNow injects the inserted-at clock (tests).
func WithNow(now func() int64) Option { return func(w *Writer) { w.nowMilli = now } }

// NewWriter builds a Writer over an existing datastore connection. Defaults
// target the SAME database + distributed tables the query plane reads, so a
// series written here is immediately queryable.
func NewWriter(conn datastore.Conn, opts ...Option) *Writer {
	w := &Writer{
		conn:         conn,
		db:           telemetrymetrics.DBName,
		tsTable:      telemetrymetrics.SeriesTableName,
		samplesTable: telemetrymetrics.MetricTableName,
		nowMilli:     func() int64 { return time.Now().UnixMilli() },
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// WriteMetrics is the zapmetricreceiver.Handler: it decomposes a batch into
// series + samples and writes both tables. A nil / empty batch is a no-op.
// WriteMetricsMany writes N batches in TWO inserts instead of 2N, and that
// ratio is the whole point.
//
// Every insert into the samples table fans out through the 5m and 30m rollup
// views, so one INSERT becomes three parts — and the ZAP wire carries one
// batch per RESOURCE, so a fleet of collectors produces inserts in proportion
// to the number of OBJECTS it observes rather than the amount of data. Measured
// 2026-08-08 on hanzo-k8s: ~518 writes a minute against a merge pool already
// pinned 32/32, event.metric_30m climbing past 3,000 parts in one partition,
// and ClickHouse then answering code 252 to EVERY metric write — app telemetry
// that had been landing for months included. A k8s_cluster receiver, which
// models each object as its own resource, would have been ~16,000 a minute.
//
// Rows are cheap to concatenate and inserts are not, so the caller can buffer
// whatever arrives in a window and land it as one pair of statements. The
// per-batch semantics are untouched: buildRows still runs per batch, so each
// batch keeps its own resource attributes and fingerprints exactly as it would
// alone. Only the STATEMENT COUNT changes.
//
// A nil or empty batch is skipped rather than refused — one bad sender must not
// discard a window's worth of everyone else's metrics.
func (w *Writer) WriteMetricsMany(ctx context.Context, batches []*zapmetricreceiver.MetricBatch) error {
	ts, samples := w.buildRowsMany(batches)
	if len(ts) == 0 && len(samples) == 0 {
		return nil
	}
	if err := w.writeTimeSeries(ctx, ts); err != nil {
		return fmt.Errorf("datastoremetrics: write time_series: %w", err)
	}
	if err := w.writeSamples(ctx, samples); err != nil {
		return fmt.Errorf("datastoremetrics: write samples: %w", err)
	}
	return nil
}

// buildRowsMany is the row half of WriteMetricsMany, split out so the
// concatenation is testable without standing up a datastore connection. Each
// batch is still built on its own — same resource attributes, same
// fingerprints — so this is a change of statement count, not of meaning.
func (w *Writer) buildRowsMany(batches []*zapmetricreceiver.MetricBatch) ([]tsRow, []sampleRow) {
	var ts []tsRow
	var samples []sampleRow
	for _, batch := range batches {
		if batch == nil || len(batch.Families) == 0 {
			continue
		}
		if batch.TimestampNs == 0 {
			batch.TimestampNs = w.nowMilli() * 1e6
		}
		t, s := buildRows(batch)
		ts = append(ts, t...)
		samples = append(samples, s...)
	}
	return ts, samples
}

func (w *Writer) WriteMetrics(ctx context.Context, batch *zapmetricreceiver.MetricBatch) error {
	if batch == nil || len(batch.Families) == 0 {
		return nil
	}
	if batch.TimestampNs == 0 {
		batch.TimestampNs = w.nowMilli() * 1e6
	}
	ts, samples := buildRows(batch)
	if len(ts) == 0 && len(samples) == 0 {
		return nil
	}
	if err := w.writeTimeSeries(ctx, ts); err != nil {
		return fmt.Errorf("datastoremetrics: write time_series: %w", err)
	}
	if err := w.writeSamples(ctx, samples); err != nil {
		return fmt.Errorf("datastoremetrics: write samples: %w", err)
	}
	return nil
}

// buildRows is the pure batch → rows transformation — all fingerprinting and
// classic-shape decomposition, no IO. Kept free of the connection so it is
// exhaustively unit-testable.
func buildRows(batch *zapmetricreceiver.MetricBatch) (tsRows []tsRow, sampleRows []sampleRow) {
	resourceAttrs := normalizedResource(batch)
	env := resourceAttrs["deployment.environment"]

	resourceHash := fingerprint(initialOffset, resourceAttrs)
	// The ZAP wire carries no instrumentation scope; the scope layer is empty,
	// so its hash is just the resource hash carried forward.
	scopeAttrs := map[string]string{}
	scopeHash := fingerprint(resourceHash, scopeAttrs)

	unixMilli := batch.TimestampNs / 1e6
	tsUnixMilli := unixMilli / 3600000 * 3600000 // floor to the hour

	seenTS := make(map[[2]uint64]bool)

	emit := func(name, desc, unit, typ, temporality string, isMonotonic bool, baseLabels, extras map[string]string, value float64) {
		point := mergeAttrs(baseLabels, extras)
		fp := hashWithName(fingerprint(scopeHash, point), name)

		sampleRows = append(sampleRows, sampleRow{
			env: env, temporality: temporality, metricName: name,
			fingerprint: fp, unixMilli: unixMilli, value: value,
		})

		key := [2]uint64{fp, uint64(tsUnixMilli)}
		if seenTS[key] {
			return // same series already described this hour — one metadata row suffices
		}
		seenTS[key] = true
		tsRows = append(tsRows, tsRow{
			env: env, temporality: temporality, metricName: name, description: desc, unit: unit,
			typ: typ, isMonotonic: isMonotonic, fingerprint: fp, unixMilli: tsUnixMilli,
			labels: labelsJSON(name, point, scopeAttrs, resourceAttrs), attrs: point,
			scopeAttrs: scopeAttrs, resourceAttrs: resourceAttrs,
		})
	}

	for i := range batch.Families {
		fam := &batch.Families[i]
		desc := fam.Help
		for j := range fam.Metrics {
			m := &fam.Metrics[j]
			labels := m.Labels
			switch fam.Type {
			case "counter":
				if m.Value == nil {
					continue
				}
				emit(fam.Name, desc, "", typeSum, temporalityCumulative, true,
					labels, tempExtras(temporalityCumulative), *m.Value)
			case "gauge":
				if m.Value == nil {
					continue
				}
				emit(fam.Name, desc, "", typeGauge, temporalityUnspecified, false,
					labels, tempExtras(temporalityUnspecified), *m.Value)
			case "histogram":
				emitHistogram(emit, fam.Name, desc, labels, m)
			case "summary":
				emitSummary(emit, fam.Name, desc, labels, m)
			default:
				// unknown family type: skip rather than write a malformed row
			}
		}
	}
	return tsRows, sampleRows
}

// emitFunc is the closure buildRows hands to the per-type decomposers.
type emitFunc func(name, desc, unit, typ, temporality string, isMonotonic bool, baseLabels, extras map[string]string, value float64)

// emitHistogram decomposes a classic histogram into `.count`, `.sum` and
// cumulative `.bucket{le=…}` samples (including le=+Inf) — the exact series the
// query plane's histogram_quantile expects.
func emitHistogram(emit emitFunc, name, desc string, labels map[string]string, m *zapmetricreceiver.Metric) {
	if m.SampleCount != nil {
		emit(name+suffixCount, desc, unitCount, typeSum, temporalityCumulative, true,
			labels, tempExtras(temporalityCumulative), float64(*m.SampleCount))
	}
	if m.SampleSum != nil {
		emit(name+suffixSum, desc, "", typeSum, temporalityCumulative, true,
			labels, tempExtras(temporalityCumulative), *m.SampleSum)
	}
	var lastCumulative float64
	for _, b := range m.Buckets {
		lastCumulative = float64(b.CumulativeCount)
		emit(name+suffixBucket, desc, "", typeHistogram, temporalityCumulative, true,
			labels, leExtras(strconv.FormatFloat(b.UpperBound, 'f', -1, 64), temporalityCumulative),
			float64(b.CumulativeCount))
	}
	// le=+Inf carries the total count. Prefer SampleCount; fall back to the last
	// cumulative bucket when the sender omitted it.
	infValue := lastCumulative
	if m.SampleCount != nil {
		infValue = float64(*m.SampleCount)
	} else if len(m.Buckets) == 0 {
		return // nothing to anchor +Inf to
	}
	emit(name+suffixBucket, desc, "", typeHistogram, temporalityCumulative, true,
		labels, leExtras("+Inf", temporalityCumulative), infValue)
}

// emitSummary decomposes a summary into `.count`, `.sum` and
// `.quantile{quantile=…}` samples.
func emitSummary(emit emitFunc, name, desc string, labels map[string]string, m *zapmetricreceiver.Metric) {
	if m.SampleCount != nil {
		emit(name+suffixCount, desc, unitCount, typeSum, temporalityCumulative, true,
			labels, tempExtras(temporalityCumulative), float64(*m.SampleCount))
	}
	if m.SampleSum != nil {
		emit(name+suffixSum, desc, "", typeSum, temporalityCumulative, true,
			labels, tempExtras(temporalityCumulative), *m.SampleSum)
	}
	for _, q := range m.Quantiles {
		emit(name+suffixQuantile, desc, "", typeSummary, temporalityCumulative, true,
			labels, quantileExtras(strconv.FormatFloat(q.Quantile, 'f', -1, 64), temporalityCumulative),
			q.Value)
	}
}

// normalizedResource copies the batch resource map and lifts AppName / Version
// into the OTLP service.* attributes when absent, so dashboards can group by
// service.name.
func normalizedResource(batch *zapmetricreceiver.MetricBatch) map[string]string {
	r := make(map[string]string, len(batch.Resource)+2)
	for k, v := range batch.Resource {
		r[k] = v
	}
	if batch.AppName != "" {
		if _, ok := r["service.name"]; !ok {
			r["service.name"] = batch.AppName
		}
	}
	if batch.Version != "" {
		if _, ok := r["service.version"]; !ok {
			r["service.version"] = batch.Version
		}
	}
	return r
}

func tempExtras(t string) map[string]string { return map[string]string{"__temporality__": t} }

func leExtras(le, t string) map[string]string {
	return map[string]string{"le": le, "__temporality__": t}
}

func quantileExtras(q, t string) map[string]string {
	return map[string]string{"quantile": q, "__temporality__": t}
}

func (w *Writer) writeTimeSeries(ctx context.Context, rows []tsRow) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := w.conn.PrepareBatch(ctx,
		fmt.Sprintf(timeSeriesSQLTmpl, w.db, w.tsTable), driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := w.nowMilli()
	for _, r := range rows {
		if err := stmt.Append(
			r.env, r.temporality, r.metricName, r.description, r.unit, r.typ,
			r.isMonotonic, r.fingerprint, r.unixMilli, r.labels,
			r.attrs, r.scopeAttrs, r.resourceAttrs, false, now,
		); err != nil {
			return err
		}
	}
	return stmt.Send()
}

func (w *Writer) writeSamples(ctx context.Context, rows []sampleRow) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := w.conn.PrepareBatch(ctx,
		fmt.Sprintf(samplesSQLTmpl, w.db, w.samplesTable), driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := w.nowMilli()
	for _, r := range rows {
		if err := stmt.Append(
			r.env, r.temporality, r.metricName, r.fingerprint, r.unixMilli, r.value, r.flags, now,
		); err != nil {
			return err
		}
	}
	return stmt.Send()
}
