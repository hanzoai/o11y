// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package datastoretraces

import (
	"encoding/json"
	"hash/fnv"
	"strconv"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/zapreceiver"
)

var testNow = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

func testBatch() *zapreceiver.SpanBatch {
	return &zapreceiver.SpanBatch{
		AppName:  "gateway",
		Version:  "1.2.3",
		Resource: map[string]string{"deployment.environment": "prod", "product": "ai"},
		Spans: []zapreceiver.Span{
			{
				TraceID: "t1", SpanID: "s1", Name: "GET /v1/x", Kind: "SPAN_KIND_SERVER",
				StartUnixNs: 1_700_000_000_000_000_000, EndUnixNs: 1_700_000_000_250_000_000,
				Attributes: map[string]any{"http.url": "https://api.hanzo.ai/v1/x", "url.path": "/v1/x", "retries": float64(2), "cache": true},
				StatusCode: "STATUS_CODE_OK",
			},
			{
				TraceID: "t1", SpanID: "s2", ParentSpanID: "s1", Name: "db.query", Kind: "client",
				StartUnixNs: 1_700_000_000_050_000_000, EndUnixNs: 1_700_000_000_100_000_000,
				Attributes: map[string]any{"retries": float64(2)},
				StatusCode: "error",
				Events:     []zapreceiver.SpanEvent{{Name: "retry", TimeUnixNs: 1}},
			},
		},
	}
}

func spanByID(t *testing.T, r rows, id string) spanRow {
	t.Helper()
	for _, s := range r.spans {
		if s.spanID == id {
			return s
		}
	}
	t.Fatalf("span %q not built", id)
	return spanRow{}
}

func TestBuildRowsEnvelope(t *testing.T) {
	r := buildRows(testBatch(), "hanzo", testNow)
	if len(r.spans) != 2 {
		t.Fatalf("spans: got %d want 2", len(r.spans))
	}

	s1 := spanByID(t, r, "s1")
	if s1.org != "hanzo" || s1.id != "s1" || s1.traceID != "t1" || s1.parent != "" {
		t.Fatalf("identity wrong: %+v", s1)
	}
	if s1.kind != "server" || s1.status != "ok" {
		t.Fatalf("enum normalization wrong: kind=%q status=%q", s1.kind, s1.status)
	}
	if s1.service != "gateway" || s1.product != "ai" {
		t.Fatalf("service/product lift wrong: %+v", s1)
	}
	if s1.url != "https://api.hanzo.ai/v1/x" || s1.path != "/v1/x" {
		t.Fatalf("url/path promotion wrong: %+v", s1)
	}
	if s1.duration != 250_000_000 {
		t.Fatalf("duration: got %d", s1.duration)
	}
	if s1.time != time.Unix(0, 1_700_000_000_000_000_000).UTC() {
		t.Fatalf("time wrong: %v", s1.time)
	}
	if s1.attrs["retries"] != "2" || s1.attrs["cache"] != "true" {
		t.Fatalf("attributes must carry the JSON scalar form: %+v", s1.attrs)
	}

	s2 := spanByID(t, r, "s2")
	if s2.status != "error" || s2.kind != "client" || s2.parent != "s1" {
		t.Fatalf("s2 wrong: %+v", s2)
	}
	var events []zapreceiver.SpanEvent
	if err := json.Unmarshal([]byte(s2.attrs[spanEventsKey]), &events); err != nil || len(events) != 1 || events[0].Name != "retry" {
		t.Fatalf("span events must fold into attributes[%q]: %q err=%v", spanEventsKey, s2.attrs[spanEventsKey], err)
	}
}

func TestBuildRowsDeterministicIdentity(t *testing.T) {
	a := buildRows(testBatch(), "hanzo", testNow)
	b := buildRows(testBatch(), "hanzo", testNow)
	for i := range a.spans {
		if a.spans[i].id != b.spans[i].id || !a.spans[i].time.Equal(b.spans[i].time) {
			t.Fatalf("identity must be deterministic: %+v vs %+v", a.spans[i], b.spans[i])
		}
	}

	batch := testBatch()
	batch.Spans[0].SpanID = "" // sender omitted the id
	c := buildRows(batch, "hanzo", testNow)
	d := buildRows(batch, "hanzo", testNow)
	if c.spans[0].id == "" || c.spans[0].id != d.spans[0].id {
		t.Fatalf("content id must be deterministic and non-empty: %q vs %q", c.spans[0].id, d.spans[0].id)
	}
	if c.spans[0].id == c.spans[1].id {
		t.Fatalf("distinct spans must not share a content id")
	}
}

