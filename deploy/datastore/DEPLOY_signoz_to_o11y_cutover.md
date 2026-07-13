# ClickHouse `signoz_*` → `o11y_*` schema cutover — lockstep deploy plan

The org-wide `signoz`→`o11y` rebrand renamed the **physical** ClickHouse
databases and tables. The writer (collector), the data-preserving RENAME
migration, and both readers (o11y querier + cloud) now all agree on `o11y_*`.

**These three MUST ship together. If the collector writes `o11y_*` before the
migration renames the existing data, or the readers query `o11y_*` before either,
telemetry blackholes** (new ingest lands in tables the readers don't query, or
readers query tables that don't exist yet).

## The shared identifier set (writer == migration == readers)

Proven byte-identical between the schema-migrator DDL (writer) and the RENAME
migration; both readers reference only names drawn from this set.

**Databases (6):** `o11y_traces` `o11y_metrics` `o11y_logs` `o11y_metadata`
`o11y_analytics` `o11y_meter`

**Tables carrying the prefix (8, traces db):** `o11y_index_v3` /
`distributed_o11y_index_v3`, `o11y_index_v2` / `distributed_o11y_index_v2`,
`o11y_error_index_v2` / `distributed_o11y_error_index_v2`, `o11y_spans` /
`distributed_o11y_spans`. (All other tables — `logs_v2`, `samples_v4`,
`time_series_v4`, `distributed_tag_attributes_v2`, … — keep their names; only
their containing database is renamed, moved wholesale by `RENAME DATABASE`.)

**Deliberately NOT renamed (wire contracts, unchanged on both sides):**
spanmetrics/usage metric NAMES (`signoz_calls_total`, `signoz_latency*`,
`signoz_*_count`/`_bytes`, `signoz_normalize_operator_logs_processed`,
`signoz_metadata_exporter_json_logs_processed`), resource ATTRIBUTES
(`signoz.gen_ai.*`), OTel component TYPE strings (`signozclickhousemetrics`,
`signozclickhousemeter`, `signozlogspipeline`, `signozllmpricing`),
query-time SQL aliases (`signoz_input_idx__`, `signoz_log_id__`).

## Component versions that must move together

| component | change | artifact |
|---|---|---|
| writer | schema-migrator + ClickHouse exporters emit `o11y_*` | `ghcr.io/hanzoai/otel-collector` **v0.144.8** (+ `o11y-schema-migrator`) |
| migration | data-preserving `RENAME DATABASE`/`RENAME TABLE` | `deploy/datastore/migrations/0001_rename_signoz_to_o11y.sql` |
| reader (o11y) | querier reads `o11y_*` | hanzoai/o11y#28 @ collector v0.144.8 |
| reader (cloud) | direct ClickHouse reads `o11y_*` | hanzoai/cloud#202 @ collector v0.144.8 |

## Pre-flight — prove it on a FRESH/scratch ClickHouse (never prod)

Run end-to-end against a throwaway ClickHouse before touching prod:

1. **Create schema fresh** with the v0.144.8 schema-migrator → it creates the
   `o11y_*` databases + tables directly (no `signoz_*` ever exist).
   ```
   docker run --rm --net <scratch-ch-net> ghcr.io/hanzoai/o11y-schema-migrator:v0.144.8 \
     sync --dsn 'tcp://<scratch-ch>:9000' --replication=false --up=
   ```
   Verify: `SELECT name FROM system.databases WHERE name LIKE 'o11y_%'` returns 6;
   `SELECT count() FROM system.tables WHERE name LIKE '%o11y_index_v3%'` > 0;
   `SELECT count() FROM system.databases WHERE name LIKE 'signoz_%'` = **0**.
2. **Emit a trace + a metric** through the v0.144.8 collector at the scratch DSN
   (OTLP/ZAP in). Confirm rows land: `SELECT count() FROM o11y_traces.distributed_o11y_index_v3`
   > 0 and `SELECT count() FROM o11y_metrics.distributed_samples_v4` > 0.
3. **Query it back through o11y** (community server @ collector v0.144.8, pointed
   at scratch DSN): the trace is visible via the querier and cloud's
   `requestLogs`/`metricsRead` SQL (`FROM o11y_traces.distributed_o11y_index_v3`)
   returns the row. Green = writer↔reader agree on a fresh install.
4. **Rehearse the RENAME on a signoz-seeded scratch DB**: create with the OLD
   v0.144.7 migrator (produces `signoz_*`), ingest a row, run
   `0001_rename_signoz_to_o11y.sql`, then confirm the row is still present under
   `o11y_*` and `system.databases` shows no `signoz_*`. Proves data is preserved
   in place (metadata-only rename, no copy).

## Prod cutover order (single maintenance window)

> Gated by the migration's own pre-check (`SELECT ... system.databases WHERE name
> IN ('signoz_traces', …)`). Fresh installs already on `o11y_*` skip the migration.
> Clustered deployments: append `ON CLUSTER '<cluster>'` to every RENAME (see the
> migration header).

1. **Deploy the collector v0.144.8** (schema-migrator + collector). On an existing
   install the migrator is a no-op for already-present databases; the collector
   now WRITES `o11y_*`. Briefly, new ingest targets `o11y_*` while old data is
   still under `signoz_*` — this is why step 2 follows immediately.
2. **Run the RENAME migration** `0001_rename_signoz_to_o11y.sql` once. It moves the
   existing `signoz_*` databases/tables (with all rows) to `o11y_*` — metadata-only,
   no drop, no copy. After this, historical + new data share the `o11y_*` names.
3. **Deploy o11y + cloud** (both @ collector v0.144.8). They now READ `o11y_*`,
   matching what the collector writes and the migration produced.

Roll the window tight: the gap between (1) and (3) is the only interval where a
stale reader (still on `signoz_*`) would see no data. Because the migration is a
metadata rename, step 2 is near-instant, keeping the window minimal.

## Rollback

`RENAME` is reversible: `RENAME DATABASE o11y_traces TO signoz_traces; …` (invert
every statement) restores the old names, and redeploying the prior collector
v0.144.7 + prior o11y/cloud images resumes `signoz_*` reads/writes. No data is
lost either direction because nothing is dropped.
