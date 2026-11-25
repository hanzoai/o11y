-- 0005_event_metric_rollups.sql — attach the live ClickHouse 26.2 materialized
-- views that populate event.metric_5m, event.metric_30m, event.series_6h,
-- event.series_1d, and event.series_1w from event.metric and event.series.
--
-- ROOT CAUSE (verified on live datastore-0, ClickHouse 26.2.3.2):
--   * event.metric has ~1.59B rows (3.53 GiB) and event.series has ~8.71M rows
--     (81.28 MiB), both with dependencies_table = [].
--   * event.metric_5m, event.metric_30m, event.series_6h, event.series_1d, and
--     event.series_1w all exist with the live (pre-0003) schema and 0 rows
--     because ZERO materialized views exist in the `event` database for them.
--   * The legacy materialized views (samples_v4_agg_5m_mv, samples_v4_agg_30m_mv,
--     time_series_v4_{6hrs,1day,1week}_mv) still live in `o11y_metrics`, which
--     no longer receives ingestion.
--   * Since WriteMetricsMany batching landed (pkg/datastoremetrics/writer.go),
--     event.metric insert frequency dropped from ~518/min to ~28/min
--     (~1,678 INSERTs/hour), well inside ClickHouse's merge budget.
--
-- FORWARD COMPATIBILITY WITH 0003_event_sample.sql:
--   View names match 0003_event_sample.sql step 1 verbatim (`event.fill_metric_5m`,
--   `event.fill_metric_30m`, `event.fill_series_6h`, `event.fill_series_1d`,
--   `event.fill_series_1w`), so when 0003 later runs its `DROP VIEW IF EXISTS`
--   step before promoting org-first tables, every view created here is cleanly
--   unwired without leaving an orphan.
--
-- IDEMPOTENT & SAFE:
--   Uses `CREATE MATERIALIZED VIEW IF NOT EXISTS` and refuses to run if
--   0003_event_sample.sql has already retired `event.metric` or added `org`.

-- =====================================================================
-- 0) PRE-FLIGHT GATE — verify live pre-0003 event.* metric tables exist
-- =====================================================================

SELECT throwIf(
    (SELECT count() FROM system.tables
      WHERE database = 'event'
        AND name IN ('metric', 'metric_5m', 'metric_30m',
                     'series', 'series_6h', 'series_1d', 'series_1w')) != 7,
    'REFUSING: one or more live event metric/series rollup tables are missing (was 0003_event_sample.sql already applied?).');

SELECT throwIf(
    (SELECT count() FROM system.columns
      WHERE database = 'event'
        AND table IN ('metric', 'metric_5m', 'metric_30m',
                      'series', 'series_6h', 'series_1d', 'series_1w')
        AND name = 'org') > 0,
    'REFUSING: event metric/series tables already carry an org column; use the org-first views from 0003_event_sample.sql instead.');

-- =====================================================================
-- 1) METRIC SAMPLE ROLLUP VIEWS (event.metric -> metric_5m -> metric_30m)
-- =====================================================================
-- Every SELECT column name and type matches live ClickHouse 26.2 target
-- schemas (`SimpleAggregateFunction` columns accept raw/aggregated values
-- directly; `intDiv(unix_milli, N) * N` emits exact Int64).
-- `WHERE bitAnd(flags, 1) = 0` skips Prometheus stale markers, matching
-- both legacy samples_v4_agg_5m_mv and 0003_event_sample.sql.

CREATE MATERIALIZED VIEW IF NOT EXISTS event.fill_metric_5m
TO event.metric_5m
AS
SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    intDiv(unix_milli, 300000) * 300000 AS unix_milli,
    anyLast(value) AS last,
    min(value)     AS min,
    max(value)     AS max,
    sum(value)     AS sum,
    count(*)       AS count
FROM event.metric
WHERE bitAnd(flags, 1) = 0
GROUP BY env, temporality, metric_name, fingerprint, unix_milli;

CREATE MATERIALIZED VIEW IF NOT EXISTS event.fill_metric_30m
TO event.metric_30m
AS
SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    intDiv(unix_milli, 1800000) * 1800000 AS unix_milli,
    anyLast(last) AS last,
    min(min)      AS min,
    max(max)      AS max,
    sum(sum)      AS sum,
    sum(count)    AS count
