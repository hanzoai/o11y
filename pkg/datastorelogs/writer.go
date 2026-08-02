// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package datastorelogs is the o11y-native datastore log driver: it writes the
// log signal to the event plane — event.log plus the support tables declared
// beside pkg/telemetrylogs/tables.go (log_attribute, log_key, log_resource_key,
// log_resource) — over upstream datastore-go v2.
//
// It is the write-side companion to pkg/zaplogreceiver, exactly as
// pkg/datastoremetrics is to pkg/zapmetricreceiver: WriteLogs satisfies
// zaplogreceiver.Handler, so a receiver dispatches each decoded batch straight
// into the datastore:
//
//	w := datastorelogs.NewWriter(store.Datastore())
//	rcv, _ := zaplogreceiver.New(zaplogreceiver.Config{OnBatch: w.WriteLogs})
//
// WHY THIS EXISTS. Before it, the log lane's only wired path was the embedded
// otelcol pipeline in hanzoai/cloud (datastorelogsexporter), which writes the
// OTLP-fork family (o11y_logs.distributed_logs_v2 + its four side tables) that
// the event-plane readers do not read. This driver is the one log write path
// onto the envelope those readers query.
//
// IDEMPOTENT RE-INGEST. event.log is ReplacingMergeTree(ingested_at) with
// identity (org, service, time, id). The wire carries no record id, so id is a
// content hash over every payload field — identity is a pure function of the
// input, re-sending a batch produces new VERSIONS of the same identities, and
// a Replacing merge collapses them to one row. (Two genuinely identical
// records in one batch collapse too; senders that need multiplicity carry a
// distinguishing attribute, the same contract every content-addressed log
// store has.) The support tables are ReplacingMergeTree on their full
// identity, so re-emission is dedup by construction.
//
// RESOURCE CONTRACT. The batch's resource attributes hash to one FNV-1a 64
// number over their canonical JSON: event.log stores the number (resource
// UInt64), event.log_resource stores the DECIMAL string of the same number
// beside the JSON (fingerprint, labels) — string and numeric forms
// interchangeable via toUInt64/toString. pkg/datastoretraces uses the
// identical scheme for span_resource.
package datastorelogs

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/hanzo-ds/go"
	"github.com/hanzo-ds/go/lib/driver"
	"github.com/hanzoai/o11y/pkg/sightings"
	"github.com/hanzoai/o11y/pkg/telemetrylogs"
	"github.com/hanzoai/o11y/pkg/zaplogreceiver"
)

// Field contexts and data types — the exact strings the read plane filters on
// (FieldContext.TagType() / FieldDataType.TagDataType()).
const (
	contextTag      = "tag"
	contextResource = "resource"

	dataTypeString  = "string"
	dataTypeFloat64 = "float64"
	dataTypeBool    = "bool"
)

// resourceBucketSeconds is the seen_at_ts_bucket_start granularity of
// event.log_resource — the 1800s window the read plane's resource CTE looks
// back by.
const resourceBucketSeconds = 1800

// INSERT templates — column lists match the applied event-plane schema.
// Omitted envelope columns (kind, el, anonymous_id, person_id, host) take
// their defaults; host is DEFAULT domain(url). ingested_at is passed
// explicitly: it is the Replacing version, and an injected clock keeps it
// deterministic under test.
const (
	logSQLTmpl         = "INSERT INTO %s.%s (org, time, ingested_at, id, name, product, session_id, distinct_id, url, path, attributes, service, severity_text, severity_number, body, trace_id, span_id, resource, resource_fingerprint) VALUES"
	attributeSQLTmpl   = "INSERT INTO %s.%s (org, tag_key, tag_type, tag_data_type, string_value, number_value, unix_milli) VALUES"
	keySQLTmpl         = "INSERT INTO %s.%s (org, name, datatype) VALUES"
	resourceKeySQLTmpl = "INSERT INTO %s.%s (org, name, datatype) VALUES"
	resourceSQLTmpl    = "INSERT INTO %s.%s (org, fingerprint, labels, seen_at_ts_bucket_start) VALUES"
)

// logRow maps 1:1 to the inserted event.log columns (minus write-time
// ingested_at).
type logRow struct {
	org            string
	time           time.Time
	id             string
	name           string
	product        string
	sessionID      string
	distinctID     string
	url            string
	path           string
	attrs          map[string]string
	service        string
	severityText   string
	severityNumber uint8
	body           string
	traceID        string
	spanID         string
	resource       uint64
	// resourceFingerprint is the decimal string of resource — the same value
	// log_resource stores, so the read plane's __resource_filter CTE matches
	// rows written here.
	resourceFingerprint string
}

