-- 0002_event_fact.sql — ONE occurrence table for the whole platform.
--
-- WHAT THIS IS. `event` is the namespace and it holds two fact tables: `event.fact`,
-- one row per thing that HAPPENED, discriminated by a `signal` column; and
-- `event.sample`, one row per thing MEASURED. Everything else in the namespace is a
-- rollup of one of them or a dimension beside it.
--
-- WHY ONE TABLE AND NOT FIVE. Five tables with an identical envelope are consistent,
-- not unified: no cross-signal question can be asked without a hand-written five-way
-- UNION ALL, and a new product is a new table name — which is exactly how a namespace
-- grows a `_v2`. One table discriminated by a column answers "everything that happened
-- to org X in the last hour" as an ordinary query, and a new product is a `signal`
-- VALUE, which costs no name at all.
--
-- WHY `signal` IS IN THE PARTITION KEY. It buys four separate things:
--   1. Per-signal TTL at whole-part-drop cost. A part is single-signal, so the TTL's
--      WHERE is constant across it and `ttl_only_drop_parts` still drops whole parts.
--      Four different retentions survive intact in one table, with no mutations.
--   2. Pruning equal to separate tables: `WHERE signal='log'` reads a quarter of them.
--   3. The cross-signal query stays cheap, because time still leads within a partition.
--   4. Idempotency isolation: Replacing collapses only WITHIN a partition, so an `id`
--      collision between an `act` and a `log` cannot silently delete a row.
--
-- WHY THE TIME GRAIN IS A MONTH AND NOT A DAY. Point 4 cuts both ways: Replacing
-- collapses only within a partition, so THE PARTITION GRAIN IS THE REDELIVERY WINDOW.
-- The four tables this replaces all partition by month. A redelivered fact is a second
-- INSERT and takes a fresh `ingested_at`, so a delivery at 23:59:59.900 and its retry
-- 200ms later land in ONE monthly partition and collapse — but in TWO daily ones, where
-- they are two rows in two partitions and `OPTIMIZE ... FINAL` cannot merge across a
-- partition boundary. The duplicate is permanent. A daily grain would narrow the
-- redelivery window 30x against the tables this replaces, which is a regression the new
-- table has no reason to carry, and it buys nothing the month does not already give:
-- `ttl_only_drop_parts` drops PARTS, not partitions, and parts cluster by ingest time —
-- which is exactly how the 30-day TTL on the live `event.log` already expires.
--
-- WHY IT PARTITIONS ON THE INGEST CLOCK. `time` is the caller's and may be backdated;
-- retention is measured from `ingested_at`, which is a column DEFAULT nothing on the
-- wire can reach. Partitioning on the event clock would land a backdated fact in an
-- already-expired partition.
--
-- `id` IS LAST IN ORDER BY AND MUST NEVER BE REMOVED. It is the entirety of redelivery
-- idempotency: the sink commits before it acks, so a redelivered fact must collapse
-- rather than duplicate. Drop `id` from the key and every bus retry doubles a row.
--
-- ONE RENAME, THEN ADDITIVE. `event.trace` and `event.operation` already exist under
-- the names step 4 wants, holding a shape nothing has ever fed. Step 1 measures that
-- and moves them to `attic` — an entry move, not a copy. Everything after step 1
-- CREATEs. Safe to run on a live deployment, with ingest up: nothing writes the two
-- tables being moved, and the four occurrence tables keep serving until their readers
-- move (0004).
--
-- THE RENAME IS FIRST BECAUSE `CREATE TABLE IF NOT EXISTS` AGAINST AN OCCUPIED NAME IS
-- A SILENT NO-OP. Put the CREATEs first and they quietly do nothing, the rollup view
-- then dies on the OLD table's columns (`Code: 8 THERE_IS_NO_COLUMN: 'spans'`), and the
-- run aborts with the fact table built and the backfill entirely unrun. Ordering is the
-- whole fix, and `IF NOT EXISTS` is dropped from those two CREATEs so that a rename
-- that did not happen fails loudly instead of silently.
--
-- CLUSTERED DEPLOYMENTS: append `ON CLUSTER '<cluster>'` to every statement, and split
-- each table into a local MergeTree plus a Distributed wrapper. This deployment is
-- single-shard, so the local table IS the table.

