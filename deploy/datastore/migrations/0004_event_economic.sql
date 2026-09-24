-- 0004_event_economic.sql — the economic signal gets its retention.
--
-- WHAT THIS IS. `economic` is a sixth signal on event.fact: one payment between two
-- orgs, written once into each party's partition (hanzoai/cloud apps/event/economic.go).
-- 0002 says every signal the table can hold is covered by a TTL clause, so that nothing
-- grows without a retention. The economic writer landed without one, which kept every
-- payment forever. This gives it its own.
--
-- WHY TEN YEARS, AND WHY IT IS A FLOOR. These rows are the books a tax year is read
-- from, so they are kept as long as the law may still ask for them:
--
--   26 CFR 1.6001-1(e): records "shall be retained so long as the contents thereof may
--   become material in the administration of any internal revenue law."
--
--   IRS, "How long should I keep records" — the longest finite period is SEVEN YEARS,
--   for a claim for a loss from worthless securities or a bad debt deduction; the others
--   are three, four (employment tax) and six (income under-reported by more than 25%).
--   Each runs from when the return was FILED, not from when the payment was made.
--
--   IRS General Instructions for Certain Information Returns — a 1099 filer keeps copies
--   "for at least 3 years (4 years for Form 1099-C) from the due date of the returns".
--
-- A payment made on 1 January of year Y is reported on a return filed as late as
-- 15 October of Y+1 under extension: 21.5 months after it. Seven years from that filing
-- is 8 years 9.5 months after the payment. The clock here is ingested_at, which is never
-- earlier than the payment (a rail states a payment after it moves, and a rail that was
-- down states it later), so ten years covers the longest finite period from an extended
-- return with a year to spare for one filed late. The two cases the IRS keeps open
-- INDEFINITELY — no return filed, or a fraudulent one — no retention can serve, and a
-- row is not deleted early for them either: this is the least the plane keeps, not the
-- most a reader may need.
--
-- IT IS NOT THE HAND-OFF. The EVENT stream holds a fact for 72 hours; a warehouse down
-- longer than that loses nothing, because a rail keeps owing a statement until the
-- event app answers that both parties' rows are HERE (cloud apps/x402/economic.go). This
-- clause is what happens to a row after it has landed.
--
-- WHOLE PARTS, AS BEFORE. `signal` is in the partition key, so a part is single-signal
-- and `ttl_only_drop_parts` drops an economic part whole when its last row is ten years
-- old. The four clauses before it are 0002's, restated unchanged: MODIFY TTL replaces
-- the table's TTL, so a clause left out here would be a retention removed.
--
-- NO REWRITE. materialize_ttl_after_modify = 0 records the new TTL for parts written
-- from now on and rewrites none of the parts already on disk. None of those holds an
-- economic row when this runs before the writer ships; one that does keeps its row
-- until a merge rewrites the part with this TTL — longer, never shorter.
--
-- ORDER: run this before, or with, the cloud release whose sink lands `economic`, so
-- every economic part carries its retention from its first insert.

ALTER TABLE event.fact
    MODIFY TTL toDateTime(ingested_at) + INTERVAL 30 DAY DELETE WHERE signal IN ('log', 'span', 'clip'),
               toDateTime(ingested_at) + INTERVAL 90 DAY DELETE WHERE signal = 'error',
               toDateTime(ingested_at) + INTERVAL 2 YEAR DELETE WHERE signal = 'act',
               toDateTime(ingested_at) + INTERVAL 10 YEAR DELETE WHERE signal = 'economic'
SETTINGS materialize_ttl_after_modify = 0;

-- The table now names a retention for every signal a writer lands. Refused otherwise.
SELECT throwIf(
    position((SELECT engine_full FROM system.tables WHERE database = 'event' AND name = 'fact'),
             'toIntervalYear(10) WHERE signal = \'economic\'') = 0,
    'event.fact has no retention for the economic signal — the ALTER above did not apply.') AS ok;
