-- 0003_event_sample.sql — the metric plane gets a tenant, by RENAME.
--
-- WHAT THIS IS. `metric` is renamed to `sample`, because the row is one SAMPLE — the
-- word `metric` names the SERIES it belongs to, which is the table beside it. And the
-- rename is how the plane acquires an `org` column at all.
--
-- THE FINDING THAT MAKES THIS ALMOST FREE. Three correctly-keyed, org-first tables are
-- already sitting empty beside their populated tenant-less twins. That is `_v2` sprawl
-- without the suffix: two shapes of one table, one of them dead. So those three acquire
-- a tenant with ZERO rows rewritten — the empty org-first table takes the canonical
-- name, and the populated tenant-less one goes to `attic` to read out its own retention.
--
-- The whole migration, in one table. Every name on the left goes to `attic`; every name
-- on the right ends up canonical and org-first:
--
--   retiring, tenant-less              taking the canonical name
--   ─────────────────────              ─────────────────────────────────────────
--   metric            268,642,107  →   metric_buffer     0    promoted (step 2)
--   series              3,640,478  →   series_buffer     0    promoted (step 2)
--   descriptor             82,168  →   metric_attribute  0    promoted (step 2)
--   metric_5m          42,787,465  →   sample_5m              created  (step 4)
--   metric_30m          7,468,272  →   sample_30m             created  (step 4)
--   series_6h             919,324  →   series_6h              created  (step 4)
--   series_1d             406,895  →   series_1d              created  (step 4)
--   series_1w              55,887  →   series_1w              created  (step 4)
--
-- WHY THE SWAP AND NOT `ALTER TABLE ... ADD COLUMN org`. Adding the column is the cheap
-- part, and it is not the point: `org` has to be FIRST in the sort key. The fingerprint
-- is a hash of the labels and is therefore tenant-independent — two orgs reporting
-- `http_requests{path="/v1"}` produce the SAME fingerprint, and in a Replacing or
-- Aggregating table keyed without `org` they are ONE row. That is not a read-time gap a
-- WHERE clause can close; it is one tenant's row overwriting another's on merge.
-- Replayed against a copy of the live shapes, every ALTER that could put `org` in the
-- key was refused by the engine:
--
--   MODIFY ORDER BY (org, env, …)                        Code 36 — "Primary key must be
--       a prefix of the sorting key, but the column in the position 0 is org, not env"
--   ADD COLUMN org …; then MODIFY ORDER BY (…, org)      Code 36 — "Existing column org
--       is used in the expression that was added to the sorting key. You can add
--       expressions that use only the newly added columns"
--   ADD COLUMN org … DEFAULT '', MODIFY ORDER BY (…, org) Code 36 — "Newly added column
--       org has a default expression, so adding expressions that use it to the sorting
--       key is forbidden"
--
-- Exactly one form is accepted — ADD COLUMN with no default, `org` APPENDED — and it
-- buys the worst of both: no pruning, because a tenant's read still scans every other
-- tenant's granules; and 268M rows of history branded with the empty-string tenant this
-- design refuses to introduce. The swap is taken because it is the only form that
-- produces an org-FIRST key, and because it costs nothing — the org-first tables are
-- already sitting there.
--
-- THE SAME RULE APPLIES TO THE ROLLUPS. Promoting only metric/series/descriptor leaves
-- the five rollup tables tenant-less under canonical names and carries them forward as
-- though that were fine. Replayed on a copy, two orgs' series rows sharing one
-- fingerprint both reached `series_6h`, which has no `org` column, and the second
-- REPLACED the first. A tenancy migration that ends with one tenant's row overwriting
-- another's is worse than not running. So the rule — the tenant-less table goes to
-- `attic`, the org-first one takes the name — is applied to all eleven names in one
-- RENAME, and the five rollup targets, which have no org-first twin to promote, are
-- CREATEd org-first here.
--
-- The rollup history goes to `attic` alongside the samples it summarizes. That is not a
-- new loss, it is the SAME loss already accepted for the 268M rows underneath it: a
-- tenant-less summary of retired tenant-less samples has not earned a canonical name.
--
-- NOT DONE HERE, DELIBERATELY: no `$unattributed` backfill. A sentinel tenant in the
-- live vocabulary is a tenant every isolation test then has to know about forever, in
-- exchange for history that expires on its own in a month.
--
-- ORDERING. This migration, the `telemetrymetrics` READ change and the collector's
-- WRITE change are ONE window. After the RENAME there is no `event.metric` and no
-- `event.descriptor`, so a writer still addressing those names fails, and a read with no
-- leading `org` predicate against a table that now has an `org` column reads every
-- tenant's samples. `pkg/telemetrymetrics/tables.go` is the one place both names live.
-- If either side is not ready, do not run this.
--
-- OPERATOR-RUN, one window. The RENAME moves table entries; Datastore never copies or
-- rewrites parts. Reversed by renaming back and re-creating the five views on the old
-- names.