func TestBuildRowsSupportTables(t *testing.T) {
	r := buildRows(testBatch(), "hanzo", testNow)

	// retries=2 appears on both spans -> ONE attribute row, ONE key row.
	attrs := map[attrRow]struct{}{}
	for _, a := range r.attrs {
		if _, dup := attrs[a]; dup {
			t.Fatalf("attribute rows must dedup within the batch: %+v", a)
		}
		attrs[a] = struct{}{}
	}
	if _, ok := attrs[attrRow{org: "hanzo", tagKey: "retries", tagType: contextTag, tagDataType: dataTypeFloat64, stringValue: "2", numberValue: 2}]; !ok {
		t.Fatalf("missing float attribute row: %+v", r.attrs)
	}
	if _, ok := attrs[attrRow{org: "hanzo", tagKey: "deployment.environment", tagType: contextResource, tagDataType: dataTypeString, stringValue: "prod"}]; !ok {
		t.Fatalf("missing resource attribute row: %+v", r.attrs)
	}

	keys := map[keyRow]struct{}{}
	for _, k := range r.keys {
		keys[k] = struct{}{}
	}
	for _, want := range []keyRow{
		{org: "hanzo", tagKey: "cache", tagType: contextTag, dataType: dataTypeBool},
		{org: "hanzo", tagKey: "service.name", tagType: contextResource, dataType: dataTypeString},
	} {
		if _, ok := keys[want]; !ok {
			t.Fatalf("missing key row %+v in %+v", want, r.keys)
		}
	}

	// Both spans start inside one 1800s bucket -> ONE resource row, and the
	// fingerprint is the decimal FNV-1a of the canonical labels JSON.
	if len(r.resources) != 1 {
		t.Fatalf("resources: got %d want 1: %+v", len(r.resources), r.resources)
	}
	res := r.resources[0]
	if res.bucket%resourceBucketSeconds != 0 || res.bucket > 1_700_000_000 || res.bucket+resourceBucketSeconds <= 1_700_000_000 {
		t.Fatalf("bucket must floor the span second to %ds: %d", resourceBucketSeconds, res.bucket)
	}
	h := fnv.New64a()
	h.Write([]byte(res.labels))
	if res.fingerprint != strconv.FormatUint(h.Sum64(), 10) {
		t.Fatalf("fingerprint %q must be the decimal FNV-1a of labels %q", res.fingerprint, res.labels)
	}
	// Every span row carries the SAME fingerprint string, so the read plane's
	// __resource_filter join (span.resource_fingerprint IN span_resource.fingerprint)
	// matches rows written here.
	for _, s := range r.spans {
		if s.resourceFingerprint != res.fingerprint {
			t.Fatalf("span resource_fingerprint %q must equal span_resource fingerprint %q", s.resourceFingerprint, res.fingerprint)
		}
	}
	var labels map[string]string
	if err := json.Unmarshal([]byte(res.labels), &labels); err != nil || labels["service.name"] != "gateway" || labels["service.version"] != "1.2.3" {
		t.Fatalf("labels must carry the normalized resource: %q err=%v", res.labels, err)
	}

	// Operations: one per (service, name), stamped with the latest sighting.
	if len(r.operations) != 2 {
		t.Fatalf("operations: got %d want 2: %+v", len(r.operations), r.operations)
	}
	for _, op := range r.operations {
		if op.service != "gateway" {
			t.Fatalf("operation service wrong: %+v", op)
		}
	}

	// Trace partial: 2 spans folded to min start / max end / count.
	if len(r.traces) != 1 {
		t.Fatalf("traces: got %d want 1", len(r.traces))
	}
	tr := r.traces[0]
	if tr.numSpans != 2 ||
		!tr.start.Equal(time.Unix(0, 1_700_000_000_000_000_000).UTC()) ||
		!tr.end.Equal(time.Unix(0, 1_700_000_000_250_000_000).UTC()) {
		t.Fatalf("trace fold wrong: %+v", tr)
	}
}

func TestOrgAndZeroTimeFallbacks(t *testing.T) {
	batch := testBatch()
	batch.Resource["org"] = "zoo"
	r := buildRows(batch, orgOf(batch, "hanzo"), testNow)
	if r.spans[0].org != "zoo" {
		t.Fatalf("batch resource org must win: %+v", r.spans[0])
	}

	batch = testBatch()
	batch.Spans[0].StartUnixNs, batch.Spans[0].EndUnixNs = 0, 0
	r = buildRows(batch, "hanzo", testNow)
	s := spanByID(t, r, "s1")
	if !s.time.Equal(testNow) || s.duration != 0 {
		t.Fatalf("zero start must fall back to the injected clock: %+v", s)
	}
}

func TestNormEnum(t *testing.T) {
	for in, want := range map[string]string{
		"SPAN_KIND_SERVER": "server", "client": "client", "SPAN_KIND_UNSPECIFIED": "", "unspecified": "", "": "",
	} {
		if got := normEnum(in, "span_kind_"); got != want {
			t.Fatalf("normEnum(%q) = %q want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"STATUS_CODE_OK": "ok", "unset": "", "error": "error"} {
		if got := normEnum(in, "status_code_"); got != want {
			t.Fatalf("normEnum(%q) = %q want %q", in, got, want)
		}
	}
}
