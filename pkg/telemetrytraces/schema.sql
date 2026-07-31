-- event — traces support tables.
--
-- THIS IS THE DEFINITION, NOT A MIGRATION. There is no migration chain, no
-- version suffix and no ALTER path: `CREATE TABLE IF NOT EXISTS` is a no-op
-- against a store that already has the table, so this file is replayable and
-- is the single statement of what these tables ARE.
--
-- Names are the constants in tables.go — singular, unversioned, and prefixed by
-- the signal they support, which is what keeps span_attribute and log_attribute
-- apart now that both planes share one database. There are no Distributed
-- wrappers on this deployment, so the local and distributed name are one string.
--
-- Shape rules honoured by every table below:
--   * org is the FIRST sort-key column      -> rows are tenant-scopable by
--     construction, contiguous per tenant, one partition set for all tenants.
--   * the tenant NEVER appears in PARTITION BY -> N tenants x M months part-sets
--     is the one defect that collapses merges. PARTITION BY toYYYYMM(...) only.
--   * LowCardinality(String) for org/type/service-like columns.
--   * DoubleDelta+ZSTD on timestamps, ZSTD(1) on strings, T64 on counters.
--   * TTL ... SETTINGS ttl_only_drop_parts = 1 -> drop whole parts, never rewrite.
--   * 30-day TTL everywhere, matching event.span. A support table must not
--     outlive the spans it describes: an operation or attribute value whose
--     spans have aged out describes nothing and must stop autocompleting.
--   * No ReplicatedMergeTree — there is no ZooKeeper on this deployment.
--
-- Note on org: none of the five readers emits a `WHERE org = ?` predicate yet.
-- That is a code gap, not a schema gap. org sits first precisely so that adding
-- the predicate turns every one of these reads into a pure prefix seek without
-- touching the schema. Until then a read is one range scan per org, which is
-- cheap because org is LowCardinality.