-- =====================================================================
-- 0) GATE — executable, not advisory.
-- =====================================================================
--
-- (a) The three tables about to take a canonical name must EXIST and be EMPTY. A
--     missing one means the plane is not what this file was written against; a non-empty
--     one means something started writing it and the rename would bury those rows.

SELECT throwIf(
    (SELECT count() FROM system.tables
      WHERE database = 'event' AND name IN ('metric_buffer', 'series_buffer', 'metric_attribute')) != 3
 OR (SELECT sum(total_rows) FROM system.tables
      WHERE database = 'event' AND name IN ('metric_buffer', 'series_buffer', 'metric_attribute')) > 0,
    'REFUSING: the org-first tables this migration promotes are missing, or one of them is no longer empty.');

-- (b) The shapes must still be compatible. READ THE WHOLE DEFINITION, not a projection
--     of it. An earlier draft diffed `system.columns`, and a TTL is not a column — so the
--     gate could not see that `metric_buffer` and `series_buffer` carry
--     `toIntervalDay(1)` where `metric` and `series` carry thirty days, and the swap
--     collapsed the plane's retention 30x in silence. `create_table_query` is the whole
--     definition in one field: columns, engine, partition key, sort key, TTL, settings.
--     Nothing can hide from it the way a retention hid from a column list. (Step 3 below
--     repairs that particular difference; this print is how a NEW one gets noticed.)

SELECT name, create_table_query
  FROM system.tables
 WHERE database = 'event'
   AND name IN ('metric', 'metric_buffer', 'series', 'series_buffer', 'descriptor', 'metric_attribute')
 ORDER BY name
 FORMAT Vertical;

--     And the two properties the migration depends on and cannot repair itself: the
--     promoted table must be org-FIRST, and it must not be missing a column its
--     predecessor had.

SELECT throwIf(
    (SELECT countIf(startsWith(sorting_key, 'org')) FROM system.tables
      WHERE database = 'event' AND name IN ('metric_buffer', 'series_buffer', 'metric_attribute')) != 3,
    'REFUSING: a table about to take a canonical name is not org-first. org must LEAD the sort key or two tenants sharing a fingerprint collapse into one row.');

SELECT throwIf(
    (SELECT count() FROM (
        SELECT table, name FROM system.columns
         WHERE database = 'event' AND table IN ('metric', 'series', 'descriptor')
        EXCEPT
        SELECT transform(table,
                         ['metric_buffer', 'series_buffer', 'metric_attribute'],
                         ['metric',        'series',        'descriptor']),
               name
          FROM system.columns
         WHERE database = 'event' AND table IN ('metric_buffer', 'series_buffer', 'metric_attribute')
     )) > 0,
    'REFUSING: a table being retired has a column its replacement does not. That is a shape change, not a rename — diff system.columns for the pair and read it.');

-- (c) The collector must be ready to write `org` from its authenticated identity, and to
--     write it to `sample`/`series`/`series_attribute`. A collector that writes the new
--     tables without an org produces rows no tenant owns, which the reads then fail
--     closed on — data accepted and unreadable. Step 6 has the query that proves it,
--     once the collector has run for an hour.

