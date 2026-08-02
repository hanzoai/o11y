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
-- WHY IT PARTITIONS ON THE INGEST CLOCK. `time` is the caller's and may be backdated;
-- retention is measured from `ingested_at`, which is a column DEFAULT nothing on the
-- wire can reach. Partitioning on the event clock would land a backdated fact in an
-- already-expired partition.
--
-- `id` IS LAST IN ORDER BY AND MUST NEVER BE REMOVED. It is the entirety of redelivery
-- idempotency: the sink commits before it acks, so a redelivered fact must collapse
-- rather than duplicate. Drop `id` from the key and every bus retry doubles a row.
--
-- ADDITIVE. Every statement here CREATEs. Nothing is dropped, renamed or rewritten;
-- the four occurrence tables keep serving until their readers move (0004). Safe to run
-- on a live deployment, in the middle of the day, with ingest up.
--
-- CLUSTERED DEPLOYMENTS: append `ON CLUSTER '<cluster>'` to every statement, and split
-- each table into a local MergeTree plus a Distributed wrapper. This deployment is
-- single-shard, so the local table IS the table.

-- =====================================================================
-- 1) event.fact — the one occurrence table.
-- =====================================================================
--
-- THE COLUMN-GROWTH RULE, which is what keeps 39 columns from becoming 200: a column
-- exists when a SORT KEY, a SKIP INDEX, an AGGREGATE or a TYPED CONTRACT needs it.
-- Everything else is `attributes`. A Map costs roughly 6x a native column's bytes at
-- the same query time and no join — so the escape hatch is cheap enough that nothing
-- has to become a column to be storable.
--
-- SPARSITY IS NOT A COST. Measured on the live `event.log` (798,375 rows): an
-- unpopulated envelope column costs 515 bytes for the whole table. Six of them is
-- ~3 KiB against 48 MiB — 0.006%. The "sparse columns are wasteful" instinct is a
-- row-store instinct.

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
    INDEX by_severity severity      TYPE minmax GRANULARITY 1
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY (signal, toDate(ingested_at))
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
         -- declared at 825k rows; but the engine default for a Replacing table is
         -- `throw`, so without this setting the ALTER that adds the first projection
         -- FAILS. Declaring it now is what makes the remedy a one-line ALTER on the
         -- day a p95 justifies it, rather than a table rebuild.
         deduplicate_merge_projection_mode = 'rebuild';

-- =====================================================================
-- 2) event.session — one row per session, DERIVED, never trusted.
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
-- 3) event.trace / event.operation — REBUILT, because they are dead.
-- =====================================================================
--
-- Measured on the live warehouse: `event.trace` holds ONE row against 8,820 distinct
-- trace ids in `event.span`, and `event.operation` holds two. Nothing ever fed them.
-- Any design that leans on them as a fallback is leaning on nothing, so they are
-- rebuilt here as views over the fact table.
--
-- The two tables ALREADY EXIST with the right shape and the wrong (empty) contents, so
-- the operator moves them aside first — see step 6. These CREATEs assume that has
-- happened; run them after it.

CREATE TABLE IF NOT EXISTS event.trace
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

CREATE TABLE IF NOT EXISTS event.operation
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
-- 4) BACKFILL — four statements, 825,095 rows, seconds.
-- =====================================================================
--
-- This is a RE-DERIVE, not a data migration. Idempotent: the target is a Replacing
-- table keyed (org, time, id) within a single-signal partition, so re-running merges
-- rather than duplicating. Safe to run twice, safe to run while ingest is live.
--
-- Run these AFTER the writers have been repointed (0004 in the runbook), so nothing
-- lands in the old tables behind the backfill.

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
-- 5) VERIFY — assert the re-derive, per signal.
-- =====================================================================
-- Each pair must match. Run BEFORE step 6 and before anything is retired.

SELECT 'act'   AS signal, (SELECT count() FROM event.event) AS was, countIf(signal = 'act')   AS now FROM event.fact
UNION ALL
SELECT 'error',           (SELECT count() FROM event.error),         countIf(signal = 'error')  FROM event.fact
UNION ALL
SELECT 'log',             (SELECT count() FROM event.log),           countIf(signal = 'log')    FROM event.fact
UNION ALL
SELECT 'span',            (SELECT count() FROM event.span),          countIf(signal = 'span')   FROM event.fact;

-- The tenant contract, asserted against the data rather than against the code.
-- Both must return zero rows. A UUID in either column means a writer is still
-- spelling a tenant as a key instead of a slug.
SELECT DISTINCT org     FROM event.fact WHERE match(org,     '^[0-9a-f]{8}-[0-9a-f]{4}-');
SELECT DISTINCT product FROM event.fact WHERE match(product, '^[0-9a-f]{8}-[0-9a-f]{4}-');

-- =====================================================================
-- 6) THE ROLLUP CUTOVER — operator-run, metadata-only, reversible.
-- =====================================================================
--
-- `event.trace` and `event.operation` already exist under the names step 3 wants.
-- They are moved aside rather than dropped, so the step is a RENAME (an entry move —
-- Datastore never copies or rewrites parts) and is undone by renaming back.
--
-- GATE: both are dead. Confirm before running:
--     SELECT count() FROM event.trace;      -- expect 1
--     SELECT count() FROM event.operation;  -- expect 2
--
-- `attic` is where a superseded object waits out its own TTL. It is a DATABASE, not a
-- suffix: nothing in this namespace is ever named `_old`, `_v2` or `_new`.

CREATE DATABASE IF NOT EXISTS attic;

-- RENAME TABLE event.trace     TO attic.trace;
-- RENAME TABLE event.operation TO attic.operation;
--   ...then run step 3, then:
-- INSERT INTO event.trace SELECT org, trace_id, start, end,
--        countIf(signal='span'), countIf(signal='error'), max(duration),
--        anyIf(service, parent=''), anyIf(name, parent='')
--   FROM event.fact WHERE trace_id != '' AND signal IN ('span','error')
--   GROUP BY org, trace_id;   -- backfill what the MV will only see going forward