// attrRow is one event.log_attribute value sighting; the whole row is its
// identity, so a map dedups within the batch exactly as the Replacing merge
// dedups across batches.
type attrRow struct {
	org         string
	tagKey      string
	tagType     string
	tagDataType string
	stringValue string
	numberValue float64
}

// keyRow is one event.log_key / event.log_resource_key sighting — the two
// tables share one schema and differ only in field context.
type keyRow struct {
	org      string
	name     string
	datatype string
}

// resourceRow is one event.log_resource sighting: the batch's resource
// identity observed in one 30-minute bucket.
type resourceRow struct {
	org         string
	fingerprint string
	labels      string
	bucket      int64
}

// rows is the full decomposition of one batch.
type rows struct {
	logs         []logRow
	attrs        []attrRow
	keys         []keyRow
	resourceKeys []keyRow
	resources    []resourceRow
}

// Writer ingests decoded log batches into the event plane over upstream
// datastore-go v2. Safe for concurrent use (datastore.Conn is).
type Writer struct {
	conn datastore.Conn
	db   string
	org  string
	now  func() time.Time

	// One sighting Set per DIMENSION table. Not one per row TYPE: log_key and
	// log_resource_key are both keyRow, and a shared Set would let a key seen
	// in one table suppress the same key in the other.
	//
	// event.log itself has no Set — a log line is a fact, never a repeat.
	seenAttrs        *sightings.Set[bucketed[attrRow]]
	seenKeys         *sightings.Set[bucketed[keyRow]]
	seenResourceKeys *sightings.Set[bucketed[keyRow]]
	seenResources    *sightings.Set[resourceRow] // already carries its bucket
}

// bucketed qualifies a sighting by the time window it was seen in, so a
// dimension row is rewritten once per bucket instead of once per batch (never
// rewriting it would freeze the tables' recency columns).
type bucketed[T comparable] struct {
	bucket int64
	row    T
}

// Option configures a Writer.
type Option func(*Writer)

// WithDatabase overrides the target database (default: the event plane).
func WithDatabase(db string) Option { return func(w *Writer) { w.db = db } }

// WithOrg sets the tenant stamped on rows whose batch resource carries no
// "org" attribute (default "hanzo" — the deployment's own tenant).
func WithOrg(org string) Option { return func(w *Writer) { w.org = org } }

// WithNow injects the clock (tests). It feeds ingested_at (the Replacing
// version), the support tables' unix_milli version, and the zero-time
// fallbacks.
func WithNow(now func() time.Time) Option { return func(w *Writer) { w.now = now } }