-- =====================================================================
-- 1) THE CUTOVER — metadata-only, reversible, and it goes FIRST.
-- =====================================================================
--
-- Measured on the live warehouse: `event.trace` holds ONE row against 9,130 distinct
-- trace ids in `event.span`, and `event.operation` holds two against 68 distinct
-- (service, name) pairs. Nothing ever fed them. Any design that leans on them as a
-- fallback is leaning on nothing, so step 4 rebuilds them as rollups of the fact table.
--
-- The gate below does not restate those numbers, it RE-DERIVES them: each rollup is
-- refused if it holds as much as a tenth of the population it would hold were it alive.
-- A constant would rot; a ratio against the source cannot.
--
-- `attic` is where a superseded object waits out its own TTL. It is a DATABASE, not a
-- suffix: nothing in this namespace is ever named `_old`, `_v2` or `_new`. Both tables
-- carry a 30-day TTL, so they read out and leave without anyone deciding to delete them.

CREATE DATABASE IF NOT EXISTS attic;

SELECT throwIf(
    (SELECT count() FROM event.trace) * 10 > (SELECT uniqExact(trace_id) FROM event.span),
    'REFUSING: event.trace is not dead — it holds a tenth or more of the traces in event.span. Read it before moving it aside.');

SELECT throwIf(
    (SELECT count() FROM event.operation) * 10 > (SELECT uniqExact((service, name)) FROM event.span),
    'REFUSING: event.operation is not dead — it holds a tenth or more of the (service, name) pairs in event.span. Read it before moving it aside.');

RENAME TABLE event.trace     TO attic.trace,
             event.operation TO attic.operation;

-- =====================================================================
-- 2) event.fact — the one occurrence table.
-- =====================================================================
--
-- THE COLUMN-GROWTH RULE, which is what keeps 39 columns from becoming 200: a column
-- exists when a SORT KEY, a SKIP INDEX, an AGGREGATE or a TYPED CONTRACT needs it.
-- Everything else is `attributes`. A Map costs roughly 6x a native column's bytes at
-- the same query time and no join — so the escape hatch is cheap enough that nothing
-- has to become a column to be storable.
--
-- SPARSITY IS NOT A COST. Measured on the live `event.log`: an unpopulated envelope
-- column costs 515 bytes for the whole table. Six of them is ~3 KiB against 48 MiB —
-- 0.006%. The "sparse columns are wasteful" instinct is a row-store instinct.

