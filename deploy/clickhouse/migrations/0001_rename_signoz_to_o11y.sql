-- 0001_rename_signoz_to_o11y.sql
--
-- Data-preserving ClickHouse cutover: rename the legacy `signoz_*` telemetry
-- databases and their `signoz_`-prefixed tables to the debranded `o11y_*`
-- identifiers that the o11y querier now reads (pkg/query-service, pkg/telemetry*).
--
-- WHY: the o11y read plane was debranded signoz_* -> o11y_* (databases, tables,
-- and query template identifiers). For a live deployment to keep serving its
-- existing telemetry after the upgrade, the physical ClickHouse objects must be
-- renamed to match. This is a coordinated cutover with the otel-collector write
-- side (see the CAVEAT below).
--
-- DATA-PRESERVING: every statement is a metadata-only `RENAME` (ClickHouse moves
-- the table/database entry — it never copies or rewrites parts). There is NO
-- DROP, NO CREATE ... AS, NO INSERT. Live telemetry is preserved in place.
--
-- GUARD / IDEMPOTENCY: ClickHouse `RENAME DATABASE|TABLE` has no `IF EXISTS`
-- clause, so apply this script ONCE during the cutover window, gated by the
-- pre-check below. The check makes it safe whether or not the objects exist
-- (fresh installs that were provisioned directly as o11y_* are skipped; a
-- half-applied run resumes because already-renamed objects fall out of the set).
--
--   -- run first; only proceed with the renames it reports as still `signoz_*`:
--   SELECT name FROM system.databases
--    WHERE name IN ('signoz_traces','signoz_metrics','signoz_logs',
--                   'signoz_metadata','signoz_analytics','signoz_meter');
--   SELECT database, name FROM system.tables
--    WHERE database IN ('signoz_traces','o11y_traces')
--      AND name LIKE 'signoz_%' OR name LIKE 'distributed_signoz_%';
--
-- CLUSTERED DEPLOYMENTS: append `ON CLUSTER '<cluster>'` (SigNoz/o11y default
-- cluster name is `cluster`) to every statement so the rename is replicated to
-- all shards/replicas, e.g. `RENAME DATABASE signoz_traces TO o11y_traces ON CLUSTER 'cluster';`.
--
-- =====================================================================
-- 1) Databases — moves every table inside (samples_v4, time_series_v4,
--    logs_v2, *_v4, usage, tag_attributes_v2, …) WITH its data.
-- =====================================================================
RENAME DATABASE signoz_traces   TO o11y_traces;
RENAME DATABASE signoz_metrics  TO o11y_metrics;
RENAME DATABASE signoz_logs     TO o11y_logs;
RENAME DATABASE signoz_metadata TO o11y_metadata;
RENAME DATABASE signoz_analytics TO o11y_analytics;
RENAME DATABASE signoz_meter    TO o11y_meter;

-- =====================================================================
-- 2) Tables whose NAME carries the signoz_ prefix (traces database only).
--    They now live under o11y_traces (renamed in step 1); rename the table
--    identifiers to the o11y_ names the querier references. Distributed and
--    local (sharded) tables are renamed as matched pairs.
-- =====================================================================
RENAME TABLE o11y_traces.signoz_index_v3                 TO o11y_traces.o11y_index_v3;
RENAME TABLE o11y_traces.distributed_signoz_index_v3     TO o11y_traces.distributed_o11y_index_v3;
RENAME TABLE o11y_traces.signoz_index_v2                 TO o11y_traces.o11y_index_v2;
RENAME TABLE o11y_traces.distributed_signoz_index_v2     TO o11y_traces.distributed_o11y_index_v2;
RENAME TABLE o11y_traces.signoz_error_index_v2           TO o11y_traces.o11y_error_index_v2;
RENAME TABLE o11y_traces.distributed_signoz_error_index_v2 TO o11y_traces.distributed_o11y_error_index_v2;
RENAME TABLE o11y_traces.signoz_spans                    TO o11y_traces.o11y_spans;
RENAME TABLE o11y_traces.distributed_signoz_spans        TO o11y_traces.distributed_o11y_spans;

-- =====================================================================
-- NOTE — COLUMNS: no data-preserving `ALTER TABLE … RENAME COLUMN` is required.
-- The audit found NO stored column carrying a `signoz_` prefix; the `signoz_*`
-- tokens that look column-shaped in the query layer are (a) SQL result aliases
-- (e.g. signoz_input_idx__, signoz_log_id__) which are query-time only, and
-- (b) span-metrics metric NAMES stored as row VALUES in the metrics `__name__`
-- column (signoz_calls_total, signoz_latency_bucket/_count, signoz_db_calls_total).
-- Those metric-name values are written by the collector spanmetrics connector,
-- not schema, so they are cut over on the write side (+ optional value backfill)
-- — outside DDL scope. If a backfill is desired it is a plain UPDATE on the
-- metric-name column, still no drop.
--
-- CAVEAT (coordination) — as of otel-collector v0.144.7 the collector's SQL
-- schema (cmd/o11yschemamigrator) still CREATES/writes the physical objects
-- under the `signoz_*` names above (only its Go package paths were debranded).
-- Apply this migration only in lockstep with a collector release whose schema
-- migrator emits the `o11y_*` identifiers, otherwise new ingest lands in the old
-- names. This file is the read-plane/data half of that coordinated rename.