-- =====================================================================
-- 1) UNWIRE THE ROLLUP VIEWS — before the rename, not after.
-- =====================================================================
--
-- A materialized view binds its source BY NAME, and a RENAME moves names. Replayed on a
-- copy of the live shapes, doing the swap with the views in place produced two different
-- silent failures at once:
--
--   fill_metric_5m  ORPHANED  — nothing is called `event.metric` any more, so it lost its
--                               source and `sample_5m`/`sample_30m` stopped being fed.
--                               (`fill_metric_30m` went the same way when `metric_5m`
--                               moved.) The engine holds no dependency edge for either.
--   fill_series_6h  RE-ATTACHED — `event.series` still exists and is now the ORG-FIRST
--                               table, so the view kept firing and began pouring two
--                               tenants into an org-less rollup.
--
-- Neither announces itself. Dropping first is what leaves no instant in which a view is
-- attached to a table it was not written for.

DROP VIEW IF EXISTS event.fill_metric_5m;
DROP VIEW IF EXISTS event.fill_metric_30m;
DROP VIEW IF EXISTS event.fill_series_6h;
DROP VIEW IF EXISTS event.fill_series_1d;
DROP VIEW IF EXISTS event.fill_series_1w;

-- =====================================================================
-- 2) THE SWAP — one statement, atomic across all eleven names.
-- =====================================================================
--
-- `RENAME` takes a comma-separated list and applies it as one operation, so there is no
-- instant at which a canonical name is missing.
--
-- The rollup names are RETIRED here rather than promoted: `metric_5m` does not become
-- `sample_5m`, because it has no `org` column and `sample_5m` must. Step 4 creates them.

CREATE DATABASE IF NOT EXISTS attic;

RENAME TABLE
    event.metric            TO attic.metric,
    event.metric_buffer     TO event.sample,
    event.series            TO attic.series,
    event.series_buffer     TO event.series,
    event.descriptor        TO attic.descriptor,
    event.metric_attribute  TO event.series_attribute,
    event.metric_5m         TO attic.metric_5m,
    event.metric_30m        TO attic.metric_30m,
    event.series_6h         TO attic.series_6h,
    event.series_1d         TO attic.series_1d,
    event.series_1w         TO attic.series_1w;

-- =====================================================================
-- 3) RETENTION — restore the thirty days the promotion would have dropped.
-- =====================================================================
--
-- `metric` and `series` keep 30 days (`toIntervalSecond(2592000)`); the buffers that
-- take their names keep ONE (`toIntervalDay(1)`), because a buffer is a buffer. Promoted
-- unchanged, the plane's retention silently falls 30x and the loss is invisible until a
-- month-old dashboard comes back empty. Both tables are empty at this instant, so
-- MODIFY TTL is metadata and nothing is rewritten.
--
-- `metric_attribute` already carries `toIntervalDay(30)`, matching the `descriptor` it
-- replaces, so it is left alone — step 6 asserts that rather than trusting it.

ALTER TABLE event.sample MODIFY TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY;
ALTER TABLE event.series MODIFY TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY;

-- =====================================================================
-- 4) THE ROLLUP TABLES — same grains, org first.
-- =====================================================================
--
-- Each mirrors the table step 2 retired, with `org` prepended to the columns and leading
-- the sort key. The grain names a VALUE (5m, 30m, 6h, 1d, 1w) and never a version.

