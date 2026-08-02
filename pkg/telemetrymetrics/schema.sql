-- event — metrics support tables.
--
-- THIS IS THE DEFINITION, NOT A MIGRATION. There is no migration chain, no
-- version suffix and no ALTER path: `CREATE TABLE IF NOT EXISTS` is a no-op
-- against a store that already has the table, so this file is replayable and
-- is the single statement of what these tables ARE.
--
-- Names are the constants in tables.go — singular, unversioned, and prefixed by
-- the signal they support, which is what keeps metric_attribute and
-- span_attribute apart now that both planes share one database. There are no
-- Distributed wrappers on this deployment, so the local and distributed name
-- are one string.
--
-- The already-deployed half of this signal (metric, metric_5m, metric_30m,
-- series, series_6h, series_1d, series_1w, descriptor) is NOT restated here.
-- These eight are the tables the exponential-histogram, reduced-metric and
-- metric-metadata read paths name and the store does not yet have.
--
-- Shape rules honoured by every table below:
--   * org is the FIRST sort-key column      -> rows are tenant-scopable by
--     construction, contiguous per tenant, one partition set for all tenants.
--   * the tenant NEVER appears in PARTITION BY -> N tenants x M months part-sets
--     is the one defect that collapses merges.
--   * LowCardinality(String) for org/env/temporality/metric_name-like columns.
--   * DoubleDelta+ZSTD on timestamps, Gorilla+ZSTD on float samples,
--     Delta(8)+ZSTD on fingerprints, T64+ZSTD on counters, ZSTD(1) on strings.
--   * bloom_filter skip index only where a read does an equality/IN lookup on a
--     high-cardinality NON-prefix column. A JOIN key gets none: ClickHouse
--     probes the build side of a hash join and never consults a skip index, so
--     an index there is write amplification that is never read.
--   * TTL ... SETTINGS ttl_only_drop_parts = 1 -> drop whole parts, never rewrite.
--   * No ReplicatedMergeTree — there is no ZooKeeper on this deployment.
--
-- Note on PARTITION BY toDate(...), not toYYYYMM(...). Two reasons, both
-- functional rather than stylistic:
--   1. metric_buffer and series_buffer have a 24h TTL. ttl_only_drop_parts = 1
--      drops a WHOLE part or nothing, so a monthly part cannot be dropped until
--      the whole month has expired — a "24h buffer" would hold up to 60 days.
--      Day-granular parts are what make a sub-day TTL mean anything.
--   2. The six sample/series tables must age out in lockstep with the deployed
--      metric/series family, which is toDate + 30d. metricsexplorer and
--      inframonitoring UNION a series* read with a series_reduced read; if the
--      reduced arm lingered a month longer it would list metrics whose raw data
--      is already gone.
-- The rule this protects — never put the tenant in PARTITION BY — is honoured:
-- org appears in no partition expression. Worst-case part-set here is ~30 daily
-- partitions per table for ONE partition set shared by all tenants, well inside
-- what merges want.
--
-- Note on org: no reader emits `WHERE org = ?` yet, and no writer supplies the
-- column (the collector INSERTs name their columns explicitly, so org takes its
-- DEFAULT). That is a code gap, not a schema gap. org sits first precisely so
-- that adding the predicate turns these reads into prefix seeks without touching
-- the schema, and so the day a second tenant writes, its rows are already
-- physically separated.

