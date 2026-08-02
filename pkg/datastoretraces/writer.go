// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package datastoretraces is the o11y-native datastore span driver: it writes
// the trace signal to the event plane — event.span plus the support tables
// declared beside pkg/telemetrytraces/tables.go (span_attribute, span_key,
// span_resource, operation, trace) — over upstream datastore-go v2.
//
// It is the write-side companion to pkg/zapreceiver, exactly as
// pkg/datastoremetrics is to pkg/zapmetricreceiver: WriteSpans satisfies
// zapreceiver.Handler, so a receiver dispatches each decoded batch straight
// into the datastore:
//
//	w := datastoretraces.NewWriter(store.Datastore())
//	rcv, _ := zapreceiver.New(zapreceiver.Config{OnBatch: w.WriteSpans})
//
// WHY THIS EXISTS. Before it, the span lane had NO event-plane writer: the only
// wired path was the embedded otelcol pipeline in hanzoai/cloud
// (datastoretracesexporter), which writes the OTLP-fork family
// (o11y_traces.o11y_index_v3) that the event-plane readers do not read. This
// driver is the one span write path onto the envelope those readers query.
//
// IDEMPOTENT RE-INGEST. event.span is ReplacingMergeTree(ingested_at) with
// identity (org, trace_id, time, id). id is the span's own span_id — or a
// content hash when the sender omitted one — so every identity column is a
// pure function of the input, and re-sending a batch produces new VERSIONS of
// the same identities, which a Replacing merge collapses to one row. The
// support tables are ReplacingMergeTree keyed on their full identity for the
// same reason. The one exception is event.trace (AggregatingMergeTree): its
// num_spans is a SimpleAggregateFunction(sum) folded from per-batch partials,
// so re-ingesting a batch adds its span count again — start/end stay exact
// because min/max absorb duplicates; the count is the documented cost of the
// partial-summary design and is outside the Replacing idempotency contract.
package datastoretraces

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/hanzo-ds/go"
	"github.com/hanzo-ds/go/lib/driver"
	"github.com/hanzoai/o11y/pkg/sightings"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"
	"github.com/hanzoai/o11y/pkg/zapreceiver"
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
// event.span_resource — the same 1800s window the read plane's resource CTE
// looks back by (statement_builder pins start-1800 against the bucket).
const resourceBucketSeconds = 1800

// spanEventsKey and spanStatusMessageKey are where the two span fields with no
// envelope column of their own land in the attributes map: event.span carries
// neither an events array nor a status message by design, and dropping them
// would be silent data loss, so they travel under reserved keys. The trace read
// plane resolves `events` and `status_message` to exactly these two keys
// (spantypes.ExprEvents, indexV3Columns["status_message"]) — one name per value,
// written and read in one place each.
const (
	spanEventsKey        = "span.events"
	spanStatusMessageKey = "status.message"
)

// INSERT templates — column lists match the applied event-plane schema.
// Omitted envelope columns (el, anonymous_id, person_id, host) take their
// defaults; host is DEFAULT domain(url), so it materializes from the url we
// insert.
//
// ingested_at IS DELIBERATELY NOT BOUND. It is three things at once in this
// table's DDL:
//
//	ReplacingMergeTree(ingested_at)     -- the version that decides which
//	                                       duplicate row survives a merge
//	PARTITION BY toDate(ingested_at)    -- which partition the row lands in
//	TTL toDateTime(ingested_at) + 30d   -- when the row is deleted
//
// so a writer that binds it hands all three to whoever produced the batch. A
// sender could keep its spans forever by dating them forward, drop them
// immediately by dating them back, scatter one batch across partitions, or win
// a Replacing merge against a newer row. None of that is a retention policy —
// it is the absence of one.
//
// DEFAULT now64(3) is on the column, so leaving it out is not a gap: the server
// stamps its own arrival time, which is the only clock that can be trusted to
// mean "when we received this". cloud's planesink.go already omits it and its
// TestRetentionIsNotARequestParameter forbids binding it; this writer was the
// divergent one, and it is the side that owns the DDL.
const (
	spanSQLTmpl      = "INSERT INTO %s.%s (org, time, id, name, kind, product, session_id, distinct_id, url, path, attributes, service, trace_id, span_id, parent, duration, status, resource_fingerprint) VALUES"
	attributeSQLTmpl = "INSERT INTO %s.%s (org, tag_key, tag_type, tag_data_type, string_value, number_value, unix_milli) VALUES"
	keySQLTmpl       = "INSERT INTO %s.%s (org, tagKey, tagType, dataType, isColumn, unix_milli) VALUES"
	resourceSQLTmpl  = "INSERT INTO %s.%s (org, seen_at_ts_bucket_start, fingerprint, labels) VALUES"
	operationSQLTmpl = "INSERT INTO %s.%s (org, serviceName, name, time) VALUES"
	traceSQLTmpl     = "INSERT INTO %s.%s (org, trace_id, start, end, num_spans) VALUES"
)