CREATE TABLE IF NOT EXISTS event.fact
(
    -- ── spine ────────────────────────────────────────────────────────────────
    -- org is THE tenant: the IAM organization slug, stamped server-side from the
    -- validated principal, first in every sort key in this namespace. Never a UUID,
    -- never an integer, never translated through a lookup table.
    `org`          LowCardinality(String),
    -- signal is the closed discriminator: act | clip | error | log | span. The door
    -- derives what it accepts from the set of writers, so this vocabulary and the
    -- storage cannot disagree.
    `signal`       LowCardinality(String),
    -- time is when it HAPPENED, on the caller's clock (clamped forward to now).
    `time`         DateTime64(9) CODEC(DoubleDelta, ZSTD(1)),
    -- ingested_at is when the SERVER learned it. Retention and Replacing precedence
    -- both read this, so neither is the caller's to choose.
    `ingested_at`  DateTime64(3) DEFAULT now64(3) CODEC(DoubleDelta, ZSTD(1)),
    -- id is the fact's identity and the Replacing key's tail. Load-bearing.
    `id`           String CODEC(ZSTD(1)),

    -- ── what ─────────────────────────────────────────────────────────────────
    `name`         String CODEC(ZSTD(1)),
    -- kind is the discriminator WITHIN a signal — for act it is
    -- track|page|identify|group, for span the OTel span kind, for error the
    -- reported sub-type.
    `kind`         LowCardinality(String),
    -- message is one column because a log's body IS an error's message: the human
    -- text of what happened. Two names for it is how they diverge.
    `message`      String CODEC(ZSTD(1)),
    -- severity is the OTLP number, 1..24, and it is the ONLY spelling. The label
    -- ("ERROR", "warn") is a read-time function of it, so a row cannot carry a number
    -- and a word that disagree.
    `severity`     UInt8,
    -- duration in nanoseconds — a span's, and a clip's.
    `duration`     UInt64 CODEC(T64, ZSTD(1)),

    -- ── where ────────────────────────────────────────────────────────────────
    -- product is the SURFACE that produced the fact (docs, chat, console). A Sentry
    -- "project" is this, spelled in Sentry's vocabulary.
    `product`      LowCardinality(String),
    `env`          LowCardinality(String),
    `service`      LowCardinality(String),
    `release`      String CODEC(ZSTD(1)),
    `url`          String CODEC(ZSTD(1)),
    `path`         String CODEC(ZSTD(1)),
    `host`         LowCardinality(String) DEFAULT domain(url),

    -- ── who ──────────────────────────────────────────────────────────────────
    `person_id`    String CODEC(ZSTD(1)),
    `distinct_id`  String CODEC(ZSTD(1)),
    `anonymous_id` String CODEC(ZSTD(1)),
    -- groups is group-type -> group-key. A Map rather than group0..group4, because
    -- five positional slots are a sixth slot waiting to become a `_v2`.
    `groups`       Map(LowCardinality(String), String) CODEC(ZSTD(1)),

    -- ── correlation ──────────────────────────────────────────────────────────
    `session_id`   String CODEC(ZSTD(1)),
    `trace_id`     String CODEC(ZSTD(1)),
    `span_id`      String CODEC(ZSTD(1)),
    `parent`       String CODEC(ZSTD(1)),
    -- resource is the resource fingerprint; it joins event.log_resource /
    -- event.span_resource on their `fingerprint`. ONE name for it: the pre-existing
    -- plane spelled the same fact `resource UInt64` on one table and
    -- `resource_fingerprint String` on the same table, which is the two-spellings
    -- defect this schema exists to remove.
    `resource`     String CODEC(ZSTD(1)),

    -- ── open ─────────────────────────────────────────────────────────────────
    `attributes`   Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    -- el is the @hanzo/observe AST annotation: a NAMESPACE, not six more envelope
    -- columns. $name and $path collide head-on with the envelope's own name and path.
    `el`           Tuple(
                       label String,
                       role LowCardinality(String),
                       testid String,
                       name String,
                       component LowCardinality(String),
                       path Array(String)) CODEC(ZSTD(1)),

    -- ── error ────────────────────────────────────────────────────────────────
    -- issue is the deterministic grouping fingerprint and the join key of the
    -- relational issue row (status, assignee, first_seen). Named `issue` rather than
    -- `group` because that is what it identifies, and because `group` is a reserved
    -- word one letter from `groups`.
    `issue`        String CODEC(ZSTD(1)),
    `class`        LowCardinality(String),
    -- origin is the place of the fault — Sentry's culprit. Not `site`, which already
    -- names a published Site elsewhere in the platform.
    `origin`       String CODEC(ZSTD(1)),
    `handled`      Bool,
    `frames.function` Array(String) CODEC(ZSTD(1)),
    `frames.file`     Array(String) CODEC(ZSTD(1)),
    `frames.line`     Array(UInt32) CODEC(ZSTD(1)),
    `frames.column`   Array(UInt32) CODEC(ZSTD(1)),
    `frames.own`      Array(Bool)   CODEC(ZSTD(1)),

    -- ── span ─────────────────────────────────────────────────────────────────
    `status`       LowCardinality(String),

    -- ── clip ─────────────────────────────────────────────────────────────────
    -- A trace is assembled from spans by trace_id; a session is assembled from clips
    -- by session_id. A clip costs exactly two columns and reuses duration, session_id
    -- and url — which is the whole argument for one table: a new product that would
    -- have been a new table name is two columns and a signal VALUE.
    --
    -- object is the object-store address of the rrweb blob. The BLOB NEVER TRAVELS ON
    -- THE BUS AND NEVER LANDS IN A ROW: it is a multi-megabyte time-ordered binary,
    -- wrong for a message and wrong for a column. The bus carries the index.
    `object`       String CODEC(ZSTD(1)),
    `bytes`        UInt64 CODEC(T64, ZSTD(1)),

    -- ── derived ──────────────────────────────────────────────────────────────
    -- ts_bucket_start is NOT an envelope column: it is MATERIALIZED, so nothing writes
    -- it, nothing publishes it and it is not part of the contract — it is a pruning
    -- aid, on the same footing as a skip index. It exists because the o11y log/trace
    -- query builders emit `ts_bucket_start BETWEEN ...` as an AND'd predicate; under
    -- this sort key `time` prunes strictly better, so the column is redundant for
    -- pruning and is kept only so those builders compile. REMOVED BY: dropping the
    -- ~15 `ts_bucket_start` predicates in o11y/pkg/telemetry{logs,traces} and
    -- pkg/query-service, after which this line goes with them.
    `ts_bucket_start` UInt64 MATERIALIZED intDiv(toUnixTimestamp(time), 1800) * 1800 CODEC(DoubleDelta, ZSTD(1)),

    INDEX by_name     name          TYPE bloom_filter GRANULARITY 4,
    INDEX by_person   person_id     TYPE bloom_filter GRANULARITY 4,
    INDEX by_session  session_id    TYPE bloom_filter GRANULARITY 4,
    INDEX by_trace    trace_id      TYPE bloom_filter GRANULARITY 4,
    INDEX by_issue    issue         TYPE bloom_filter GRANULARITY 4,
    INDEX by_service  service       TYPE set(0)       GRANULARITY 4,
    INDEX by_message  message       TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 4,
    INDEX by_file     `frames.file` TYPE bloom_filter GRANULARITY 4,
    INDEX by_testid   el.testid     TYPE bloom_filter GRANULARITY 4,
    INDEX by_duration duration      TYPE minmax GRANULARITY 1,
    INDEX by_severity severity      TYPE minmax GRANULARITY 1,

    -- ── the tenant contract, ENFORCED ────────────────────────────────────────
    --
    -- `org` is the IAM organization slug and `product` is a surface. Neither is ever a
    -- UUID. That was written as a comment and asserted by a SELECT nobody had to read,
    -- and both were true of the code and false of the data: the Sentry writer emitted
    -- `orgID.String()`, so ONE tenant sat on the plane under TWO spellings — 86 rows
    -- as cd35a51b-f7e7-5412-b67f-58b8703e5219 and 2 as `hanzo`, which is that uuid's
    -- own preimage (uuid5 of "hanzo:o11y:org:hanzo"). A reader binding either spelling
    -- saw a fraction of its own errors.
    --
    -- A CONSTRAINT is the difference between a rule and a hope. It is evaluated on
    -- INSERT, so a writer that regresses is REFUSED at the door rather than discovered
    -- later by a migration, and the contract lives in ONE place — here — instead of in
    -- prose plus a check somebody has to remember to run.
    --
    -- The predicate is the uuid GRAMMAR, spelled out, and not `toUUIDOrNull(x) IS
    -- NULL` — which reads better and is WRONG: that parser accepts any 36 characters
    -- hyphenated in the right four places, so the slug
    -- `maxpower-team-alpha-beta-gamma-delta` parses as a uuid and a legitimate tenant
    -- would have its telemetry refused. A constraint that can reject a real tenant is
    -- worse than the defect it guards. Verified against the live engine (26.2.3): that
    -- slug is admitted by the regex below and rejected by toUUIDOrNull.
    --
    -- Both columns are LowCardinality, so the match runs over the dictionary rather
    -- than once per row.
    CONSTRAINT tenant_is_a_slug   CHECK NOT match(org,     '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'),
    CONSTRAINT product_is_a_slug  CHECK NOT match(product, '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$')
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY (signal, toYYYYMM(ingested_at))
PRIMARY KEY (org, time)
ORDER BY (org, time, id)
-- The four retentions the plane already had, preserved. The signal set is CLOSED and
-- enforced at the door (a fact whose signal has no writer is refused, not accepted and
-- discarded), so every partition this table can hold is covered by a clause below and
-- nothing can grow without a retention.
TTL toDateTime(ingested_at) + INTERVAL 30 DAY DELETE WHERE signal IN ('log', 'span', 'clip'),
    toDateTime(ingested_at) + INTERVAL 90 DAY DELETE WHERE signal = 'error',
    toDateTime(ingested_at) + INTERVAL 2 YEAR DELETE WHERE signal = 'act'
SETTINGS index_granularity = 8192,
         ttl_only_drop_parts = 1,
         -- Armed, unspent. A projection is a permanent second copy, so none is
         -- declared at this size; but the engine default for a Replacing table is
         -- `throw`, so without this setting the ALTER that adds the first projection
         -- FAILS. Declaring it now is what makes the remedy a one-line ALTER on the
         -- day a p95 justifies it, rather than a table rebuild.
         deduplicate_merge_projection_mode = 'rebuild';

-- =====================================================================
-- 3) event.session — one row per session, DERIVED, never trusted.
-- =====================================================================
--
-- The counters are computed from other signals rather than read from SDK-supplied
-- metadata: views come from `act`, errors from `error`, clips from `clip`. A client
-- that lies about its own session counts therefore cannot, and one materialized view
-- over one table replaces the three tables the fork used for the same answer.
--
-- THE COUNTERS COUNT IDS, NOT ROWS, and that is not a stylistic choice. A materialized
-- view fires on the INSERTED BLOCK, so it never sees the Replacing collapse that makes
-- `event.fact` idempotent — a redelivered fact would increment a `count()` a second
-- time and leave the rollup permanently disagreeing with the table it summarizes.
-- Counting distinct ids makes the rollup collapse the same redelivery the fact table
-- does, so the two cannot drift.
--
-- `bytes` is the ONE column that cannot be made idempotent this way (a sum of distinct
-- values has no such trick), so a redelivered clip over-counts it. It is a size hint,
-- never a billed or asserted number. The exact answer is one query away:
--     SELECT sum(bytes) FROM event.fact WHERE org = ? AND session_id = ? AND signal = 'clip'

