// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package datastorelogs

import (
	"encoding/json"
	"hash/fnv"
	"strconv"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/zaplogreceiver"
)

var testNow = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

func testBatch() *zaplogreceiver.LogBatch {
	return &zaplogreceiver.LogBatch{
		AppName:  "gateway",
		Version:  "1.2.3",
		Resource: map[string]string{"deployment.environment": "prod", "product": "ai"},
		Records: []zaplogreceiver.LogRecord{
			{
				TimeUnixNs: 1_700_000_000_000_000_000, Severity: 9, SeverityText: "info",
				Body:       "request served",
				Attributes: map[string]any{"http.url": "https://api.hanzo.ai/v1/x", "url.path": "/v1/x", "latency_ms": float64(12.5), "cached": false},
				TraceID:    "t1", SpanID: "s1",
			},
			{
				TimeUnixNs: 1_700_000_000_100_000_000, Severity: 17, SeverityText: "error",
				Body: "boom", EventName: "panic",
			},
		},
	}
}

func TestBuildRowsEnvelope(t *testing.T) {
	r := buildRows(testBatch(), "hanzo", testNow)
	if len(r.logs) != 2 {
		t.Fatalf("logs: got %d want 2", len(r.logs))
	}

	l := r.logs[0]
	if l.org != "hanzo" || l.service != "gateway" || l.product != "ai" {
		t.Fatalf("org/service/product wrong: %+v", l)
	}
	if l.severityText != "info" || l.severityNumber != 9 || l.body != "request served" {
		t.Fatalf("severity/body wrong: %+v", l)
	}
	if l.traceID != "t1" || l.spanID != "s1" {
		t.Fatalf("trace linkage wrong: %+v", l)
	}
	if l.url != "https://api.hanzo.ai/v1/x" || l.path != "/v1/x" {
		t.Fatalf("url/path promotion wrong: %+v", l)
	}
	if l.attrs["latency_ms"] != "12.5" || l.attrs["cached"] != "false" {
		t.Fatalf("attributes must carry the JSON scalar form: %+v", l.attrs)
	}
	if !l.time.Equal(time.Unix(0, 1_700_000_000_000_000_000).UTC()) {
		t.Fatalf("time wrong: %v", l.time)
	}
	if r.logs[1].name != "panic" {
		t.Fatalf("EventName must land as name: %+v", r.logs[1])
	}

	// The envelope's resource number and the support table's decimal string
	// are the SAME value.
	if len(r.resources) != 1 {
		t.Fatalf("resources: got %d want 1", len(r.resources))
	}
	if strconv.FormatUint(l.resource, 10) != r.resources[0].fingerprint {
		t.Fatalf("event.log resource %d must equal log_resource fingerprint %q", l.resource, r.resources[0].fingerprint)
	}
	// Every log row carries the SAME fingerprint string, so the read plane's
	// __resource_filter join (log.resource_fingerprint IN log_resource.fingerprint)
	// matches rows written here.
	for _, row := range r.logs {
		if row.resourceFingerprint != r.resources[0].fingerprint {
			t.Fatalf("log resource_fingerprint %q must equal log_resource fingerprint %q", row.resourceFingerprint, r.resources[0].fingerprint)
		}
	}
	h := fnv.New64a()
	h.Write([]byte(r.resources[0].labels))
	if h.Sum64() != l.resource {
		t.Fatalf("fingerprint must be FNV-1a of the canonical labels JSON")
	}
	var labels map[string]string
	if err := json.Unmarshal([]byte(r.resources[0].labels), &labels); err != nil || labels["service.name"] != "gateway" {
		t.Fatalf("labels must carry the normalized resource: %q err=%v", r.resources[0].labels, err)
	}
	if r.resources[0].bucket%resourceBucketSeconds != 0 {
		t.Fatalf("bucket must floor to %ds: %d", resourceBucketSeconds, r.resources[0].bucket)
	}
}