// spanRow maps 1:1 to the inserted event.span columns. ingested_at is not among
// them and must not be added: the server defaults it (see spanSQLTmpl).
type spanRow struct {
	org        string
	time       time.Time
	id         string
	name       string
	kind       string
	product    string
	sessionID  string
	distinctID string
	url        string
	path       string
	attrs      map[string]string
	service    string
	traceID    string
	spanID     string
	parent     string
	duration   uint64
	status     string
	// resourceFingerprint is the decimal string of the batch's resource
	// fingerprint — the same value span_resource stores, so the read plane's
	// __resource_filter CTE matches rows written here.
	resourceFingerprint string
}

// attrRow is one event.span_attribute value sighting. Comparable on purpose:
// the whole row is its identity, so a map dedups within the batch exactly as
// the Replacing merge dedups across batches.
type attrRow struct {
	org         string
	tagKey      string
	tagType     string
	tagDataType string
	stringValue string
	numberValue float64
}

// keyRow is one event.span_key sighting (isColumn is always false: every
// wire attribute lives in the attributes map, not a dedicated column).
type keyRow struct {
	org      string
	tagKey   string
	tagType  string
	dataType string
}

// resourceRow is one event.span_resource sighting: the batch's resource
// identity observed in one 30-minute bucket.
type resourceRow struct {
	org         string
	bucket      int64
	fingerprint string
	labels      string
}

// operationRow is one event.operation sighting; time is the latest span start
// observed for the (service, name) pair in this batch.
type operationRow struct {
	org     string
	service string
	name    string
	time    time.Time
}

// traceRow is one event.trace PARTIAL: this batch's contribution to the
// trace's summary, folded by the AggregatingMergeTree.
type traceRow struct {
	org      string
	traceID  string
	start    time.Time
	end      time.Time
	numSpans uint64
}

// rows is the full decomposition of one batch.
type rows struct {
	spans      []spanRow
	attrs      []attrRow
	keys       []keyRow
	resources  []resourceRow
	operations []operationRow
	traces     []traceRow
}

// Writer ingests decoded span batches into the event plane over upstream
// datastore-go v2. Safe for concurrent use (datastore.Conn is).
type Writer struct {
	conn datastore.Conn
	db   string
	org  string
	now  func() time.Time

	// One sighting Set per DIMENSION table, suppressing the repeats that make
	// up almost all dimension traffic. See pkg/sightings.
	//
	// event.span and event.trace have NO Set, for different reasons: a span is
	// a fact, and a trace row is a PARTIAL contribution folded by an
	// AggregatingMergeTree — suppressing a repeat there would silently drop
	// spans from the trace's own span count.
	seenAttrs      *sightings.Set[bucketed[attrRow]]
	seenKeys       *sightings.Set[bucketed[keyRow]]
	seenResources  *sightings.Set[resourceRow] // already carries its bucket
	seenOperations *sightings.Set[bucketed[operationIdentity]]
}

// bucketed qualifies a sighting by the window it was seen in, so a dimension
// row refreshes once per bucket instead of once per batch.
type bucketed[T comparable] struct {
	bucket int64
	row    T
}