FROM event.metric_5m
GROUP BY env, temporality, metric_name, fingerprint, unix_milli;

-- =====================================================================
-- 2) SERIES DIMENSION ROLLUP VIEWS (event.series -> 6h -> 1d -> 1w)
-- =====================================================================
-- telemetrymetrics.WhichTSTableToUse routes >= 6h to event.series_6h,
-- >= 1d to event.series_1d, and >= 1w to event.series_1w. Without these
-- views, multi-day dashboard and PromQL queries read empty tables.

CREATE MATERIALIZED VIEW IF NOT EXISTS event.fill_series_6h
TO event.series_6h
AS
SELECT
    env,
    temporality,
    metric_name,
    description,
    unit,
    type,
    is_monotonic,
    fingerprint,
    intDiv(unix_milli, 21600000) * 21600000 AS unix_milli,
    labels,
    attrs,
    scope_attrs,
    resource_attrs,
    __normalized
FROM event.series;

CREATE MATERIALIZED VIEW IF NOT EXISTS event.fill_series_1d
TO event.series_1d
AS
SELECT
    env,
    temporality,
    metric_name,
    description,
    unit,
    type,
    is_monotonic,
    fingerprint,
    intDiv(unix_milli, 86400000) * 86400000 AS unix_milli,
    labels,
    attrs,
    scope_attrs,
    resource_attrs,
    __normalized
FROM event.series_6h;

CREATE MATERIALIZED VIEW IF NOT EXISTS event.fill_series_1w
TO event.series_1w
AS
SELECT
    env,
    temporality,
    metric_name,
    description,
    unit,
    type,
    is_monotonic,
    fingerprint,
    intDiv(unix_milli, 604800000) * 604800000 AS unix_milli,
    labels,
    attrs,
    scope_attrs,
    resource_attrs,
    __normalized
FROM event.series_1d;

-- =====================================================================
-- 3) POST-FLIGHT VERIFY — prove the dependency chain is wired
-- =====================================================================

SELECT throwIf(
    (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'metric'    AND has(dependencies_table, 'fill_metric_5m'))  != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'metric_5m' AND has(dependencies_table, 'fill_metric_30m')) != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'series'    AND has(dependencies_table, 'fill_series_6h'))  != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'series_6h' AND has(dependencies_table, 'fill_series_1d'))  != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'series_1d' AND has(dependencies_table, 'fill_series_1w'))  != 1,
    'ROLLUP CHAIN NOT WIRED: one or more fill_* materialized views failed to attach in system.tables.dependencies_table.') AS ok;

-- =====================================================================
-- 4) OPTIONAL OPERATOR BACKFILL (run day-by-day to bound memory usage)
-- =====================================================================
-- Materialized views only capture new inserts going forward. Because
-- event.metric_5m -> fill_metric_30m and event.series_6h -> 1d -> 1w
-- cascade automatically, backfilling event.metric_5m (day-by-day) also
-- populates event.metric_30m, and backfilling event.series_6h also
-- populates event.series_1d and event.series_1w:
--
--   INSERT INTO event.metric_5m
--   SELECT env, temporality, metric_name, fingerprint,
--          intDiv(unix_milli, 300000) * 300000 AS unix_milli,
--          anyLast(value) AS last, min(value) AS min, max(value) AS max,
--          sum(value) AS sum, count(*) AS count
--   FROM event.metric
--   WHERE toDate(unix_milli / 1000) = toDate('2026-11-25')
--     AND bitAnd(flags, 1) = 0
--   GROUP BY env, temporality, metric_name, fingerprint, unix_milli
--   SETTINGS max_memory_usage = 4000000000;
--
--   INSERT INTO event.series_6h
--   SELECT env, temporality, metric_name, description, unit, type, is_monotonic,
--          fingerprint, intDiv(unix_milli, 21600000) * 21600000 AS unix_milli,
--          labels, attrs, scope_attrs, resource_attrs, __normalized
--   FROM event.series
--   WHERE toDate(unix_milli / 1000) >= today() - 30;
