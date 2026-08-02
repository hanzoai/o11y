-- pkg/telemetrylogs/schema.sql
--
-- THIS IS THE DEFINITION, NOT A MIGRATION.
--
-- The shapes of the tables whose NAMES are declared in tables.go, kept beside them:
-- one place declares what a table is called, the next declares what it holds. There is
-- no migration chain, no version suffix and no ALTER path — zero customers, so the
-- schema is created once, correctly. To change a table, change it here and recreate.
--
-- These are the SUPPORTING tables of the logs signal. The signal itself is event.log,
-- which is already applied and is not restated here. Under the one naming scheme a
-- table that supports a signal is PREFIXED by that signal, which is the only thing
-- keeping log_attribute and span_attribute apart now that both planes share the one
-- event database.
--
-- Column names and types are not invented: every one is fixed by a read site.
--   telemetryresourcefilter/field_mapper.go:13-17 declares log_resource's three
--   columns and their types outright (labels String, fingerprint String,
--   seen_at_ts_bucket_start Int64), and the SAME generic resolver builds the SQL for
--   both log_resource and span_resource — so the two tables must stay identical.
--   telemetrymetadata/metadata.go:468 states that the attribute-key and resource-key
--   tables share one schema and differ only in field context, which is why log_key and
--   log_resource_key are byte-identical below.
--
-- Two names in tables.go are deliberately absent here — log_path and log_promoted have
-- no reader. An absent table fails loudly; a table with guessed columns fails silently.

