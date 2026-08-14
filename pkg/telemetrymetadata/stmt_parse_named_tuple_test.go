package telemetrymetadata

import "testing"

// TestNamedTupleSchemaReadsBack pins that the plane's OWN log table can be read
// back by the metadata step every logs query runs first.
//
// event.log and event.span each carry `el`, a NAMED tuple. The parser read only
// the unnamed form, so SHOW CREATE TABLE stopped at the first element's type and
// getLogsKeys answered ErrFailedToGetLogsKeys — which surfaced as HTTP 500
// "failed to get logs keys" on EVERY logs query, against a table holding 122M
// rows a day. A schema this service defines has to be one this service can read.
func TestNamedTupleSchemaReadsBack(t *testing.T) {
	const ddl = "CREATE TABLE event.log (" +
		"`org` LowCardinality(String), " +
		"`time` DateTime64(9) CODEC(DoubleDelta, ZSTD(1)), " +
		"`attributes` Map(LowCardinality(String), String) CODEC(ZSTD(1)), " +
		"`el` Tuple(label String, role LowCardinality(String), path Array(String)) CODEC(ZSTD(1)), " +
		"`service` LowCardinality(String), " +
		"`ts_bucket_start` UInt64 MATERIALIZED intDiv(toUnixTimestamp(time), 1800) * 1800" +
		") ENGINE = ReplacingMergeTree ORDER BY (org, service, time)"

	if _, err := ExtractFieldKeysFromTblStatement(ddl); err != nil {
		t.Fatalf("the plane's own log schema does not read back: %v", err)
	}
}