CREATE TABLE IF NOT EXISTS event.session
(
    `org`        LowCardinality(String),
    `session_id` String CODEC(ZSTD(1)),
    `start`      SimpleAggregateFunction(min, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1)),
    `end`        SimpleAggregateFunction(max, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1)),
    `views`      AggregateFunction(uniqExactIf, String, UInt8),
    `errors`     AggregateFunction(uniqExactIf, String, UInt8),
    `clips`      AggregateFunction(uniqExactIf, String, UInt8),
    `bytes`      SimpleAggregateFunction(sum, UInt64) CODEC(T64, ZSTD(1)),
    `entry`      AggregateFunction(argMinIf, String, DateTime64(9), UInt8),
    `exit`       AggregateFunction(argMaxIf, String, DateTime64(9), UInt8)
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(`end`)
ORDER BY (org, session_id)
TTL toDateTime(`end`) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS event.session_roll TO event.session AS
SELECT
    org,
    session_id,
    min(time)                                             AS `start`,
    max(time)                                             AS `end`,
    uniqExactIfState(id, signal = 'act' AND kind = 'page') AS views,
    uniqExactIfState(id, signal = 'error')                AS errors,
    uniqExactIfState(id, signal = 'clip')                 AS clips,
    sumIf(bytes, signal = 'clip')                         AS bytes,
    argMinIfState(path, time, signal = 'act')             AS entry,
    argMaxIfState(path, time, signal = 'act')             AS `exit`
