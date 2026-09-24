-- 0005_iam_roles.sql — who may do what in the event plane, and to whose rows.
--
-- WHAT THIS IS. The warehouse (hanzoai/datastore) authenticates every caller with a
-- Hanzo IAM access token: no stored users, no database passwords. Its IAM user
-- directory names an ephemeral user after the token's `sub`, binds the bearer's HOME
-- org to the session (the first entry of the signed `orgs` membership set, read in SQL
-- as currentOrg(); never the `owner` claim, which names the org of the application the
-- token was minted through), and grants the session roles decided from the token
-- (datastore docker/server/datastore-iam.xml):
--
--   tenant      every IAM identity
--   sink        a program token issued to hanzo-cloud (the event sink)
--   writer      a program token issued to hanzo-o11y (the telemetry plane)
--   schema      a program token issued to hanzo-o11y: this file's runner
--   reader      every person (not a program, member of at least one org)
--   superadmin  a person whose home org is admin, signed with the admin org's own key
--
-- superadmin and schema are declared by the warehouse image itself (users.d,
-- datastore-iam-roles.xml), because something has to exist before the first migration
-- can run. The four roles below, their grants and every policy on `event` are this
-- file's.
--
-- WHO RUNS IT. The o11y application, as itself, under `schema`: no person, no
-- SuperAdmin, no password. `schema` may create exactly these four roles, may grant only
-- SELECT and INSERT on `event`, and may create policies only on `event`, so this file
-- cannot reach anything outside the plane it owns.
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
-- without anyone remembering to extend this file. Permissive policies are OR-ed, so the
-- `USING 1` policies of superadmin and schema widen their own view to every row and
-- nobody else's. schema needs it: a migration that backfills reads every org's rows,
-- and under `tenant` alone it would read none and copy nothing, silently.
--
-- NULL, NOT ''. currentOrg() is NULL for a bearer with no membership set, and for every
-- login that is not an IAM token. `org = NULL` matches nothing, so such a caller sees no
-- rows, including the unattributed rows the metric tables store under ''.
--
-- WHAT A ROW POLICY DOES NOT COVER. It filters reads. The two writers insert rows for
-- every org because they are the platform's own ingest; that is what `sink` and
-- `writer` are for, and why neither is granted SELECT. A caller that logs in with a
-- password rather than a token holds none of these roles and is not confined here
-- (access_control_improvements.users_without_row_policies_can_read_rows); the end state
-- is that no such caller exists.
--
-- ORDER. Land it with, or after, the warehouse image that has the IAM directory,
-- currentOrg() and the two bootstrap roles. The statements are idempotent.

CREATE ROLE IF NOT EXISTS tenant;
CREATE ROLE IF NOT EXISTS reader;
CREATE ROLE IF NOT EXISTS writer;
CREATE ROLE IF NOT EXISTS sink;

GRANT SELECT ON event.* TO reader;
GRANT INSERT ON event.* TO writer;
GRANT INSERT ON event.fact TO sink;

CREATE ROW POLICY OR REPLACE tenant ON event.* USING org = currentOrg() AS PERMISSIVE TO tenant;
CREATE ROW POLICY OR REPLACE superadmin ON event.* USING 1 AS PERMISSIVE TO superadmin;
CREATE ROW POLICY OR REPLACE schema ON event.* USING 1 AS PERMISSIVE TO schema;

-- The roles exist and the namespace carries exactly the three policies above.
SELECT throwIf(
    (SELECT count() FROM system.roles
      WHERE name IN ('tenant', 'reader', 'writer', 'sink', 'schema', 'superadmin')) != 6
 OR (SELECT count() FROM system.row_policies
      WHERE database = 'event' AND table = ''
        AND ((short_name = 'tenant' AND select_filter = 'org = currentOrg()' AND apply_to_list = ['tenant'])
          OR (short_name = 'superadmin' AND select_filter = '1' AND apply_to_list = ['superadmin'])
          OR (short_name = 'schema' AND select_filter = '1' AND apply_to_list = ['schema']))) != 3,
    'The IAM roles or the event.* row policies are not as this file declares them.') AS ok;
