package telemetrymetadata

import "testing"

// The DDL production actually serves for event.span, verbatim from
// SHOW CREATE TABLE (real newlines restored).
const spanDDL = "CREATE TABLE event.span\n(\n    `org` LowCardinality(String),\n    `time` DateTime64(9) CODEC(DoubleDelta, ZSTD(1)),\n    `ingested_at` DateTime64(3) DEFAULT now64(3) CODEC(DoubleDelta, ZSTD(1)),\n    `id` String CODEC(ZSTD(1)),\n    `name` String CODEC(ZSTD(1)),\n    `kind` LowCardinality(String),\n    `product` LowCardinality(String),\n    `session_id` String CODEC(ZSTD(1)),\n    `distinct_id` String CODEC(ZSTD(1)),\n    `anonymous_id` String CODEC(ZSTD(1)),\n    `person_id` String CODEC(ZSTD(1)),\n    `url` String CODEC(ZSTD(1)),\n    `path` String CODEC(ZSTD(1)),\n    `attributes` Map(LowCardinality(String), String) CODEC(ZSTD(1)),\n    `el` Tuple(\n        label String,\n        role LowCardinality(String),\n        testid String,\n        name String,\n        component LowCardinality(String),\n        path Array(String)) CODEC(ZSTD(1)),\n    `host` LowCardinality(String) DEFAULT domain(url),\n    `service` LowCardinality(String),\n    `trace_id` String CODEC(ZSTD(1)),\n    `span_id` String CODEC(ZSTD(1)),\n    `parent` String CODEC(ZSTD(1)),\n    `duration` UInt64 CODEC(T64, ZSTD(1)),\n    `status` LowCardinality(String),\n    `resource_fingerprint` String CODEC(ZSTD(1)),\n    `ts_bucket_start` UInt64 MATERIALIZED intDiv(toUnixTimestamp(time), 1800) * 1800 CODEC(DoubleDelta, ZSTD(1)),\n    INDEX by_time time TYPE minmax GRANULARITY 1,\n    INDEX by_service service TYPE set(0) GRANULARITY 4,\n    INDEX by_duration duration TYPE minmax GRANULARITY 1\n)\nENGINE = ReplacingMergeTree(ingested_at)\nPARTITION BY toDate(ingested_at)\nORDER BY (org, trace_id, time, id)\nTTL toDateTime(ingested_at) + toIntervalDay(30)\nSETTINGS index_granularity = 8192, ttl_only_drop_parts = 1"

func TestReproSpanDDL(t *testing.T) {
	_, err := ExtractFieldKeysFromTblStatement(spanDDL)
	if err != nil {
		t.Fatalf("FULL span DDL failed: %v", err)
	}
	t.Log("full span DDL parsed OK")
}

// Bisect: the same DDL with the named-Tuple column removed.
func TestReproSpanDDLNoTuple(t *testing.T) {
	ddl := "CREATE TABLE event.span\n(\n    `org` LowCardinality(String),\n    `attributes` Map(LowCardinality(String), String) CODEC(ZSTD(1)),\n    `host` LowCardinality(String) DEFAULT domain(url)\n)\nENGINE = ReplacingMergeTree(ingested_at)\nORDER BY (org)"
	_, err := ExtractFieldKeysFromTblStatement(ddl)
	if err != nil {
		t.Fatalf("no-tuple DDL failed: %v", err)
	}
	t.Log("no-tuple DDL parsed OK")
}

// Bisect: just the named Tuple column.
func TestReproTupleOnly(t *testing.T) {
	ddl := "CREATE TABLE event.span\n(\n    `org` LowCardinality(String),\n    `el` Tuple(\n        label String,\n        role LowCardinality(String),\n        path Array(String)) CODEC(ZSTD(1))\n)\nENGINE = ReplacingMergeTree(ingested_at)\nORDER BY (org)"
	_, err := ExtractFieldKeysFromTblStatement(ddl)
	if err != nil {
		t.Fatalf("tuple-only DDL failed: %v", err)
	}
	t.Log("tuple-only DDL parsed OK")
}
