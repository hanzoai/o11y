-- 0005_iam_roles.sql — who may do what in the event plane, and to whose rows.
--
-- WHAT THIS IS. The warehouse (hanzoai/datastore) authenticates every caller with a
-- Hanzo IAM access token: no stored users, no database passwords. Its IAM user
-- directory names an ephemeral user after the token's `sub`, binds the token's `owner`
-- to the session (read in SQL as currentOrg()), and grants the session roles decided
-- from the token (datastore docker/server/datastore-iam.xml):
--
--   tenant      every IAM identity
--   sink        a client_credentials token issued to hanzo-cloud (the event sink)
--   writer      a client_credentials token issued to hanzo-o11y (the telemetry plane)
--   reader      every person (a token that is not a program's)
--   superadmin  owner == 'admin', and nothing else
--
-- The directory only decides which roles a token holds. What the roles may do, and which
-- rows they see, is this file.
--
-- ONE PREDICATE, ON THE WHOLE NAMESPACE. Every table in `event` is org-first (0002,
-- 0003, and the telemetry schema.sql files lead each sort key with `org`), so the
-- confinement is a single database-level policy, `ON event.*`, rather than one per
-- table: a table added later is confined the moment it exists. A table in `event`
-- without an `org` column fails a tenant's SELECT with an unknown-column error. That is
-- the wanted failure: it refuses, it does not leak.
--
-- ON THE BASELINE ROLE, NOT ON READER. The policy is attached to `tenant`, which every
-- IAM identity holds whatever else it holds, so a role granted SELECT later is confined
-- without anyone remembering to extend this file. Permissive policies are OR-ed, so
-- superadmin's `USING 1` widens its own view to every row and nobody else's.
--
-- NULL, NOT ''. currentOrg() is NULL for a token that names no organization, and for
-- every login that is not an IAM token. `org = NULL` matches nothing, so such a caller
-- sees no rows, including the unattributed rows the metric tables store under ''.
--
-- WHAT A ROW POLICY DOES NOT COVER. It filters reads. The two writers insert rows for
-- every org because they are the platform's own ingest; that is what `sink` and
-- `writer` are for, and why neither is granted SELECT. A caller that logs in with a
-- password rather than a token holds none of these roles and is not confined here
-- (access_control_improvements.users_without_row_policies_can_read_rows); the end state
-- is that no such caller exists.
--
-- ORDER. Land it with, or after, the warehouse image that has the IAM directory and
-- currentOrg(). A policy's filter is resolved when a covered role reads, and on an older
-- binary no session holds these roles, so nothing reads through them there. The
-- statements are idempotent. Run by an identity holding ALL WITH GRANT OPTION:
-- superadmin's grant needs it.

CREATE ROLE IF NOT EXISTS tenant;
CREATE ROLE IF NOT EXISTS reader;
CREATE ROLE IF NOT EXISTS writer;
CREATE ROLE IF NOT EXISTS sink;
CREATE ROLE IF NOT EXISTS superadmin;

GRANT SELECT ON event.* TO reader;
GRANT INSERT ON event.* TO writer;
GRANT INSERT ON event.fact TO sink;
GRANT ALL ON *.* TO superadmin WITH GRANT OPTION;

CREATE ROW POLICY OR REPLACE tenant ON event.* USING org = currentOrg() AS PERMISSIVE TO tenant;
CREATE ROW POLICY OR REPLACE superadmin ON event.* USING 1 AS PERMISSIVE TO superadmin;

-- The five roles exist and the namespace carries both policies, with the filters above.
SELECT throwIf(
    (SELECT count() FROM system.roles WHERE name IN ('tenant', 'reader', 'writer', 'sink', 'superadmin')) != 5
 OR (SELECT count() FROM system.row_policies
      WHERE database = 'event' AND table = '' AND short_name = 'tenant'
        AND select_filter = 'org = currentOrg()' AND apply_to_list = ['tenant']) != 1
 OR (SELECT count() FROM system.row_policies
      WHERE database = 'event' AND table = '' AND short_name = 'superadmin'
        AND select_filter = '1' AND apply_to_list = ['superadmin']) != 1,
    'The IAM roles or the event.* row policies are not as this file declares them.') AS ok;