FROM event.fact
WHERE session_id != ''
GROUP BY org, session_id;

-- How a session list reads it, once:
--     SELECT org, session_id, min(start) AS start, max(end) AS end,
--            uniqExactIfMerge(views)  AS views,
--            uniqExactIfMerge(errors) AS errors,
--            uniqExactIfMerge(clips)  AS clips,
--            sum(bytes)               AS bytes,
--            argMinIfMerge(entry)     AS entry,
--            argMaxIfMerge(exit)      AS exit
--       FROM event.session WHERE org = ? GROUP BY org, session_id

-- =====================================================================
-- 4) event.trace / event.operation — REBUILT on the names step 1 freed.
-- =====================================================================
--
-- No `IF NOT EXISTS` here, deliberately: these two names were occupied a moment ago and
-- step 1 is the only thing that freed them. Without the rename these CREATEs must FAIL,
-- because the alternative — succeeding silently against the old shape — is the defect
-- that made this file unrunnable.

CREATE TABLE event.trace
(
    `org`       LowCardinality(String),
    `trace_id`  String CODEC(ZSTD(1)),
    `start`     SimpleAggregateFunction(min, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1)),
    `end`       SimpleAggregateFunction(max, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1)),
    `spans`     AggregateFunction(uniqExactIf, String, UInt8),
    `errors`    AggregateFunction(uniqExactIf, String, UInt8),
    `duration`  SimpleAggregateFunction(max, UInt64) CODEC(T64, ZSTD(1)),
    `service`   AggregateFunction(argMinIf, String, DateTime64(9), UInt8),
    `name`      AggregateFunction(argMinIf, String, DateTime64(9), UInt8)
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(`end`)
ORDER BY (org, trace_id)
TTL toDateTime(`end`) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS event.trace_roll TO event.trace AS
SELECT
    org,
    trace_id,
    min(time)                                AS `start`,
    max(time)                                AS `end`,
    uniqExactIfState(id, signal = 'span')    AS spans,
    uniqExactIfState(id, signal = 'error')   AS errors,
    max(duration)                            AS duration,
    -- The ROOT SPAN names the trace: the earliest SPAN with no parent. The signal
    -- test is load-bearing — an error carries no parent either, so `parent = ''`
    -- alone attributes the trace's name to whatever failed in it. argMin over an
    -- empty match yields an empty state that merges away, where `any` would let a
    -- block with no root win the merge with a blank.
    argMinIfState(service, time, signal = 'span' AND parent = '') AS service,
    argMinIfState(name,    time, signal = 'span' AND parent = '') AS name