// NewWriter builds a Writer over an existing datastore connection. Defaults
// target the same database + tables the event-plane readers query, so a
// record written here is immediately searchable.
func NewWriter(conn datastore.Conn, opts ...Option) *Writer {
	w := &Writer{
		conn:             conn,
		db:               telemetrylogs.DBName,
		org:              "hanzo",
		now:              func() time.Time { return time.Now().UTC() },
		seenAttrs:        sightings.New[bucketed[attrRow]](0),
		seenKeys:         sightings.New[bucketed[keyRow]](0),
		seenResourceKeys: sightings.New[bucketed[keyRow]](0),
		seenResources:    sightings.New[resourceRow](0),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// WriteLogs is the zaplogreceiver.Handler: it decomposes a batch into the five
// event-plane tables and writes them all. A nil / empty batch is a no-op.
func (w *Writer) WriteLogs(ctx context.Context, batch *zaplogreceiver.LogBatch) error {
	if batch == nil || len(batch.Records) == 0 {
		return nil
	}
	now := w.now()
	r := buildRows(batch, orgOf(batch, w.org), now)
	nowMilli := now.UnixMilli()

	if err := send(ctx, w.conn, fmt.Sprintf(logSQLTmpl, w.db, telemetrylogs.LogTableName), r.logs,
		func(b driver.Batch, l logRow) error {
			return b.Append(l.org, l.time, now, l.id, l.name, l.product, l.sessionID, l.distinctID,
				l.url, l.path, l.attrs, l.service, l.severityText, l.severityNumber, l.body,
				l.traceID, l.spanID, l.resource, l.resourceFingerprint)
		}); err != nil {
		return fmt.Errorf("datastorelogs: write log: %w", err)
	}
	// The four DIMENSION tables below are sightings, ~99.7% of which repeat
	// batch after batch. Each suppressed set is one INSERT round-trip not taken
	// on the receiver's connection handler, which calls this writer
	// synchronously — so this is intake latency, not just database load.
	bucket := bucketOf(now)
	if err := sightings.Write(w.seenAttrs, r.attrs,
		func(a attrRow) bucketed[attrRow] { return bucketed[attrRow]{bucket, a} },
		func(novel []attrRow) error {
			return send(ctx, w.conn, fmt.Sprintf(attributeSQLTmpl, w.db, telemetrylogs.LogAttributeTableName), novel,
				func(b driver.Batch, a attrRow) error {
					return b.Append(a.org, a.tagKey, a.tagType, a.tagDataType, a.stringValue, a.numberValue, nowMilli)
				})
		}); err != nil {
		return fmt.Errorf("datastorelogs: write log_attribute: %w", err)
	}
	if err := sightings.Write(w.seenKeys, r.keys,
		func(k keyRow) bucketed[keyRow] { return bucketed[keyRow]{bucket, k} },
		func(novel []keyRow) error {
			return send(ctx, w.conn, fmt.Sprintf(keySQLTmpl, w.db, telemetrylogs.LogKeyTableName), novel,
				func(b driver.Batch, k keyRow) error {
					return b.Append(k.org, k.name, k.datatype)
				})
		}); err != nil {
		return fmt.Errorf("datastorelogs: write log_key: %w", err)
	}
	if err := sightings.Write(w.seenResourceKeys, r.resourceKeys,
		func(k keyRow) bucketed[keyRow] { return bucketed[keyRow]{bucket, k} },
		func(novel []keyRow) error {
			return send(ctx, w.conn, fmt.Sprintf(resourceKeySQLTmpl, w.db, telemetrylogs.LogResourceKeyTableName), novel,
				func(b driver.Batch, k keyRow) error {
					return b.Append(k.org, k.name, k.datatype)
				})
		}); err != nil {
		return fmt.Errorf("datastorelogs: write log_resource_key: %w", err)
	}
	if err := sightings.Write(w.seenResources, r.resources,
		func(res resourceRow) resourceRow { return res }, // bucket is already a field
		func(novel []resourceRow) error {
			return send(ctx, w.conn, fmt.Sprintf(resourceSQLTmpl, w.db, telemetrylogs.LogResourceTableName), novel,
				func(b driver.Batch, res resourceRow) error {
					return b.Append(res.org, res.fingerprint, res.labels, res.bucket)
				})
		}); err != nil {
		return fmt.Errorf("datastorelogs: write log_resource: %w", err)
	}
	return nil
}

// bucketOf is the resource-bucket floor, the one window every sighting in this
// package is qualified by — the same 1800s grid event.log_resource stores.
func bucketOf(t time.Time) int64 {
	return t.Unix() / resourceBucketSeconds * resourceBucketSeconds
}

// send is the one batch-INSERT shape: prepare, append every row, send.
// Empty input is a no-op so callers never open a batch they won't fill.
func send[T any](ctx context.Context, conn datastore.Conn, sql string, rows []T, appendRow func(driver.Batch, T) error) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if err := appendRow(stmt, r); err != nil {
			return err
		}
	}
	return stmt.Send()
}

// buildRows is the pure batch → rows transformation — normalization,
// classification, dedup and identity, no IO. Kept free of the connection so it
// is exhaustively unit-testable.
func buildRows(batch *zaplogreceiver.LogBatch, org string, now time.Time) rows {
	resource := normalizedResource(batch)
	service := resource["service.name"]
	fp, labels := resourceIdentity(resource)
	fpStr := strconv.FormatUint(fp, 10)

	var out rows
	seenAttr := map[attrRow]struct{}{}
	seenKey := map[keyRow]struct{}{}
	seenResourceKey := map[keyRow]struct{}{}
	seenBucket := map[int64]struct{}{}

	for k := range resource {
		rk := keyRow{org: org, name: k, datatype: dataTypeString}
		if _, ok := seenResourceKey[rk]; !ok {
			seenResourceKey[rk] = struct{}{}
			out.resourceKeys = append(out.resourceKeys, rk)
		}
		a := attrRow{org: org, tagKey: k, tagType: contextResource, tagDataType: dataTypeString, stringValue: resource[k]}
		if _, ok := seenAttr[a]; !ok {
			seenAttr[a] = struct{}{}
			out.attrs = append(out.attrs, a)
		}
	}

	for i := range batch.Records {
		rec := &batch.Records[i]

		ts := now
		switch {
		case rec.TimeUnixNs > 0:
			ts = time.Unix(0, rec.TimeUnixNs).UTC()
		case rec.ObservedTimeUnixNs > 0:
			ts = time.Unix(0, rec.ObservedTimeUnixNs).UTC()
		}

		attrs := make(map[string]string, len(rec.Attributes))
		for k, v := range rec.Attributes {
			dataType, stringValue, numberValue := classify(v)
			attrs[k] = stringValue
			a := attrRow{org: org, tagKey: k, tagType: contextTag, tagDataType: dataType, stringValue: stringValue, numberValue: numberValue}
			if _, ok := seenAttr[a]; !ok {
				seenAttr[a] = struct{}{}
				out.attrs = append(out.attrs, a)
			}
			kr := keyRow{org: org, name: k, datatype: dataType}
			if _, ok := seenKey[kr]; !ok {
				seenKey[kr] = struct{}{}
				out.keys = append(out.keys, kr)
			}
		}

		out.logs = append(out.logs, logRow{
			org:                 org,
			time:                ts,
			id:                  recordID(rec, service, attrs),
			name:                rec.EventName,
			product:             resource["product"],
			sessionID:           pick(attrs, "session.id", "session_id"),
			distinctID:          pick(attrs, "distinct.id", "distinct_id"),
			url:                 pick(attrs, "url.full", "http.url", "url"),
			path:                pick(attrs, "url.path", "http.target", "http.route", "path"),
			attrs:               attrs,
			service:             service,
			severityText:        rec.SeverityText,
			severityNumber:      clampSeverity(rec.Severity),
			body:                rec.Body,
			traceID:             rec.TraceID,
			spanID:              rec.SpanID,
			resource:            fp,
			resourceFingerprint: fpStr,
		})

		bucket := ts.Unix()
		bucket -= bucket % resourceBucketSeconds
		if _, ok := seenBucket[bucket]; !ok {
			seenBucket[bucket] = struct{}{}
			out.resources = append(out.resources, resourceRow{org: org, fingerprint: fpStr, labels: labels, bucket: bucket})
		}
	}
	return out
}

