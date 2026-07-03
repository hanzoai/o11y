# o11y

<h1 align="center" style="border-bottom: none">
    <a href="https://o11y.hanzo.ai" target="_blank">
        <img alt="Hanzo O11y" src="https://github.com/user-attachments/assets/ef9a33f7-12d7-4c94-8908-0a02b22f0c18" width="100" height="100">
    </a>
    <br>Hanzo O11y
</h1>

Hanzo's observability spine — OTLP in, Hanzo Datastore (ClickHouse OLAP) on disk,
serving o11y.hanzo.ai. A clean full fork of **latest SigNoz** (synced to upstream
`main`), building green (`go build ./...` = 0), MIT-only (no `ee/`).

## Dependency ownership (fork boundary)

All SigNoz-branded platform deps are OWNED as public `hanzoai/*` forks — never
consumed as upstream-branded modules. Do NOT reintroduce `github.com/SigNoz/*`.

| upstream | fork | tag pinned | fork module path |
|---|---|---|---|
| SigNoz/signoz-otel-collector | hanzoai/signoz-otel-collector | v0.144.6 | `github.com/hanzoai/signoz-otel-collector` (renamed) |
| SigNoz/clickhouse-go-mock | hanzoai/clickhouse-go-mock | v0.14.1 | `github.com/hanzoai/clickhouse-go-mock` (renamed) |
| SigNoz/govaluate | hanzoai/govaluate | v0.1.0 | `github.com/hanzoai/govaluate` (renamed) |
| SigNoz/expr | hanzoai/expr | v1.17.8 | `github.com/expr-lang/expr` (KEEP upstream path — consumed via `replace`) |
| SigNoz/signoz | hanzoai/signoz | synced to upstream `main` (3e6339019) | owned base; `pkg/ cmd/` re-synced wholesale to latest, `ee/` stripped (MIT-only) |

The vendored SigNoz source in `pkg/ ee/ cmd/` self-references as
`github.com/hanzoai/o11y/...` (NOT `github.com/SigNoz/signoz/...`). Generic
ecosystem libs (`golang.org/x`, `google.golang.org`, prometheus, otel, gin,
cobra, `luxfi/*`) are NOT forked — only the SigNoz platform.

## Build

`go build ./...` = 0 on `main`. The tree is a clean fork of latest SigNoz — the
prior "missing 22 packages" gap was a version-skew artifact (a franken-fork mixing
SigNoz eras); it dissolved when the whole `pkg/ cmd/` tree was re-synced to one
consistent upstream version. Keep it that way: bump by re-syncing to a newer SigNoz
`main`, not by piecemeal-porting individual packages.

The real server binary is `./cmd/community` (NOT `./cmd/server`, which does not
exist). Build check: `GOPRIVATE='github.com/hanzoai/*' GOSUMDB=off go build ./cmd/community`.

## Container image (`ghcr.io/hanzoai/o11y`)

Root `Dockerfile` + `.github/workflows/docker.yaml` build a standalone community
server image on push to `main` and `v*` tags → `ghcr.io/hanzoai/o11y:<sha>` (+ `:main`).
This replaces an unrelated upstream image that previously squatted the tags.

- Builds ONLY `./cmd/community`. It does NOT import `github.com/hanzoai/cloud`, so
  go.mod's `replace github.com/hanzoai/cloud => ../cloud` is inert for the image and
  no cloud sibling is checked out. (Cloud lives only in root `mount.go`, compiled by
  the `go build ./...` CI job — so `ci.yaml` still needs the sibling; the container
  does not.) A bare `go mod download` WOULD fail (cloud→../cloud); build the one pkg.