-- ---------------------------------------------------------------------------
-- metric_attribute — one attribute of one metric; the metric label dimension.
--
-- ENGINE AggregatingMergeTree: first/last-reported are min/max folds over an
-- unbounded restatement stream — every scrape re-asserts "this label existed at
-- time T". SimpleAggregateFunction(min|max) makes that fold happen at merge
-- time instead of at query time, which is the only reason the table stays one
-- row per (metric, attribute, value) instead of one row per scrape. This is
-- exactly the engine the already-deployed event.descriptor uses, and the
-- collector's metadata INSERT (o11ydatastoremetrics/exporter.go metadataSQLTmpl)
-- writes this exact column list.
--
-- ORDER BY drops `temporality`, which descriptor leads with. That is the
-- difference between the two tables and it is deliberate: this one is keyed on
-- the metric+attribute IDENTITY, so `(metric_name, attr_name, attr_string_value)
-- IN (...)` (metadata.go:2583) and the GROUP BY beneath it (:2588) are a
-- sort-key match, and count(*)>0 / groupUniqArray(attr_string_value) cannot
-- double-count the same label reported under two temporalities. metric_name
-- leading also serves every LIKE 'system.%' / 'k8s.%' namespace filter.
--
-- INDEX on attr_string_value: it is the high-cardinality column (host names, pod
-- names) and metadata.go:1857 / hosts.go:362 look it up by EQUALITY, but it is
-- the last sort-key column. bloom_filter is consulted for = and IN. The ILIKE
-- '%x%' search path is not indexable by any ClickHouse skip index and relies on
-- the metric_name prefix to bound the scan.
CREATE TABLE IF NOT EXISTS event.metric_attribute
(
    `org`                       LowCardinality(String) DEFAULT '',
    `temporality`               LowCardinality(String) CODEC(ZSTD(1)),
    `metric_name`               LowCardinality(String) CODEC(ZSTD(1)),
    `description`               String CODEC(ZSTD(1)),
    `unit`                      LowCardinality(String) CODEC(ZSTD(1)),
    `type`                      LowCardinality(String) CODEC(ZSTD(1)),
    `is_monotonic`              Bool CODEC(ZSTD(1)),
    `attr_name`                 LowCardinality(String) CODEC(ZSTD(1)),
    -- 'resource' | 'scope' | 'point' — the literals in the priority CASE at
    -- metadata.go:952-998.
    `attr_type`                 LowCardinality(String) CODEC(ZSTD(1)),
    `attr_datatype`             LowCardinality(String) CODEC(ZSTD(1)),
    `attr_string_value`         String CODEC(ZSTD(1)),
    `first_reported_unix_milli` SimpleAggregateFunction(min, UInt64) CODEC(ZSTD(1)),
    `last_reported_unix_milli`  SimpleAggregateFunction(max, UInt64) CODEC(ZSTD(1)),
    INDEX by_attr_value attr_string_value TYPE bloom_filter GRANULARITY 4
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(last_reported_unix_milli / 1000)
ORDER BY (org, metric_name, attr_name, attr_type, attr_datatype, attr_string_value)
TTL toDateTime(last_reported_unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- metric_buffer — raw ~24h sample buffer for reduced metrics.
--
-- Read INSTEAD of event.metric when a reduced-metric query lands inside the last
-- day (statement_builder.go:189-193). Column-for-column event.metric, because
-- AggregationColumnForSamplesTable puts MetricBufferTableName in the same case
-- arm as MetricTableName — the value column is `value` and the same
-- buildTemporalAggregationCTE joins it to the series CTE.
--
-- ENGINE ReplacingMergeTree(inserted_at_unix_milli), not MergeTree: the row's
-- identity IS its sort key — one series, one millisecond, one point — so a
-- retried write is a duplicate, and a duplicate silently inflates sum(value) and
-- count(value). Replacing folds it at merge time; the version column makes the
-- winner deterministic. (The deployed event.metric is plain MergeTree; that is
-- the older shape, not a target to copy.)
--
-- ORDER BY mirrors event.metric after org, because rate()/increase() run a
-- WINDOW PARTITION BY fingerprint and need physically fingerprint-ordered rows.
-- No skip index: fingerprint here is a JOIN key, never an IN lookup.
--
-- TTL 1 DAY is the whole point of the table — WhichSamplesTableToUse only
-- reaches for it when start >= now-1d.
CREATE TABLE IF NOT EXISTS event.metric_buffer
(
    `org`                    LowCardinality(String) DEFAULT '',
    `env`                    LowCardinality(String) DEFAULT 'default',
    `temporality`            LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`            LowCardinality(String),
    `fingerprint`            UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`             Int64 CODEC(DoubleDelta, ZSTD(1)),
    `value`                  Float64 CODEC(Gorilla, ZSTD(1)),
    `flags`                  UInt32 DEFAULT 0 CODEC(ZSTD(1)),
    `inserted_at_unix_milli` UInt64 DEFAULT 0 CODEC(T64, ZSTD(1))
)
ENGINE = ReplacingMergeTree(inserted_at_unix_milli)
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 1 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- series_buffer — series dimension for the buffer.
--
-- The fingerprint/labels catalog metric_buffer joins through. It carries BOTH
-- raw and reduced-catalog rows in one table, discriminated by `is_reduced` —
-- that column is the only thing that makes this table different from
-- event.series, and every raw read pins it false (statement_builder.go:515,
-- metadata.go:1935, :2277).
--
-- ENGINE ReplacingMergeTree(inserted_at_unix_milli): a series row is re-emitted
-- every write window with identical content, so re-ingest MUST be idempotent or
-- uniq(fingerprint) counts drift. Mirrors event.series.
--
-- ORDER BY mirrors event.series after org. `is_reduced` is deliberately NOT in
-- the key: it is a 2-value column that PREWHERE filters for free, and hoisting
-- it would split the key from event.series' for no seek benefit.
--
-- Columns are event.series' exactly (the collector's timeSeriesSQLTmpl list),
-- so ONE writer shape serves both. attrs/scope_attrs/resource_attrs are
-- Map(LowCardinality(String), String), never a JSON blob — a blob forces
-- decompress+parse per row on every query. `labels` is the JSON that
-- JSONExtractString(labels, '<key>') group-bys read and is the full-text column.
CREATE TABLE IF NOT EXISTS event.series_buffer
(
    `org`                    LowCardinality(String) DEFAULT '',
    `env`                    LowCardinality(String) DEFAULT 'default',
    `temporality`            LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`            LowCardinality(String),
    `description`            LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `unit`                   LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `type`                   LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `is_monotonic`           Bool DEFAULT false CODEC(ZSTD(1)),
    `fingerprint`            UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`             Int64 CODEC(DoubleDelta, ZSTD(1)),
    `labels`                 String CODEC(ZSTD(5)),
    `attrs`                  Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `scope_attrs`            Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `resource_attrs`         Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `__normalized`           Bool DEFAULT true CODEC(ZSTD(1)),
    -- the discriminator: false = the original series, true = the reduced catalog
    `is_reduced`             Bool DEFAULT false CODEC(ZSTD(1)),
    `inserted_at_unix_milli` UInt64 DEFAULT 0 CODEC(T64, ZSTD(1))
)
ENGINE = ReplacingMergeTree(inserted_at_unix_milli)
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 1 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- metric_reduced_last — 60s pre-aggregated samples for gauge-like reduced series.
--
-- The last/min/max fold. WhichReducedSamplesTableToUse picks this for everything
-- that is not a Sum or a Histogram.
--
-- ENGINE ReplacingMergeTree(computed_at): the reader ALREADY dedups with
-- argMax(<value>, computed_at) GROUP BY (reduced_fingerprint, unix_milli)
-- (statement_builder.go:340-352) because 60s buckets get recomputed. Declaring
-- that same identity as the sort key and computed_at as the version makes the
-- dedup a merge-time fold, so the query-time argMax collapses one row instead of
-- one row per recomputation. Plain MergeTree would keep every recomputation
-- forever and make the reduced table larger than the raw one it replaces.
--
-- The value columns are backticked at every read site (`sum_last`, `count_series`,
-- `min`, `max`) — `min` and `max` collide with function names and must stay
-- quoted. count_series is the avg denominator, not a time-additive count.
--
-- INDEX on reduced_fingerprint: helpers.go:379-386 and module.go:1124 look it up
-- with `reduced_fingerprint IN (<subquery>)`, and it is sort-key position 3 —
-- exactly the equality/IN-on-a-non-prefix-column case bloom_filter serves.
CREATE TABLE IF NOT EXISTS event.metric_reduced_last
(
    `org`                 LowCardinality(String) DEFAULT '',
    `metric_name`         LowCardinality(String),
    `reduced_fingerprint` UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`          Int64 CODEC(DoubleDelta, ZSTD(1)),
    -- the dedup tiebreak: argMax(<col>, computed_at) at every read site
    `computed_at`         DateTime64(3) DEFAULT now64(3) CODEC(DoubleDelta, ZSTD(1)),
    `sum_last`            Float64 CODEC(Gorilla, ZSTD(1)),
    `count_series`        UInt64 CODEC(T64, ZSTD(1)),
    `min`                 Float64 CODEC(Gorilla, ZSTD(1)),
    `max`                 Float64 CODEC(Gorilla, ZSTD(1)),
    INDEX by_reduced_fingerprint reduced_fingerprint TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, metric_name, reduced_fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- metric_reduced_sum — 60s pre-aggregated samples for counter/histogram reduced
-- series.
--
-- Identical role, identical dedup pattern and identical engine reasoning to
-- metric_reduced_last; WhichReducedSamplesTableToUse (tables.go:398-403) routes
-- SumType and HistogramType here.
--
-- It has NO `min`/`max` on purpose: ReducedValueColumn returns ok=false for
-- SpaceAggregationMin/Max on the sum table, so the reduced arm of the query is
-- omitted entirely rather than answered from a column that would be a
-- cross-series min of pre-summed values — which is not a min of anything real.
-- Adding those columns speculatively would make a wrong answer available.
CREATE TABLE IF NOT EXISTS event.metric_reduced_sum
(
    `org`                 LowCardinality(String) DEFAULT '',
    `metric_name`         LowCardinality(String),
    `reduced_fingerprint` UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`          Int64 CODEC(DoubleDelta, ZSTD(1)),
    `computed_at`         DateTime64(3) DEFAULT now64(3) CODEC(DoubleDelta, ZSTD(1)),
    `sum`                 Float64 CODEC(Gorilla, ZSTD(1)),
    `count_series`        UInt64 CODEC(T64, ZSTD(1)),
    INDEX by_reduced_fingerprint reduced_fingerprint TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, metric_name, reduced_fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- series_reduced — series dimension for reduced (label-dropped) metrics.
--
-- The fingerprint/labels catalog the metric_reduced_* tables join through, and
-- the answer to "which reduced metrics exist" (module.go:179).
--
-- ENGINE ReplacingMergeTree(inserted_at_unix_milli): a reduced series is
-- restated each window, and uniq(fingerprint) is read as a time-series COUNT
-- (module.go:1106, :1249), so duplicates would be visible as inflated
-- cardinality. Mirrors event.series so the UNION ALL arms in metricsexplorer and
-- inframonitoring line up.
--
-- ORDER BY mirrors event.series after org: every read filters metric_name plus a
-- unix_milli range and GROUP BYs fingerprint.
--
-- INDEX on fingerprint: hosts.go:90 and helpers.go:649 filter
-- `fingerprint IN (<reduced-fingerprint subquery>)` — an IN lookup on sort-key
-- position 5, which is what bloom_filter is for. (The buffer/sample tables get
-- no such index because there fingerprint is only ever a JOIN key.)
CREATE TABLE IF NOT EXISTS event.series_reduced
(
    `org`                    LowCardinality(String) DEFAULT '',
    `env`                    LowCardinality(String) DEFAULT 'default',
    `temporality`            LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`            LowCardinality(String),
    `description`            LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `unit`                   LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `type`                   LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `is_monotonic`           Bool DEFAULT false CODEC(ZSTD(1)),
    `fingerprint`            UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`             Int64 CODEC(DoubleDelta, ZSTD(1)),
    `labels`                 String CODEC(ZSTD(5)),
    `attrs`                  Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `scope_attrs`            Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `resource_attrs`         Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `__normalized`           Bool DEFAULT true CODEC(ZSTD(1)),
    `inserted_at_unix_milli` UInt64 DEFAULT 0 CODEC(T64, ZSTD(1)),
    INDEX by_fingerprint fingerprint TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(inserted_at_unix_milli)
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- histogram — exponential-histogram sketch points.
--
-- Sketches do not decompose into plain samples, so this IS the samples table for
-- ExpHistogramType (tables.go:229-232) and there is deliberately no 5m/30m
-- rollup of it. Column list is the collector's expHistSQLTmpl verbatim.
--
-- ENGINE ReplacingMergeTree(inserted_at_unix_milli), NOT AggregatingMergeTree.
-- An AggregateFunction column needs no engine support to be read: the merge
-- happens at QUERY time, in `quantilesDDMerge(0.01, <p>)(sketch)[1]`
-- (statement_builder.go:436-439). What AggregatingMergeTree would add is a
-- MERGE-time fold of rows sharing a sort key — and here that is actively wrong,
-- because the sort key is the identity of one data point (one series, one
-- millisecond). A retried write would have its sketch state merged INTO the
-- original, double-counting the distribution instead of deduplicating it, and
-- CountExpressionForSamplesTable's count(*) would still be off. Replacing keeps
-- exactly one row per point, which is what count(*) means here.
--
-- `sketch` is AggregateFunction(quantilesDD(0.01, 0.5, 0.75, 0.9, 0.95, 0.99),
-- UInt64) — the state the collector serialises. quantilesDDMerge with a
-- different parameter list merges it correctly (verified on this store).
CREATE TABLE IF NOT EXISTS event.histogram
(
    `org`                    LowCardinality(String) DEFAULT '',
    `env`                    LowCardinality(String) DEFAULT 'default',
    `temporality`            LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`            LowCardinality(String),
    `fingerprint`            UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`             Int64 CODEC(DoubleDelta, ZSTD(1)),
    `count`                  UInt64 CODEC(T64, ZSTD(1)),
    `sum`                    Float64 CODEC(Gorilla, ZSTD(1)),
    `min`                    Float64 CODEC(Gorilla, ZSTD(1)),
    `max`                    Float64 CODEC(Gorilla, ZSTD(1)),
    `sketch`                 AggregateFunction(quantilesDD(0.01, 0.5, 0.75, 0.9, 0.95, 0.99), UInt64) CODEC(ZSTD(1)),
    `flags`                  UInt32 DEFAULT 0 CODEC(ZSTD(1)),
    `inserted_at_unix_milli` UInt64 DEFAULT 0 CODEC(T64, ZSTD(1))
)
ENGINE = ReplacingMergeTree(inserted_at_unix_milli)
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- NOT CREATED: event.reduction.
--
-- telemetrymetrics.ReductionTableName is declared but has no datastore reader —
-- nothing in the repo ever puts it in a FROM. Label-reduction rules already have
-- exactly one home, and it is the relational store:
-- pkg/types/metricreductionruletypes/reduction_rule.go:66 declares
-- `bun:"table:metric_reduction_rule"`. A rule set is a small mutable
-- configuration read by primary key, which is what a row store is for and what a
-- columnar store is worst at. Creating a second home in event would be two
-- sources of truth for one fact. An absent table fails loudly; a duplicated one
-- fails silently.