// recordID is the deterministic identity of one record: FNV-1a 64 over every
// payload field, hex-rendered. Idempotent re-ingest needs identity to be a
// function of content, never a random draw.
func recordID(rec *zaplogreceiver.LogRecord, service string, attrs map[string]string) string {
	h := fnv.New64a()
	for _, part := range []string{
		strconv.FormatInt(rec.TimeUnixNs, 10),
		strconv.Itoa(rec.Severity),
		rec.SeverityText,
		rec.Body,
		rec.EventName,
		rec.TraceID,
		rec.SpanID,
		service,
	} {
		h.Write([]byte(part))
		h.Write([]byte{0xff})
	}
	b, _ := json.Marshal(attrs)
	h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

// normalizedResource copies the batch resource map and lifts AppName / Version
// into the OTLP service.* attributes when absent — the same lift
// datastoremetrics applies, so one sender groups identically across signals.
func normalizedResource(batch *zaplogreceiver.LogBatch) map[string]string {
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

// orgOf resolves the tenant: the batch's own resource wins, the writer's
// default backs it.
func orgOf(batch *zaplogreceiver.LogBatch, def string) string {
	if o := batch.Resource["org"]; o != "" {
		return o
	}
	return def
}

// resourceIdentity is the resource → (fingerprint, labels) value: labels is
// the canonical JSON of the attribute map (encoding/json sorts map keys, so it
// is deterministic), and the fingerprint is FNV-1a 64 of exactly those bytes.
func resourceIdentity(attrs map[string]string) (uint64, string) {
	b, _ := json.Marshal(attrs)
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64(), string(b)
}

// classify types one wire attribute for the support tables: the datastore's
// three tag data types, plus the value in both string and numeric form. The
// string form is the JSON scalar encoding (strings raw, everything else
// json.Marshal), which is also what the envelope's attributes map stores — one
// representation everywhere.
func classify(v any) (dataType, stringValue string, numberValue float64) {
	switch t := v.(type) {
	case string:
		return dataTypeString, t, 0
	case bool:
		return dataTypeBool, strconv.FormatBool(t), 0
	case float64:
		return dataTypeFloat64, jsonScalar(t), t
	case float32:
		return dataTypeFloat64, jsonScalar(float64(t)), float64(t)
	case int:
		return dataTypeFloat64, strconv.Itoa(t), float64(t)
	case int64:
		return dataTypeFloat64, strconv.FormatInt(t, 10), float64(t)
	case uint64:
		return dataTypeFloat64, strconv.FormatUint(t, 10), float64(t)
	case nil:
		return dataTypeString, "", 0
	default:
		return dataTypeString, jsonScalar(t), 0
	}
}

// jsonScalar renders any value as its JSON encoding, falling back to fmt for
// the values JSON cannot carry (NaN, ±Inf).
func jsonScalar(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// pick returns the first present key's value.
func pick(attrs map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			return v
		}
	}
	return ""
}

// clampSeverity bounds the wire severity into the envelope's UInt8.
func clampSeverity(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