-- ---------------------------------------------------------------------------
-- log_attribute — attribute VALUE autocomplete.
--
-- ENGINE: ReplacingMergeTree(unix_milli). The role is a DISTINCT-over-identity
-- dimension: the writer re-emits a row every time it sees a value, and only the
-- newest sighting should survive. Plain MergeTree would accumulate one row per
-- sighting forever, which at ingest rates is unbounded. unix_milli is the version
-- column and therefore must NOT appear in the sort key — a version column inside the
-- key makes every re-sighting a new key and defeats the dedup entirely.
--
-- ORDER BY: org first, so a tenant's values are one contiguous run and tenant
-- isolation is a property of the layout rather than a filter. The remaining columns
-- are exactly the read's equality predicates in the order it applies them
-- (tag_key = ?, tag_type = ?, tag_data_type = ?), so the read is a prefix seek; the
-- trailing string_value/number_value complete the dedup identity — a value IS its
-- string and number pair.
--
-- number_value is Float64, NOT Nullable(Float64): metadata.go:1679 scans into a plain
-- `var numberValue float64`, and a NULL into a non-pointer float64 is a scan error.
-- The query's `number_value IS NOT NULL` degrades to a constant-true no-op, and the
-- real filter is the `numberValue != 0` guard at metadata.go:1689.
--
-- PARTITION BY month (never the tenant) so TTL can drop whole parts. A value seen in
-- two months exists in two partitions, since ReplacingMergeTree only dedups within a
-- partition — harmless, because every read of this table is a SELECT DISTINCT.
CREATE TABLE IF NOT EXISTS event.log_attribute
(
    `org`           LowCardinality(String),
    `tag_key`       LowCardinality(String) CODEC(ZSTD(1)),
    `tag_type`      LowCardinality(String) CODEC(ZSTD(1)),
    `tag_data_type` LowCardinality(String) CODEC(ZSTD(1)),
    `string_value`  String                CODEC(ZSTD(1)),
    `number_value`  Float64               CODEC(ZSTD(1)),
    `unix_milli`    Int64                 CODEC(DoubleDelta, ZSTD(1))
)
ENGINE = ReplacingMergeTree(unix_milli)
PARTITION BY toYYYYMM(toDateTime(unix_milli / 1000))
ORDER BY (org, tag_key, tag_type, tag_data_type, string_value, number_value)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- log_key — attribute KEY autocomplete: the distinct (name, datatype) pairs seen as
-- log attributes.
--
-- ENGINE: plain ReplacingMergeTree with NO version column. Every column is in the sort
-- key, so the row IS its identity — there is nothing to pick a newer version OF, and
-- dedup is exact.
--
-- NO PARTITION BY, and that is deliberate. This is a pure dimension bounded by the
-- instrumentation (thousands of rows, not millions), so it wants exactly ONE part set
-- that merges down to one part. Partitioning it by month would put the same
-- (org, name, datatype) in twelve partitions a year and break the dedup that is this
-- table's whole reason for existing — a monotonically growing table where a fixed-size
-- one belongs. The scale rule forbidding a partition key exists to stop unbounded part
-- sets; one part set is the strictest possible compliance with it. No TTL for the same
-- reason: an attribute key going stale is not a storage problem.
--
-- ORDER BY (org, name, datatype) matches the read exactly: WHERE is equality-or-ILIKE
-- on name then equality on datatype, and the GROUP BY is (name, datatype), so the
-- grouping runs over already-sorted input.
--
-- datatype is read as lower(datatype) because logs carry historical mixed-case data
-- (metadata.go:469). LowCardinality is dictionary encoding, not normalisation — it
-- preserves case exactly, so the lower() at read time still does its job.
CREATE TABLE IF NOT EXISTS event.log_key
(
    `org`      LowCardinality(String),
    `name`     LowCardinality(String) CODEC(ZSTD(1)),
    `datatype` LowCardinality(String) CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree
ORDER BY (org, name, datatype)
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- log_resource_key — resource KEY autocomplete, plus resource-key validation
-- (reader.go:1728 does SELECT name ... WHERE name IN (...)).
--
-- Byte-identical to log_key, because metadata.go:468 says outright that the attribute
-- and resource tables share one schema and only the table name and field context
-- differ. Same engine and same sort key for the same reasons; the field context lives
-- in the caller, not in the column set.
CREATE TABLE IF NOT EXISTS event.log_resource_key
(
    `org`      LowCardinality(String),
    `name`     LowCardinality(String) CODEC(ZSTD(1)),
    `datatype` LowCardinality(String) CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree
ORDER BY (org, name, datatype)
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- log_resource — resource-fingerprint index driving log resource filtering. The read
-- is one shape only:
--   SELECT fingerprint FROM event.log_resource
--    WHERE (simpleJSONExtractString(labels,'k') = ? AND labels LIKE ? AND labels LIKE ?)
--      AND seen_at_ts_bucket_start >= ? AND seen_at_ts_bucket_start <= ?
--    GROUP BY fingerprint
--
-- ENGINE: ReplacingMergeTree keyed on identity. fingerprint is the hash OF labels, so
-- two rows sharing (org, bucket, fingerprint) necessarily carry identical labels and
-- collapsing them is lossless — which is exactly the property that makes Replacing the
-- right engine here rather than a convenience.
--
-- ORDER BY (org, seen_at_ts_bucket_start, fingerprint): org first for tenant locality;
-- then the bucket, because the time range is the only RANGE predicate the read has and
-- a sort key prunes a range far better than any skip index can; fingerprint last, both
-- to complete the identity and to give the GROUP BY sorted input.
--
-- INDEX on labels, not lower(labels): the resolver emits `labels LIKE ?` verbatim
-- (statement_builder.go:103-105), and an index built on a lowered expression would
-- never be consulted by that predicate. ngram n=4 covers the emitted patterns, which
-- are attribute keys and values well over four characters.
--
-- NO bloom_filter on fingerprint despite it being a high-cardinality id column: the
-- read never filters ON fingerprint, it SELECTs and GROUPs BY it. An index there would
-- cost writes and be consulted by nothing.
--
-- seen_at_ts_bucket_start is unix SECONDS, not millis — statement_builder_test.go
-- pins start 1747947419s against bucket 1747945619, a 1800s lookback — so the
-- partition and TTL expressions take it as-is with no /1000.
CREATE TABLE IF NOT EXISTS event.log_resource
(
    `org`                     LowCardinality(String),
    `fingerprint`             String CODEC(ZSTD(1)),
    `labels`                  String CODEC(ZSTD(5)),
    `seen_at_ts_bucket_start` Int64  CODEC(DoubleDelta, ZSTD(1)),
    INDEX by_labels labels TYPE ngrambf_v1(4, 1024, 1, 0) GRANULARITY 1
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(toDateTime(seen_at_ts_bucket_start))
ORDER BY (org, seen_at_ts_bucket_start, fingerprint)
TTL toDateTime(seen_at_ts_bucket_start) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;