func TestRecordIDDeterministicAndDistinct(t *testing.T) {
	a := buildRows(testBatch(), "hanzo", testNow)
	b := buildRows(testBatch(), "hanzo", testNow)
	for i := range a.logs {
		if a.logs[i].id == "" || a.logs[i].id != b.logs[i].id {
			t.Fatalf("record id must be a deterministic content hash: %q vs %q", a.logs[i].id, b.logs[i].id)
		}
	}
	if a.logs[0].id == a.logs[1].id {
		t.Fatalf("distinct records must not share an id")
	}

	batch := testBatch()
	batch.Records[0].Body = "request served DIFFERENTLY"
	c := buildRows(batch, "hanzo", testNow)
	if c.logs[0].id == a.logs[0].id {
		t.Fatalf("changing the body must change the id")
	}
}

func TestSupportTables(t *testing.T) {
	r := buildRows(testBatch(), "hanzo", testNow)

	keys := map[keyRow]struct{}{}
	for _, k := range r.keys {
		if _, dup := keys[k]; dup {
			t.Fatalf("log_key rows must dedup: %+v", k)
		}
		keys[k] = struct{}{}
	}
	for _, want := range []keyRow{
		{org: "hanzo", name: "latency_ms", datatype: dataTypeFloat64},
		{org: "hanzo", name: "cached", datatype: dataTypeBool},
		{org: "hanzo", name: "http.url", datatype: dataTypeString},
	} {
		if _, ok := keys[want]; !ok {
			t.Fatalf("missing log_key %+v in %+v", want, r.keys)
		}
	}

	resourceKeys := map[keyRow]struct{}{}
	for _, k := range r.resourceKeys {
		resourceKeys[k] = struct{}{}
	}
	for _, want := range []string{"deployment.environment", "product", "service.name", "service.version"} {
		if _, ok := resourceKeys[keyRow{org: "hanzo", name: want, datatype: dataTypeString}]; !ok {
			t.Fatalf("missing log_resource_key %q in %+v", want, r.resourceKeys)
		}
	}

	attrs := map[attrRow]struct{}{}
	for _, a := range r.attrs {
		attrs[a] = struct{}{}
	}
	if _, ok := attrs[attrRow{org: "hanzo", tagKey: "latency_ms", tagType: contextTag, tagDataType: dataTypeFloat64, stringValue: "12.5", numberValue: 12.5}]; !ok {
		t.Fatalf("missing log_attribute value row: %+v", r.attrs)
	}
	if _, ok := attrs[attrRow{org: "hanzo", tagKey: "deployment.environment", tagType: contextResource, tagDataType: dataTypeString, stringValue: "prod"}]; !ok {
		t.Fatalf("missing resource-context value row: %+v", r.attrs)
	}
}

func TestFallbacksAndClamps(t *testing.T) {
	batch := testBatch()
	batch.Resource["org"] = "zoo"
	r := buildRows(batch, orgOf(batch, "hanzo"), testNow)
	if r.logs[0].org != "zoo" {
		t.Fatalf("batch resource org must win: %+v", r.logs[0])
	}

	batch = testBatch()
	batch.Records[0].TimeUnixNs = 0
	batch.Records[0].ObservedTimeUnixNs = 42
	batch.Records[1].TimeUnixNs = 0
	r = buildRows(batch, "hanzo", testNow)
	if !r.logs[0].time.Equal(time.Unix(0, 42).UTC()) {
		t.Fatalf("zero time must fall back to observed time: %v", r.logs[0].time)
	}
	if !r.logs[1].time.Equal(testNow) {
		t.Fatalf("zero everything must fall back to the injected clock: %v", r.logs[1].time)
	}

	if clampSeverity(-1) != 0 || clampSeverity(300) != 255 || clampSeverity(21) != 21 {
		t.Fatalf("severity clamp broken")
	}
}