FROM event.fact
WHERE trace_id != '' AND signal IN ('span', 'error')
GROUP BY org, trace_id;

-- How a trace list reads it, once:
--     SELECT org, trace_id, min(start) AS start, max(end) AS end,
--            uniqExactIfMerge(spans)  AS spans,
--            uniqExactIfMerge(errors) AS errors,
--            max(duration)            AS duration,
--            argMinIfMerge(service)   AS service,
--            argMinIfMerge(name)      AS name
--       FROM event.trace WHERE org = ? GROUP BY org, trace_id

CREATE TABLE event.operation
(
    `org`     LowCardinality(String),
    `service` LowCardinality(String),
    `name`    String CODEC(ZSTD(1)),
    `time`    SimpleAggregateFunction(max, DateTime64(9)) CODEC(DoubleDelta, ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(time)
ORDER BY (org, service, name)
TTL toDateTime(time) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS event.operation_roll TO event.operation AS
SELECT org, service, name, max(time) AS time
FROM event.fact
WHERE signal = 'span' AND service != ''
GROUP BY org, service, name;

-- =====================================================================
-- 5) BACKFILL — one statement per signal.
-- =====================================================================
--
-- This is a RE-DERIVE, not a data migration. Idempotent: the target is a Replacing
-- table keyed (org, time, id) within a single-signal MONTHLY partition, and each
-- statement carries the source's own `ingested_at` forward rather than taking the
-- column DEFAULT — so a second run lands every row in the partition it is already in
-- and merges rather than duplicating. Safe to run twice, safe to run while ingest is
-- live.
--
-- THE THREE ROLLUPS FILL THEMSELVES. session_roll, trace_roll and operation_roll exist
-- before this step and fire on these INSERTs, so event.session, event.trace and
-- event.operation are populated by the same four statements. There is no separate
-- rollup backfill to write, and therefore no second spelling of the rollup's SELECT to
-- keep in sync with the view's.
--
-- IDEMPOTENT IS A PROPERTY OF A TARGET, NOT OF A STATEMENT, and that is what the gate
-- below exists to say. These four are idempotent in `event.fact`: Replacing collapses a
-- re-derived row onto the one already there. The property does not travel. A
-- materialized view attached to `event.fact` fires on the INSERTED BLOCK — it never sees
-- the collapse — so it forwards every re-derived row downstream, where the guarantee is
-- whatever THAT table's engine happens to be, and where rows the fact table has never
-- held arrive for the first time and collide with nothing.
--
-- The three rollups above are exempt BY CONSTRUCTION rather than by exception: they
-- count ids and not rows (step 3 says why), so they collapse the same re-derive the fact
-- table does. A consumer this file did not write has semantics this file cannot know, so
-- the gate refuses and names it rather than guessing.
--
-- `dependencies_table` is the engine's own list, kept current as views attach and
-- detach. Asking the same question of `create_table_query` with a LIKE is asking a
-- string, and it answers differently the day someone aliases the table.
--
-- IF THE GATE FIRES, THERE ARE TWO WAYS FORWARD AND ONLY ONE OF THEM IS THIS STEP.
-- Detach the consumer, run the backfill, re-attach it — or skip step 5 and populate the
-- three rollups straight from `event.fact`:
--
--     INSERT INTO event.session <the body of event.session_roll, verbatim>
--
-- which reaches no consumer at all, because the views are attached to the fact table and
-- not to the rollups. Read the body from the view rather than copying it into a runbook:
-- `SELECT as_select FROM system.tables WHERE database = 'event' AND name = 'session_roll'`
-- is the same one spelling this file already refuses to duplicate.

SELECT throwIf(length(unowned) > 0,
    'A CONSUMER THIS FILE DOES NOT OWN IS ATTACHED TO event.fact. The backfill fires it once per re-derived row and it carries no collapse guarantee. See the two remedies above this statement.') AS ok,
       unowned
FROM (
    SELECT arrayFilter(
               d -> NOT has([('event', 'session_roll'), ('event', 'trace_roll'), ('event', 'operation_roll')], d),
               arrayZip(dependencies_database, dependencies_table)) AS unowned
    FROM system.tables
    WHERE database = 'event' AND name = 'fact'
);
--
-- NO ROW COUNT IS WRITTEN HERE. The plane is live and growing — the `log` signal alone
-- passed two million while this file was being corrected, against the 825,095 an
-- earlier draft asserted for all four — so a total in a comment is a total that is
-- wrong by the time it is read. Step 6 is the authority: it counts both sides at the
-- moment of the run and refuses to pass unless they agree.
--
-- Run these AFTER the writers have been repointed (0004 in the runbook), so nothing
-- lands in the old tables behind the backfill. Step 6 catches the mistake if they have
-- not been: a source that is still growing will not match.
--
-- THE PRECONDITION, FIRST, BECAUSE THE BACKFILL CANNOT SURVIVE A BAD SPELLING. The
-- tenant constraint on event.fact refuses a uuid, so a source table that still holds
-- one aborts its INSERT — partway through, after the earlier statements have already
-- landed. The statement below asks the same question of the SOURCES before anything is
-- written, so the answer is a refusal to start with the offending value named, rather
-- than a half-done backfill with a constraint violation to interpret.
--
-- On this deployment it was NOT vacuous: event.error held 88 rows whose `org` was a
-- uuid and 13 whose `product` was, written by the Sentry ingest path.
--
-- REPAIRING THEM IS A SEPARATE STEP AND IT BELONGS TO THE OPERATOR, because a mutation
-- on a source table is not this file's call. But the map is DERIVED, never guessed, so
-- it is written out here rather than left as an exercise.
--
-- An ORG's uuid is `uuid5(NameSpaceURL, 'hanzo:o11y:org:' || slug)`
-- (`toUUID` in pkg/identn/iamidentn/provider.go). That is invertible by re-deriving from
-- a candidate slug, and both values on this deployment reproduce exactly:
--
--     cd35a51b-f7e7-5412-b67f-58b8703e5219  =  uuid5(url, 'hanzo:o11y:org:hanzo')  -> hanzo
--     dfb7a19b-108f-5150-8131-7d207488bf48  =  uuid5(url, 'hanzo:o11y:org:admin')  -> admin
--
--     ALTER TABLE event.error UPDATE org = 'hanzo' WHERE org = 'cd35a51b-f7e7-5412-b67f-58b8703e5219';
--     ALTER TABLE event.error UPDATE org = 'admin' WHERE org = 'dfb7a19b-108f-5150-8131-7d207488bf48';
--
-- A PRODUCT's uuid is not derived and cannot be inverted: 019f9b1e-5785-7359-… is a
-- UUIDv7 (the version nibble is 7), a MINTED Sentry project id, so its slug is a lookup
-- rather than a computation — read `slug` from the `o11y_sentry_projects` row with that
-- id, in the relational store, and update `product` to it.
--
-- Re-derive before writing. The mapping above is this deployment's, and the statement
-- below is what says whether it is still complete.

SELECT throwIf(count() > 0,
    'A SOURCE TABLE STILL SPELLS A TENANT AS A UUID. event.fact refuses it, so the backfill would abort partway. Repair the source first — see the note above this statement.') AS ok
FROM (
    SELECT org, product FROM event.event
    UNION ALL SELECT org, product FROM event.error
    UNION ALL SELECT org, product FROM event.log
    UNION ALL SELECT org, product FROM event.span
)
WHERE match(org,     '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$')
   OR match(product, '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$');

INSERT INTO event.fact
    (org, signal, time, ingested_at, id, name, kind, product,
     session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el)
SELECT org, 'act', time, ingested_at, id, name, kind, product,
       session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el
FROM event.event;

INSERT INTO event.fact
    (org, signal, time, ingested_at, id, name, kind, product,
     session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el,
     issue, message, class, origin, handled, severity, release, env, service,
     trace_id, span_id,
     `frames.function`, `frames.file`, `frames.line`, `frames.column`, `frames.own`)
SELECT org, 'error', time, ingested_at, id, name, kind, product,
       session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el,
       `group`, message, class, site, handled,
       -- level -> the OTLP number, which is the one spelling from here on.
       multiIf(level = 'fatal', 21, level = 'error', 17, level = 'warning', 13,
               level = 'warn', 13, level = 'info', 9, level = 'debug', 5, 17),
       release, environment, service, trace_id, span_id,
       `frames.function`, `frames.file`, `frames.line`, `frames.column`, `frames.own`
FROM event.error;

INSERT INTO event.fact
    (org, signal, time, ingested_at, id, name, kind, product,
     session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el,
     service, severity, message, trace_id, span_id, resource)
SELECT org, 'log', time, ingested_at, id, name, kind, product,
       session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el,
       service,
       -- severity_number when the emitter set it, else derived from the text.
       if(severity_number != 0, severity_number,
          multiIf(severity_text ILIKE 'fatal%', 21, severity_text ILIKE 'error%', 17,
                  severity_text ILIKE 'warn%', 13, severity_text ILIKE 'info%', 9,
                  severity_text ILIKE 'debug%', 5, severity_text ILIKE 'trace%', 1, 0)),
       body, trace_id, span_id, resource_fingerprint
FROM event.log;

INSERT INTO event.fact
    (org, signal, time, ingested_at, id, name, kind, product,
     session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el,
     service, trace_id, span_id, parent, duration, status, resource)
SELECT org, 'span', time, ingested_at, id, name, kind, product,
       session_id, distinct_id, anonymous_id, person_id, url, path, attributes, el,
       service, trace_id, span_id, parent, duration, status, resource_fingerprint
FROM event.span;

-- =====================================================================
-- 6) VERIFY — assert the re-derive, per signal.
-- =====================================================================
--
-- IT COUNTS IDS, NOT ROWS, for the same reason the rollups do: both sides are Replacing
-- tables, so `count()` reads whatever has not merged yet and would report a difference
-- that is only a pending merge. `uniqExact(id)` is the collapsed count by definition and
-- needs no FINAL.
--
-- The assertion rides in the projection rather than in a second statement, so the pair
-- of counts is spelled once. On a good run it prints the proof, one row per signal, with
-- `ok = 0`. On a bad one it aborts and the same query without its last column names the
-- signal that differs.
--
-- `event.fact AS f` IS LOAD-BEARING. Written unqualified, `'act' AS signal` shadows the
-- `signal` COLUMN, so `signal = 'act'` is a comparison of the label against itself —
-- constantly true — and the branch counts every id in the table rather than the act
-- ones. Measured on the copy: 23,260 reported against 200 actually present. The table
-- alias is what keeps the label and the column apart.

