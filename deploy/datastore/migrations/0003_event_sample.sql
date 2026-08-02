-- 0003_event_sample.sql — the metric plane gets a tenant, by RENAME.
--
-- WHAT THIS IS. `metric` is renamed to `sample`, because the row is one SAMPLE — the
-- word `metric` names the SERIES it belongs to, which is the table beside it. And the
-- rename is how the plane acquires an `org` column at all.
--
-- THE FINDING THAT MAKES THIS ALMOST FREE. There are already SEVEN correctly-keyed,
-- org-first tables sitting empty beside the eight populated tenant-less ones:
--
--   populated, no org               empty, org-first, otherwise identical
--   ─────────────────────           ────────────────────────────────────
--   metric        268,642,107  →    metric_buffer          0
--   series          3,640,478  →    series_buffer          0
--   descriptor         82,168  →    metric_attribute       0
--                                   series_reduced         0
--                                   metric_reduced_last    0
--                                   metric_reduced_sum     0
--                                   histogram              0
--
-- That is `_v2` sprawl without the suffix: two shapes of one table, one of them dead.
-- So the whole 268M-row plane acquires a tenant with ZERO rows rewritten — the empty
-- org-first table takes the canonical name, and the populated tenant-less one goes to
-- `attic` to read out its own 30-day retention.
--
-- NOT DONE HERE, DELIBERATELY: no `$unattributed` backfill of 268M rows. A sentinel
-- tenant in the live vocabulary is a tenant that every isolation test then has to know
-- about forever, in exchange for history that expires on its own in a month.
--
-- ORDERING. This migration and the `telemetrymetrics` read change SHIP TOGETHER. A
-- metric read with no leading `org` predicate against a table that now has an `org`
-- column reads every tenant's samples. If the read change is not ready, do not run
-- this one.
--
-- OPERATOR-RUN, one window, metadata-only. `RENAME` moves a table entry; Datastore
-- never copies or rewrites parts. Reversed by renaming back.

-- =====================================================================
-- 0) GATE — run these three FIRST and read the answers.
-- =====================================================================
--
-- (a) The org-first tables must still be EMPTY. A non-zero count means something
--     started writing them and this rename would bury it.
SELECT name, total_rows
  FROM system.tables
 WHERE database = 'event'
   AND name IN ('metric_buffer', 'series_buffer', 'metric_attribute',
                'series_reduced', 'metric_reduced_last', 'metric_reduced_sum')
 ORDER BY name;
-- Expect: six rows, every total_rows = 0. STOP if not.

-- (b) The shapes must still be compatible. Diff each pair before trusting the swap.
--     Known and accepted differences, measured:
--       metric_buffer.inserted_at_unix_milli  UInt64  vs metric.…  Int64
--       series_buffer  carries an extra `is_reduced Bool DEFAULT false`
--       metric_attribute ORDER BY drops `temporality` (it is (org, metric_name, …))
--     Anything ELSE that differs is a shape change, not a rename — stop and read it.
SELECT table, name, type
  FROM system.columns
 WHERE database = 'event'
   AND table IN ('metric', 'metric_buffer', 'series', 'series_buffer',
                 'descriptor', 'metric_attribute')
 ORDER BY table, position;

-- (c) The collector must be ready to write `org` from its authenticated identity.
--     A collector that writes the new tables without it produces rows no tenant owns,
--     which the reads then fail closed on — data accepted and unreadable.

-- =====================================================================
-- 1) THE SWAP — one statement, atomic across all eight names.
-- =====================================================================
--
-- `RENAME` takes a comma-separated list and applies it as one operation, so there is
-- no instant at which a canonical name is missing.

CREATE DATABASE IF NOT EXISTS attic;

RENAME TABLE
    event.metric            TO attic.metric,
    event.metric_buffer     TO event.sample,
    event.series            TO attic.series,
    event.series_buffer     TO event.series,
    event.descriptor        TO attic.descriptor,
    event.metric_attribute  TO event.series_attribute,
    event.metric_5m         TO event.sample_5m,
    event.metric_30m        TO event.sample_30m;

-- =====================================================================
-- 2) THE ROLLUP FEEDS — repoint them at the new names.
-- =====================================================================
--
-- `fill_metric_5m` / `fill_metric_30m` are materialized views that read `event.metric`
-- and write `event.metric_5m` / `_30m`. A materialized view's SELECT is fixed at
-- creation, so a renamed source is a view pointing at `attic`. Read each one's current
-- definition and recreate it against the new names — the shape is not invented here,
-- it is whatever these print:
--
--     SELECT name, as_select FROM system.tables
--      WHERE database = 'event' AND engine = 'MaterializedView';
--
-- The five views this affects: fill_metric_5m, fill_metric_30m, fill_series_6h,
-- fill_series_1d, fill_series_1w. Their targets keep their own names (series_6h,
-- series_1d, series_1w name a GRAIN, which is a value and not a version).

-- =====================================================================
-- 3) VERIFY
-- =====================================================================
SELECT name, engine, total_rows FROM system.tables WHERE database = 'event' ORDER BY name;
-- Expect: sample, series, series_attribute, sample_5m, sample_30m present and empty;
-- no `metric`, `metric_buffer`, `series_buffer`, `descriptor`, `metric_attribute`.

-- The tenant contract on the metric plane, once the collector has written for an hour.
-- A non-empty answer means the collector is not stamping its identity.
SELECT count() FROM event.sample WHERE org = '';

-- =====================================================================
-- 4) attic RETIRES ITSELF
-- =====================================================================
-- Every table moved to `attic` carries its own TTL (30 days on the metric plane), so
-- the history stays readable and then leaves without anyone deciding to delete it.
-- `DROP DATABASE attic` once `SELECT sum(total_rows) FROM system.tables WHERE
-- database='attic'` reaches zero — an empty database is a name and nothing else.