-- span_attribute — attribute VALUE autocomplete.
--
-- ENGINE ReplacingMergeTree(unix_milli): a pure dimension keyed on its own
-- identity. The only read is `DISTINCT string_value, number_value` over the
-- identity tuple, and a Replacing merge has already collapsed exactly that
-- tuple, so DISTINCT is nearly free. unix_milli is the version, so the most
-- recently observed row wins.
-- ORDER BY: the read predicates are tag_key -> tag_type -> tag_data_type in that
-- order, so this is a pure prefix match; string_value and number_value complete
-- the dedup identity.
-- No skip index on string_value: the value search is ILIKE '%x%', and neither
-- ngrambf_v1 nor tokenbf_v1 is consulted for ILIKE (only like/startsWith/
-- endsWith/hasToken). An index there would be write amplification that never
-- gets read. The tag_key prefix already bounds the scan.
CREATE TABLE IF NOT EXISTS event.span_attribute
(
    `org`           LowCardinality(String),
    `tag_key`       String CODEC(ZSTD(1)),
    -- FieldContext.TagType(): 'tag' | 'resource' | 'scope' | 'spanfield'
    `tag_type`      LowCardinality(String),
    -- FieldDataType.TagDataType(): 'string' | 'float64' | 'bool'
    `tag_data_type` LowCardinality(String),
    `string_value`  String CODEC(ZSTD(1)),
    -- scanned into a plain Go float64 (metadata.go), so NOT Nullable.
    -- Gorilla XORs neighbours and these are sorted, so it compresses well.
    `number_value`  Float64 CODEC(Gorilla, ZSTD(1)),
    -- inferred: no SELECT reads it. It exists so PARTITION BY and TTL have a
    -- time column, and so the value plane can age out with the spans.
    `unix_milli`    Int64 CODEC(DoubleDelta, ZSTD(1))
)
ENGINE = ReplacingMergeTree(unix_milli)
PARTITION BY toYYYYMM(toDateTime(intDiv(unix_milli, 1000)))
ORDER BY (org, tag_key, tag_type, tag_data_type, string_value, number_value)
TTL toDateTime(intDiv(unix_milli, 1000)) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- span_key — attribute KEY autocomplete.
--
-- Columns are camelCase because the SQL is camelCase (metadata.go selects
-- `tagKey AS tag_key`). The alias is the wire name; these are the storage names.
--
-- ENGINE ReplacingMergeTree(unix_milli): dimension keyed on identity. The read
-- GROUP BYs (tagKey, tagType, dataType), which a Replacing merge has already
-- collapsed.
-- ORDER BY: matches the WHERE (equality or ILIKE on tagKey first) and the
-- GROUP BY exactly. isColumn is appended, not omitted — it is a real
-- discriminator and leaving it out of the sort key would let a Replacing merge
-- pick an arbitrary isColumn for a key, making the hard `isColumn = false`
-- filter flap between merges. Last position keeps the GROUP BY prefix intact.
CREATE TABLE IF NOT EXISTS event.span_key
(
    `org`        LowCardinality(String),
    `tagKey`     String CODEC(ZSTD(1)),
    `tagType`    LowCardinality(String),
    `dataType`   LowCardinality(String),
    -- hard-filtered `isColumn = false` on every read
    `isColumn`   Bool,
    -- time filtering is written but commented out at metadata.go:218-222
    -- (TODO(srikanthccv)). The column exists so that TODO is a one-line
    -- uncomment rather than a schema change, and so TTL has a column.
    `unix_milli` Int64 CODEC(DoubleDelta, ZSTD(1))
)
ENGINE = ReplacingMergeTree(unix_milli)
PARTITION BY toYYYYMM(toDateTime(intDiv(unix_milli, 1000)))
ORDER BY (org, tagKey, tagType, dataType, isColumn)
TTL toDateTime(intDiv(unix_milli, 1000)) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- span_resource — resource-fingerprint index for span filtering.
--
-- ENGINE ReplacingMergeTree (no version column): a fingerprint is re-emitted
-- every bucket, so re-ingest must be idempotent or the table grows without
-- bound. labels is functionally determined by fingerprint, so every candidate
-- row for an identity is equivalent and "last write wins" needs no version.
-- ORDER BY: seen_at_ts_bucket_start is the only range predicate every query
-- carries (>= and <=), so it leads; fingerprint completes the dedup identity.
-- INDEX on labels: every read — including the ILIKE ones — carries at least one
-- case-sensitive `labels LIKE '%"k":"v"%'` prefilter, which ngrambf_v1 IS
-- consulted for. This index is the reason those prefilters are emitted.
CREATE TABLE IF NOT EXISTS event.span_resource
(
    `org`                     LowCardinality(String),
    -- unix SECONDS (statement_builder.go: start / NsToSeconds), NOT millis
    `seen_at_ts_bucket_start` Int64 CODEC(DoubleDelta, ZSTD(1)),
    `fingerprint`             String CODEC(ZSTD(1)),
    -- MUST stay String. Read with simpleJSONExtractString(labels, 'k') and
    -- `labels LIKE '%"k":"v"%'`. A Map would break every read site.
    `labels`                  String CODEC(ZSTD(1)),
    INDEX by_labels labels TYPE ngrambf_v1(4, 1024, 1, 0) GRANULARITY 1
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(toDateTime(seen_at_ts_bucket_start))
ORDER BY (org, seen_at_ts_bucket_start, fingerprint)
TTL toDateTime(seen_at_ts_bucket_start) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- operation — one (service, operation) pair.
--
-- ENGINE ReplacingMergeTree(time): a pair is re-observed on every span and only
-- the latest sighting matters (the read is `max(time) as ts`).
-- ReplacingMergeTree keeps the row with the HIGHEST version, so max(time)
-- survives the merge, and `max(time)` over the un-merged partials is the same
-- answer — the aggregate is correct at every merge state.
-- ORDER BY: implservices filters `serviceName IN @services` and GROUP BYs
-- (name, serviceName); condition_builder does `DISTINCT name, serviceName`.
-- serviceName leads because it is the only equality predicate.
-- Caveat, accepted: Replacing dedups within a partition, so a long-lived pair
-- keeps one row per month rather than one row absolutely. That is the cost of
-- never putting anything but toYYYYMM in PARTITION BY, and 12 rows/pair/year is
-- nothing against the alternative.
CREATE TABLE IF NOT EXISTS event.operation
(
    `org`         LowCardinality(String),
    `serviceName` LowCardinality(String),
    `name`        String CODEC(ZSTD(1)),
    `time`        DateTime CODEC(DoubleDelta, ZSTD(1))
)
ENGINE = ReplacingMergeTree(time)
PARTITION BY toYYYYMM(time)
ORDER BY (org, serviceName, name)
TTL time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- trace — one whole trace's summary.
--
-- ENGINE AggregatingMergeTree over SimpleAggregateFunction columns.
-- A trace is summarised in partial rows (one per ingest batch that carried some
-- of its spans), and the readers fold them with sum(num_spans), min(start),
-- max(end). SummingMergeTree would be WRONG here: it sums the named column but
-- keeps an ARBITRARY value for every other numeric column, so start and end
-- would silently become "whichever partial row won the merge" and the waterfall
-- time range would be quietly incorrect. SimpleAggregateFunction stores the raw
-- value and merges it with the named function, so sum/min/max are correct
-- before, during and after a merge — which is the actual requirement.
-- ORDER BY: every read is an equality or IN on trace_id and nothing else.
-- No bloom_filter on trace_id: it is sort-key position 2 behind a
-- LowCardinality org, so the primary index already answers `trace_id IN (...)`
-- with a binary search per org. A skip index there is pure write amplification.
CREATE TABLE IF NOT EXISTS event.trace
(
    `org`       LowCardinality(String),
    `trace_id`  String CODEC(ZSTD(1)),
    -- DateTime64(9) matches event.span.time, so toUnixTimestamp64Nano is exact
    `start`     SimpleAggregateFunction(min, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1)),
    `end`       SimpleAggregateFunction(max, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1)),
    `num_spans` SimpleAggregateFunction(sum, UInt64) CODEC(T64, ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(`end`)
ORDER BY (org, trace_id)
TTL toDateTime(`end`) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;