SELECT signal, was, now, throwIf(was != now, 'BACKFILL INCOMPLETE: a signal in event.fact does not match its source table. Re-run step 5 for that signal.') AS ok
FROM (
    SELECT 'act'   AS signal, (SELECT uniqExact(id) FROM event.event) AS was, uniqExactIf(f.id, f.signal = 'act')   AS now FROM event.fact AS f
    UNION ALL
    SELECT 'error',           (SELECT uniqExact(id) FROM event.error),        uniqExactIf(f.id, f.signal = 'error') FROM event.fact AS f
    UNION ALL
    SELECT 'log',             (SELECT uniqExact(id) FROM event.log),          uniqExactIf(f.id, f.signal = 'log')   FROM event.fact AS f
    UNION ALL
    SELECT 'span',            (SELECT uniqExact(id) FROM event.span),         uniqExactIf(f.id, f.signal = 'span')  FROM event.fact AS f
)
ORDER BY signal;

-- THERE IS NO TENANT ASSERTION HERE, and that is the point. It used to be two bare
-- SELECTs at the end of this file — a check that ran once, printed a table, and was
-- believed by whoever happened to read it. The contract is now a CONSTRAINT on
-- event.fact (step 1), so it is evaluated on EVERY insert forever, and the
-- precondition above asks the same question of the sources before the backfill starts.
-- A rule that holds is worth more than a rule that was checked.