CREATE TABLE event.sample_5m
(
    `org`         LowCardinality(String),
    `env`         LowCardinality(String) DEFAULT 'default',
    `temporality` LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name` LowCardinality(String),
    `fingerprint` UInt64 CODEC(ZSTD(1)),
    `unix_milli`  Int64 CODEC(Delta(8), ZSTD(1)),
    `last`        SimpleAggregateFunction(anyLast, Float64) CODEC(ZSTD(1)),
    `min`         SimpleAggregateFunction(min, Float64) CODEC(ZSTD(1)),
    `max`         SimpleAggregateFunction(max, Float64) CODEC(ZSTD(1)),
    `sum`         SimpleAggregateFunction(sum, Float64) CODEC(ZSTD(1)),
    `count`       SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE TABLE event.sample_30m
(
    `org`         LowCardinality(String),
    `env`         LowCardinality(String) DEFAULT 'default',
    `temporality` LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name` LowCardinality(String),
    `fingerprint` UInt64 CODEC(ZSTD(1)),
    `unix_milli`  Int64 CODEC(Delta(8), ZSTD(1)),
    `last`        SimpleAggregateFunction(anyLast, Float64) CODEC(ZSTD(1)),
    `min`         SimpleAggregateFunction(min, Float64) CODEC(ZSTD(1)),
    `max`         SimpleAggregateFunction(max, Float64) CODEC(ZSTD(1)),
    `sum`         SimpleAggregateFunction(sum, Float64) CODEC(ZSTD(1)),
    `count`       SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE TABLE event.series_6h
(
    `org`            LowCardinality(String),
    `env`            LowCardinality(String) DEFAULT 'default',
    `temporality`    LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`    LowCardinality(String),
    `description`    LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `unit`           LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `type`           LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `is_monotonic`   Bool DEFAULT false CODEC(ZSTD(1)),
    `fingerprint`    UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`     Int64 CODEC(Delta(8), ZSTD(1)),
    `labels`         String CODEC(ZSTD(5)),
    `attrs`          Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `scope_attrs`    Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `resource_attrs` Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `__normalized`   Bool DEFAULT true CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE TABLE event.series_1d
(
    `org`            LowCardinality(String),
    `env`            LowCardinality(String) DEFAULT 'default',
    `temporality`    LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`    LowCardinality(String),
    `description`    LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `unit`           LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `type`           LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `is_monotonic`   Bool DEFAULT false CODEC(ZSTD(1)),
    `fingerprint`    UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`     Int64 CODEC(Delta(8), ZSTD(1)),
    `labels`         String CODEC(ZSTD(5)),
    `attrs`          Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `scope_attrs`    Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `resource_attrs` Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `__normalized`   Bool DEFAULT true CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE TABLE event.series_1w
(
    `org`            LowCardinality(String),
    `env`            LowCardinality(String) DEFAULT 'default',
    `temporality`    LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name`    LowCardinality(String),
    `description`    LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `unit`           LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `type`           LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    `is_monotonic`   Bool DEFAULT false CODEC(ZSTD(1)),
    `fingerprint`    UInt64 CODEC(Delta(8), ZSTD(1)),
    `unix_milli`     Int64 CODEC(Delta(8), ZSTD(1)),
    `labels`         String CODEC(ZSTD(5)),
    `attrs`          Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `scope_attrs`    Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `resource_attrs` Map(LowCardinality(String), String) DEFAULT map() CODEC(ZSTD(1)),
    `__normalized`   Bool DEFAULT true CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree
PARTITION BY toDate(unix_milli / 1000)
ORDER BY (org, env, temporality, metric_name, fingerprint, unix_milli)
TTL toDateTime(unix_milli / 1000) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- =====================================================================
-- 5) THE ROLLUP FEEDS — rebuilt on the new names, carrying the tenant.
-- =====================================================================
--
-- A view is named for what it fills, so the two that fill `sample_5m` / `sample_30m` are
-- `fill_sample_5m` / `fill_sample_30m`. The series views keep their names because
-- `series` kept its.
--
-- `org` is added to the GROUP BY, not just the projection. Without it the aggregate
-- would merge two tenants into one row before the row ever reached the target table.
--
-- The bucket floor is `intDiv` in all five. The series views inherited `floor(...)`,
-- which returns Float64 into an Int64 column — the same idea spelled two ways across
-- five sibling views, and a needless round trip through a type that cannot represent
-- every Int64. Same values, one spelling.

CREATE MATERIALIZED VIEW event.fill_sample_5m TO event.sample_5m AS
SELECT
    org,
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
FROM event.sample
WHERE bitAnd(flags, 1) = 0
GROUP BY org, env, temporality, metric_name, fingerprint, unix_milli;

CREATE MATERIALIZED VIEW event.fill_sample_30m TO event.sample_30m AS
SELECT
    org,
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
FROM event.sample_5m
GROUP BY org, env, temporality, metric_name, fingerprint, unix_milli;

CREATE MATERIALIZED VIEW event.fill_series_6h TO event.series_6h AS
SELECT
    org, env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint,
    intDiv(unix_milli, 21600000) * 21600000 AS unix_milli,
    labels, attrs, scope_attrs, resource_attrs, __normalized
FROM event.series;

CREATE MATERIALIZED VIEW event.fill_series_1d TO event.series_1d AS
SELECT
    org, env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint,
    intDiv(unix_milli, 86400000) * 86400000 AS unix_milli,
    labels, attrs, scope_attrs, resource_attrs, __normalized
FROM event.series_6h;

CREATE MATERIALIZED VIEW event.fill_series_1w TO event.series_1w AS
SELECT
    org, env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint,
    intDiv(unix_milli, 604800000) * 604800000 AS unix_milli,
    labels, attrs, scope_attrs, resource_attrs, __normalized
FROM event.series_1d;

-- =====================================================================
-- 6) VERIFY
-- =====================================================================

SELECT name, engine, total_rows FROM system.tables WHERE database = 'event' ORDER BY name;
-- Expect: sample, series, series_attribute, sample_5m, sample_30m, series_6h, series_1d,
-- series_1w present and empty; no metric, metric_buffer, series_buffer, descriptor,
-- metric_attribute, metric_5m, metric_30m.

-- Every canonical name in the metric plane leads its sort key with the tenant.

SELECT throwIf(
    (SELECT countIf(NOT startsWith(sorting_key, 'org')) FROM system.tables
      WHERE database = 'event'
        AND name IN ('sample', 'series', 'series_attribute',
                     'sample_5m', 'sample_30m', 'series_6h', 'series_1d', 'series_1w')) > 0,
    'A canonical table in the metric plane is not org-first. Two tenants sharing a fingerprint will collapse into one row.');

-- Every one of them keeps thirty days. This is the assertion the column-diff gate could
-- not make, and the reason the promotion silently cost 29 of them.

SELECT throwIf(
    (SELECT countIf(position(create_table_query, 'toIntervalDay(30)') = 0) FROM system.tables
      WHERE database = 'event'
        AND name IN ('sample', 'series', 'series_attribute',
                     'sample_5m', 'sample_30m', 'series_6h', 'series_1d', 'series_1w')) > 0,
    'RETENTION IS NOT 30 DAYS somewhere in the metric plane. Read create_table_query for the eight canonical names.');

-- The chain is WIRED: each source carries the view that reads it. This is the assertion
-- that would have caught the orphaned `fill_metric_5m` and the re-attached
-- `fill_series_6h` the moment either happened, instead of a month later.

SELECT throwIf(
    (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'sample'    AND has(dependencies_table, 'fill_sample_5m'))  != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'sample_5m' AND has(dependencies_table, 'fill_sample_30m')) != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'series'    AND has(dependencies_table, 'fill_series_6h'))  != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'series_6h' AND has(dependencies_table, 'fill_series_1d'))  != 1
 OR (SELECT count() FROM system.tables WHERE database = 'event' AND name = 'series_1d' AND has(dependencies_table, 'fill_series_1w'))  != 1,
    'THE ROLLUP CHAIN IS NOT WIRED: a fill_* view is not attached to the table it reads. sample_5m/30m and series_6h/1d/1w will silently stop being fed.');

-- The tenant contract on the metric plane, once the collector has written for an hour.
-- A non-zero answer means the collector is not stamping its identity.
--     SELECT count() FROM event.sample WHERE org = '';
--     SELECT count() FROM event.series WHERE org = '';

-- =====================================================================
-- 7) attic RETIRES ITSELF
-- =====================================================================
-- Every table moved to `attic` carries its own TTL (30 days across the metric plane), so
-- the history stays readable and then leaves without anyone deciding to delete it.
-- `DROP DATABASE attic` once `SELECT sum(total_rows) FROM system.tables WHERE
-- database='attic'` reaches zero — an empty database is a name and nothing else.