- All external deps in its graph are PUBLIC hanzoai/* forks (signoz-otel-collector,
  govaluate, clickhouse-go-mock, expr) → no private git auth, just
  `GOPRIVATE=github.com/hanzoai/* GOSUMDB=off`. GHCR push uses `GH_PAT || GITHUB_TOKEN`
  (the package is linked to hanzoai/o11y, so GITHUB_TOKEN+`packages: write` suffices).
- Runs headless: `O11Y_WEB_ENABLED=false`. `routerweb` os.Stat()s its web dir at
  boot and fatals if missing; the SPA is served by hanzoai/static at the edge. The
  `frontend/` tree is NOT bundled — its `pnpm-lock.yaml` is STALE vs `package.json`
  (mid rolldown-vite/oxlint migration), fails `--frozen-lockfile`. TODO: regen lockfile.
- Listens on `0.0.0.0:8080` (constants.HTTPHostPort); sqlstore default = sqlite at
  `/var/lib/signoz/signoz.db`; needs an external ClickHouse for telemetry.

Boot fix: `pkg/instrumentation/sdk.go` hard-pinned `semconv/v1.40.0.SchemaURL`,
which contrib `NewSDK` merges against `resource.Default()` (schema 1.41.0, from the
re-synced otel/sdk) — OTEL rejects differing non-empty schema URLs, so `signoz.New`
crashed at boot with "conflicting Schema URL". Now sourced from the detected resource
(`resource.SchemaURL()`), version-agnostic. Verified: boots, runs migrations, serves :8080.

## Hanzo layer to re-apply on the green base

The re-sync reverted these Hanzo-original packages to SigNoz canonical to kill the
skew; re-layer them on top (they live in git history at `c9ab975`): `pkg/authz/iamauthz`
(Hanzo IAM authz — REQUIRED per the house "always use Hanzo IAM" rule; SigNoz's
OpenFGA is the current default and must be replaced), `pkg/zapreceiver` +
`pkg/zapmetricreceiver` (ZAP-native OTLP receivers), `pkg/billing` + `pkg/types/billingtypes`.
Preserved through the sync: module path, `mount.go` (the `cloud.Register`/zip mount
adapter), `NOTICE`, `LICENSE`.

## Zero-onboarding identity — Hanzo IAM session only (no native auth)

o11y holds **no identity of its own**: no native users, no login/register/invite,
no first-run "setup" wizard, no token minting. It is a pure resource server that
trusts the **Hanzo IAM session** exactly like every other Hanzo service (cloud,
ai, commerce) — one IAM validates once at the edge, every service trusts it.

**The seam is `pkg/identn/iamidentn`** (pairs with `pkg/authz/iamauthz`). It is the
primary human identN resolver (registered first in `pkg/identn/resolver.go`,
`identn.iam.enabled=true` by default). It reads the identity headers the edge
gateway (`hanzoai/gateway`) injects after it validates the hanzo.id JWT against
IAM's JWKS and strips any client-supplied copies (HIP-0026):

- `X-Org-Id`     — JWT `owner` claim (org **slug**, e.g. `hanzo`) → the tenant
- `X-User-Id`    — JWT `sub` (user UUID)
- `X-User-Email` — email

On first sight of an `(org, user)` pair it **auto-provisions the tenant with zero
onboarding**: the o11y org row is created if absent (`types.NewOrganizationWithID`
+ managed roles + default configs, mirroring first-user creation) and the user is
granted the org-scoped admin role in Hanzo IAM. The Hanzo org slug maps to a
stable o11y org UUID via UUIDv5 (`iamidentn.toUUID`), so the mapping is
deterministic and stateless. Authorization is unchanged — **every access check is
still an iamauthz batch-enforce**; the founding grant just makes a logged-in Hanzo
user authorized for their own org. Cross-org is denied by org scoping
(`claims.OrgID` drives every data query).

**The setup gate is gone.** `NewAPIHandler` (`pkg/query-service/app/http_handler.go`)
now sets `SetupCompleted = true` unconditionally — no org/user counting, no root
gate. So `/api/v1/register` is inert ("self-registration is disabled") and the SPA
never shows an onboarding wizard. Native login/session/invite endpoints remain
compiled but are dead: no native users are ever created, so nothing can log in
through them. `apikeyidentn` (service-account API keys, `SIGNOZ-API-KEY`) stays for
machine/OTLP identity — that is not human auth.

**Security boundary (important):** trusting `X-Org-Id`/`X-User-Id` is safe **only
because o11y sits behind `hanzoai/gateway`**, which is the sole authority that sets
those headers (HIP-0026) and strips client copies. o11y must be reachable only via
the gateway (ClusterIP + network policy), exactly like cloud/ai/commerce. Default
sharder is `noop` (owns all org keys), so dynamically-provisioned tenants pass the
identn middleware's `IsMyOwnedKey` check — do **not** switch to `singlesharder`
(it owns one org key and would 403 every other tenant). Multi-tenancy: SQL-stored
data (llmobs observations/scores/sessions, dashboards, …) is scoped by
`claims.OrgID`; ClickHouse telemetry is isolated only insofar as the emit path tags
the same org id via resource attributes.

## Config env naming (debranded — no SIGNOZ/CLICKHOUSE)

Operator-facing env vars and config keys are Hanzo-branded, never SigNoz/ClickHouse.
Naming-only: the store is still ClickHouse wire-protocol under the hood — the
`clickhouse-go` driver, `db.system=clickhouse` semconv, and the `clickhouse`
telemetrystore provider registration all stay (implementation, not surface).

- **Env prefix `O11Y_`** (was `SIGNOZ_`): set once at `pkg/config/envprovider/provider.go`
  (`prefix`). The koanf env provider derives every structured key from it. Single `_`
  is the `::` path delimiter, double `__` is a literal `_`.
- **Store key segment `datastore`** (was `clickhouse`): the `mapstructure` tag on
  `telemetrystore.Config.Clickhouse` is `datastore` (the Go field/type keep the
  `Clickhouse` name — internal). YAML key `telemetrystore.datastore`. Provider
  selector value stays `clickhouse`.
- **Canonical DSN key `O11Y_DATASTORE_DSN`** (flat — THE operator knob): wired as an
  override alias in `pkg/signoz/config.go` (`mergeAndEnsureBackwardCompatibility`),
  mapping into `telemetrystore.datastore.dsn`. Value → Hanzo Datastore
  (`tcp://datastore.hanzo.svc:9000/?database=o11y`, set at deploy time). It takes
  precedence over the structured `O11Y_TELEMETRYSTORE_DATASTORE_DSN`, which stays as
  an internal fallback (don't document/set the long form). Keep operator knobs flat and
  short — no `O11Y_A_B_C_D` compounds where a flat alias reads better.
- **Legacy override aliases** in `pkg/signoz/config.go` (`mergeAndEnsureBackwardCompatibility`)
  debranded too: `O11Y_LOCAL_DB_PATH`, `DatastoreUrl`, `O11Y_SAAS_SEGMENT_KEY`,
  `O11Y_JWT_SECRET`; and headless web via `O11Y_WEB_ENABLED` / `O11Y_WEB_DIRECTORY`.
- **Deliberately NOT renamed** (implementation / not app-config surface): the `clickhouse`
  provider value + `MustNewName("clickhouse")` registration, `clickhouse-go` driver + mock,
  SQL/table/DB names (`signoz_traces` etc.) and query-template vars
  (`SIGNOZ_START_TIME`/`SIGNOZ_END_TIME`), the ClickHouse **server** container in `deploy/`
  (service/volume names, its own `CLICKHOUSE_SKIP_USER_SETUP` env), the `SIGNOZ_E2E_*`
  Playwright test-harness vars (separate `tests/e2e` subsystem). The
  `O11Y_OTEL_COLLECTOR_DATASTORE_*` keys in `deploy/` are consumed by the
  `hanzoai/signoz-otel-collector` fork — renamed here for consistency; that fork must
  accept the `_DATASTORE_` segment (coordinated cross-repo rename).