// operationIdentity is an operationRow WITHOUT its time: the row's time is the
// latest span start in the batch, so it differs every batch and the identity
// that should be deduplicated is the (service, name) pair alone. Bucketing it
// keeps event.operation's time column fresh to one window.
type operationIdentity struct {
	org     string
	service string
	name    string
}

// Option configures a Writer.
type Option func(*Writer)

// WithDatabase overrides the target database (default: the event plane).
func WithDatabase(db string) Option { return func(w *Writer) { w.db = db } }

// WithOrg sets the tenant stamped on rows whose batch resource carries no
// "org" attribute (default "hanzo" — the deployment's own tenant).
func WithOrg(org string) Option { return func(w *Writer) { w.org = org } }

// WithNow injects the clock (tests). It feeds the support tables' unix_milli
// version, the resource bucket, and the zero-time fallbacks — NOT event.span's
// ingested_at, which the server defaults (see spanSQLTmpl).
func WithNow(now func() time.Time) Option { return func(w *Writer) { w.now = now } }

// NewWriter builds a Writer over an existing datastore connection. Defaults
// target the same database + tables the event-plane readers query, so a span
// written here is immediately searchable.
func NewWriter(conn datastore.Conn, opts ...Option) *Writer {
	w := &Writer{
		conn:           conn,
		db:             telemetrytraces.DBName,
		org:            "hanzo",
		now:            func() time.Time { return time.Now().UTC() },
		seenAttrs:      sightings.New[bucketed[attrRow]](0),
		seenKeys:       sightings.New[bucketed[keyRow]](0),
		seenResources:  sightings.New[resourceRow](0),
		seenOperations: sightings.New[bucketed[operationIdentity]](0),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// WriteSpans is the zapreceiver.Handler: it decomposes a batch into the six
// event-plane tables and writes them all. A nil / empty batch is a no-op.
func (w *Writer) WriteSpans(ctx context.Context, batch *zapreceiver.SpanBatch) error {
	if batch == nil || len(batch.Spans) == 0 {
		return nil
	}
	now := w.now()
	r := buildRows(batch, orgOf(batch, w.org), now)
	nowMilli := now.UnixMilli()

	if err := send(ctx, w.conn, fmt.Sprintf(spanSQLTmpl, w.db, telemetrytraces.SpanTableName), r.spans,
		func(b driver.Batch, s spanRow) error {
			// No ingested_at: the server's DEFAULT now64(3) sets it. See the
			// template's comment — that column is the version, the partition key
			// and the TTL anchor, and none of the three belong to the sender.
			return b.Append(s.org, s.time, s.id, s.name, s.kind, s.product, s.sessionID, s.distinctID,
				s.url, s.path, s.attrs, s.service, s.traceID, s.spanID, s.parent, s.duration, s.status,
				s.resourceFingerprint)
		}); err != nil {
		return fmt.Errorf("datastoretraces: write span: %w", err)
	}
	// The four DIMENSION tables below are sightings that repeat batch after
	// batch. Each suppressed set is one INSERT round-trip not taken on the
	// receiver's connection handler, which calls this writer synchronously.
	bucket := bucketOf(now)
	if err := sightings.Write(w.seenAttrs, r.attrs,
		func(a attrRow) bucketed[attrRow] { return bucketed[attrRow]{bucket, a} },
		func(novel []attrRow) error {
			return send(ctx, w.conn, fmt.Sprintf(attributeSQLTmpl, w.db, telemetrytraces.SpanAttributeTableName), novel,
				func(b driver.Batch, a attrRow) error {
					return b.Append(a.org, a.tagKey, a.tagType, a.tagDataType, a.stringValue, a.numberValue, nowMilli)
				})
		}); err != nil {
		return fmt.Errorf("datastoretraces: write span_attribute: %w", err)
	}
	if err := sightings.Write(w.seenKeys, r.keys,
		func(k keyRow) bucketed[keyRow] { return bucketed[keyRow]{bucket, k} },
		func(novel []keyRow) error {
			return send(ctx, w.conn, fmt.Sprintf(keySQLTmpl, w.db, telemetrytraces.SpanKeyTableName), novel,
				func(b driver.Batch, k keyRow) error {
					return b.Append(k.org, k.tagKey, k.tagType, k.dataType, false, nowMilli)
				})
		}); err != nil {
		return fmt.Errorf("datastoretraces: write span_key: %w", err)
	}
	if err := sightings.Write(w.seenResources, r.resources,
		func(res resourceRow) resourceRow { return res }, // bucket is already a field
		func(novel []resourceRow) error {
			return send(ctx, w.conn, fmt.Sprintf(resourceSQLTmpl, w.db, telemetrytraces.SpanResourceTableName), novel,
				func(b driver.Batch, res resourceRow) error {
					return b.Append(res.org, res.bucket, res.fingerprint, res.labels)
				})
		}); err != nil {
		return fmt.Errorf("datastoretraces: write span_resource: %w", err)
	}
	if err := sightings.Write(w.seenOperations, r.operations,
		func(o operationRow) bucketed[operationIdentity] {
			return bucketed[operationIdentity]{bucket, operationIdentity{o.org, o.service, o.name}}
		},
		func(novel []operationRow) error {
			return send(ctx, w.conn, fmt.Sprintf(operationSQLTmpl, w.db, telemetrytraces.OperationTableName), novel,
				func(b driver.Batch, o operationRow) error {
					return b.Append(o.org, o.service, o.name, o.time)
				})
		}); err != nil {
		return fmt.Errorf("datastoretraces: write operation: %w", err)
	}
	if err := send(ctx, w.conn, fmt.Sprintf(traceSQLTmpl, w.db, telemetrytraces.TraceTableName), r.traces,
		func(b driver.Batch, t traceRow) error {
			return b.Append(t.org, t.traceID, t.start, t.end, t.numSpans)
		}); err != nil {
		return fmt.Errorf("datastoretraces: write trace: %w", err)
	}
	return nil
}

// bucketOf is the resource-bucket floor, the one window every sighting in this
// package is qualified by — the same 1800s grid event.span_resource stores.
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
// classification, dedup and folding, no IO. Kept free of the connection so it
// is exhaustively unit-testable.
func buildRows(batch *zapreceiver.SpanBatch, org string, now time.Time) rows {
	resource := normalizedResource(batch)
	service := resource["service.name"]
	fp, labels := resourceIdentity(resource)
	fpStr := strconv.FormatUint(fp, 10)

	var out rows
	seenAttr := map[attrRow]struct{}{}
	seenKey := map[keyRow]struct{}{}
	seenBucket := map[int64]struct{}{}
	ops := map[[2]string]time.Time{}
	traces := map[string]*traceRow{}

	sight := func(fieldContext string, attrs map[string]string, key string, v any) string {
		dataType, stringValue, numberValue := classify(v)
		a := attrRow{org: org, tagKey: key, tagType: fieldContext, tagDataType: dataType, stringValue: stringValue, numberValue: numberValue}
		if _, ok := seenAttr[a]; !ok {
			seenAttr[a] = struct{}{}
			out.attrs = append(out.attrs, a)
		}
		k := keyRow{org: org, tagKey: key, tagType: fieldContext, dataType: dataType}
		if _, ok := seenKey[k]; !ok {
			seenKey[k] = struct{}{}
			out.keys = append(out.keys, k)
		}
		if attrs != nil {
			attrs[key] = stringValue
		}
		return stringValue
	}
	for k, v := range resource {
		sight(contextResource, nil, k, v)
	}

	for i := range batch.Spans {
		s := &batch.Spans[i]

		start := now
		if s.StartUnixNs > 0 {
			start = time.Unix(0, s.StartUnixNs).UTC()
		}
		end := start
		if s.EndUnixNs > 0 {
			end = time.Unix(0, s.EndUnixNs).UTC()
		}
		var duration uint64
		if d := end.Sub(start); d > 0 {
			duration = uint64(d.Nanoseconds())
		}

		attrs := make(map[string]string, len(s.Attributes)+1)
		for k, v := range s.Attributes {
			sight(contextTag, attrs, k, v)
		}
		if len(s.Events) > 0 {
			if b, err := json.Marshal(s.Events); err == nil {
				attrs[spanEventsKey] = string(b)
			}
		}
		if s.StatusMsg != "" {
			attrs[spanStatusMessageKey] = s.StatusMsg
		}

		id := s.SpanID
		if id == "" {
			id = contentID(s.TraceID, s.Name, strconv.FormatInt(s.StartUnixNs, 10), attrs)
		}

		out.spans = append(out.spans, spanRow{
			org:                 org,
			time:                start,
			id:                  id,
			name:                s.Name,
			kind:                normEnum(s.Kind, "span_kind_"),
			product:             resource["product"],
			sessionID:           pick(attrs, "session.id", "session_id"),
			distinctID:          pick(attrs, "distinct.id", "distinct_id"),
			url:                 pick(attrs, "url.full", "http.url", "url"),
			path:                pick(attrs, "url.path", "http.target", "http.route", "path"),
			attrs:               attrs,
			service:             service,
			traceID:             s.TraceID,
			spanID:              s.SpanID,
			parent:              s.ParentSpanID,
			duration:            duration,
			status:              normEnum(s.StatusCode, "status_code_"),
			resourceFingerprint: fpStr,
		})

		bucket := start.Unix()
		bucket -= bucket % resourceBucketSeconds
		if _, ok := seenBucket[bucket]; !ok {
			seenBucket[bucket] = struct{}{}
			out.resources = append(out.resources, resourceRow{org: org, bucket: bucket, fingerprint: fpStr, labels: labels})
		}

		opKey := [2]string{service, s.Name}
		if t, ok := ops[opKey]; !ok || start.After(t) {
			ops[opKey] = start
		}

		if t, ok := traces[s.TraceID]; ok {
			if start.Before(t.start) {
				t.start = start
			}
			if end.After(t.end) {
				t.end = end
			}
			t.numSpans++
		} else {
			traces[s.TraceID] = &traceRow{org: org, traceID: s.TraceID, start: start, end: end, numSpans: 1}
		}
	}

	for k, t := range ops {
		out.operations = append(out.operations, operationRow{org: org, service: k[0], name: k[1], time: t})
	}
	for _, t := range traces {
		out.traces = append(out.traces, *t)
	}
	return out
}

// normalizedResource copies the batch resource map and lifts AppName / Version
// into the OTLP service.* attributes when absent — the same lift
// datastoremetrics applies, so one sender groups identically across signals.
func normalizedResource(batch *zapreceiver.SpanBatch) map[string]string {
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
func orgOf(batch *zapreceiver.SpanBatch, def string) string {
	if o := batch.Resource["org"]; o != "" {
		return o
	}
	return def
}

// resourceIdentity is the resource → (fingerprint, labels) value: labels is
// the canonical JSON of the attribute map (encoding/json sorts map keys, so it
// is deterministic), and the fingerprint is FNV-1a 64 of exactly those bytes.
// event.span_resource stores the fingerprint as its DECIMAL string, the same
// contract pkg/datastorelogs uses, where the identical number is also the
// event.log resource column — one scheme, string and numeric forms
// interchangeable via toUInt64/toString.
func resourceIdentity(attrs map[string]string) (uint64, string) {
	b, _ := json.Marshal(attrs)
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64(), string(b)
}

// contentID is the deterministic fallback identity for a span without a
// span_id: idempotent re-ingest needs identity to be a function of content,
// never a random draw.
func contentID(parts ...any) string {
	h := fnv.New64a()
	for _, p := range parts {
		switch t := p.(type) {
		case string:
			h.Write([]byte(t))
		case map[string]string:
			b, _ := json.Marshal(t)
			h.Write(b)
		}
		h.Write([]byte{0xff})
	}
	return strconv.FormatUint(h.Sum64(), 16)
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

// normEnum lowercases a wire enum and strips the OTLP prefix so both spellings
// ("server" and "SPAN_KIND_SERVER") land as the envelope's lowercase form;
// the unset markers collapse to the column default.
func normEnum(v, prefix string) string {
	v = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v)), prefix)
	if v == "unspecified" || v == "unset" {
		return ""
	}
	return v
}
