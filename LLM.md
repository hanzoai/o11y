# o11y

<h1 align="center" style="border-bottom: none">
    <a href="https://o11y.hanzo.ai" target="_blank">
        <img alt="Hanzo O11y" src="https://github.com/user-attachments/assets/ef9a33f7-12d7-4c94-8908-0a02b22f0c18" width="100" height="100">
    </a>
    <br>Hanzo O11y
</h1>

Hanzo's observability spine — OTLP in, Hanzo Datastore (ClickHouse OLAP) on disk,
serving o11y.hanzo.ai. A clean full fork of **latest O11y** (synced to upstream
`main`), building green (`go build ./...` = 0), MIT-only (no `ee/`).

## Dependency ownership (fork boundary)

All O11y-branded platform deps are OWNED as public `hanzoai/*` forks — never
consumed as upstream-branded modules. Do NOT reintroduce `github.com/SigNoz/*`.

| upstream | fork | tag pinned | fork module path |
|---|---|---|---|
| SigNoz/signoz-otel-collector | hanzoai/otel-collector | v0.144.7 | `github.com/hanzoai/otel-collector` (renamed) |
| SigNoz/clickhouse-go-mock | hanzoai/clickhouse-go-mock | v0.14.1 | `github.com/hanzoai/clickhouse-go-mock` (renamed) |
| SigNoz/govaluate | hanzoai/govaluate | v0.1.0 | `github.com/hanzoai/govaluate` (renamed) |
| SigNoz/expr | hanzoai/expr | v1.17.8 | `github.com/expr-lang/expr` (KEEP upstream path — consumed via `replace`) |
| SigNoz/signoz | hanzoai/o11y | synced to upstream `main` (3e6339019) | owned base; `pkg/ cmd/` re-synced wholesale to latest, `ee/` stripped (MIT-only) |

The vendored O11y source in `pkg/ ee/ cmd/` self-references as
`github.com/hanzoai/o11y/...` (NOT `github.com/SigNoz/signoz/...`). Generic
ecosystem libs (`golang.org/x`, `google.golang.org`, prometheus, otel, gin,
cobra, `luxfi/*`) are NOT forked — only the O11y platform.

## Build

`go build ./...` = 0 on `main`. The tree is a clean fork of latest O11y — the
prior "missing 22 packages" gap was a version-skew artifact (a franken-fork mixing
O11y eras); it dissolved when the whole `pkg/ cmd/` tree was re-synced to one
consistent upstream version. Keep it that way: bump by re-syncing to a newer O11y
`main`, not by piecemeal-porting individual packages.

The real server binary is `./cmd/community` (NOT `./cmd/server`, which does not
exist). Build check: `GOPRIVATE='github.com/hanzoai/*' GOSUMDB=off go build ./cmd/community`.

### go.mod is paired with the ci.yaml cloud pin — bump BOTH or CI goes red

Root `mount.go` imports `github.com/hanzoai/cloud`, and go.mod resolves it via
`replace => ../cloud`. There is no such sibling in CI, so `ci.yaml` checks one out
**at an exact commit** (`ref:` under "Checkout hanzoai/cloud") — deliberately, since
cloud's `main` drifts independently. That makes go.mod and the ci.yaml ref **ONE fact
in two places**: touch either alone and `go build ./...` dies with the misleading
`go: updates to go.mod needed; to update it: go mod tidy`. This kept CI red for weeks.

To re-tidy: `go mod tidy` against the sibling, then **bump the ci.yaml `ref` to the
same cloud commit in the same PR**. Two traps:

- Tidy resolves against **whatever `../cloud` is checked out right now** — that clone
  is a working copy, often on someone's feature branch, and it moves under you. Derive
  versions from the commit you are pinning (`git -C ../cloud show <sha>:go.mod`), not
  from the branch that happens to be there. Same class of hazard as the stale luxfi
  clones.
- The resulting version jumps are **not upgrade decisions** — MVS forces them, because
  cloud already requires them. Do not "fix" them back down; re-pair the two sides.

Since a green build is NOT sufficient evidence (`hanzoai/zip` → `zap-proto/zip` can
boot-panic while CI stays green), smoke-test the binary: it must reach
`Query server started listening on 0.0.0.0:8080`. `TestEmailRejected`
(`alertmanagernotify/email`) fails on any go.mod — upstream string mismatch; ci.yaml
already `-skip`s it by name.

## Container image (`ghcr.io/hanzoai/o11y`)

Root `Dockerfile` + `.github/workflows/docker.yaml` build a standalone community
server image on push to `main` and `v*` tags → `ghcr.io/hanzoai/o11y:<sha>` (+ `:main`).
This replaces an unrelated upstream image that previously squatted the tags.

- Builds ONLY `./cmd/community`. It does NOT import `github.com/hanzoai/cloud`, so
  go.mod's `replace github.com/hanzoai/cloud => ../cloud` is inert for the image and
  no cloud sibling is checked out. (Cloud lives only in root `mount.go`, compiled by
  the `go build ./...` CI job — so `ci.yaml` still needs the sibling; the container
  does not.) A bare `go mod download` WOULD fail (cloud→../cloud); build the one pkg.
- All external deps in its graph are PUBLIC hanzoai/* forks (otel-collector,
  govaluate, clickhouse-go-mock, expr) → no private git auth, just
  `GOPRIVATE=github.com/hanzoai/* GOSUMDB=off`. GHCR push uses `GH_PAT || GITHUB_TOKEN`
  (the package is linked to hanzoai/o11y, so GITHUB_TOKEN+`packages: write` suffices).
- Runs headless: `O11Y_WEB_ENABLED=false`. `routerweb` os.Stat()s its web dir at
  boot and fatals if missing; the SPA is served by hanzoai/static at the edge. The
  `frontend/` tree is NOT bundled — its `pnpm-lock.yaml` is STALE vs `package.json`
  (mid rolldown-vite/oxlint migration), fails `--frozen-lockfile`. TODO: regen lockfile.
- Listens on `0.0.0.0:8080` (constants.HTTPHostPort); sqlstore default = sqlite at
  `/var/lib/o11y/o11y.db`; needs an external ClickHouse for telemetry.

Boot fix: `pkg/instrumentation/sdk.go` hard-pinned `semconv/v1.40.0.SchemaURL`,
which contrib `NewSDK` merges against `resource.Default()` (schema 1.41.0, from the
re-synced otel/sdk) — OTEL rejects differing non-empty schema URLs, so `o11y.New`
crashed at boot with "conflicting Schema URL". Now sourced from the detected resource
(`resource.SchemaURL()`), version-agnostic. Verified: boots, runs migrations, serves :8080.

## Hanzo layer to re-apply on the green base

The re-sync reverted these Hanzo-original packages to O11y canonical to kill the
skew; re-layer them on top (they live in git history at `c9ab975`): `pkg/authz/iamauthz`
(Hanzo IAM authz — REQUIRED per the house "always use Hanzo IAM" rule; O11y's
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
through them. `apikeyidentn` (service-account API keys, `O11Y-API-KEY`) stays for
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

## No third-party trackers — analytics is Insights, support chat is Hanzo Chat

o11y ships **zero** third-party SaaS trackers. The upstream fork wired in product
analytics, onboarding tours and a support-chat widget, and the frontend build gated
each on `VITE_*_ENABLED !== 'false'` — **opt-OUT**, so an operator who set nothing
shipped all three: a self-hosted observability tool phoning home to third parties with
its users' data, `index.html` injecting vendor `<script>` tags on every page load, and
the chat block HMAC-hashing the logged-in user's email for the vendor. All removed
(`web.Settings`/`SettingsConfig`, `docs/config/web-settings.json`, index.html, vite
defines, window typings). Do not reintroduce them.

**Sentry stays** — our own fork (`hanzoai/sentry`) — and is **opt-IN** (`=== 'true'`).
Product analytics is Hanzo Insights (`@hanzo/insights`, first-party). `logEvent` →
`/event` on our own backend is likewise first-party; it is not a tracker.

`frontend/src/types/generated/webSettings.ts` is GENERATED from
`docs/config/web-settings.json` — edit the schema and run
`pnpm generate:config:web-settings`, never hand-edit the `.ts`.

Support chat is **one way**: `utils/supportChat.ts` → `openSupportChat()` (Hanzo Chat).
All call sites (SideNav, Support, LaunchChatSupport) route through it — no per-component
chat integration. It delegates to `utils/navigation.ts` → `openInNewTab`, which the
repo's own `o11y/no-raw-absolute-path` lint rule mandates over a bare `window.open`
(`withBasePath` passes external URLs through untouched). `openInNewTab` passes
`noopener`: `window.open`, unlike `<a target="_blank">`, does not imply it, and without
it every opened tab can navigate us back via `window.opener` (reverse tabnabbing).

## Config env naming (debranded — no O11Y/CLICKHOUSE)

Operator-facing env vars and config keys are Hanzo-branded, never SigNoz/ClickHouse.
Naming-only: the store is still ClickHouse wire-protocol under the hood — the
`clickhouse-go` driver, `db.system=clickhouse` semconv, and the `clickhouse`
telemetrystore provider registration all stay (implementation, not surface).

- **Env prefix `O11Y_`** (was `O11Y_`): set once at `pkg/config/envprovider/provider.go`
  (`prefix`). The koanf env provider derives every structured key from it. Single `_`
  is the `::` path delimiter, double `__` is a literal `_`.
- **Store key segment `datastore`** (was `clickhouse`): the `mapstructure` tag on
  `telemetrystore.Config.Clickhouse` is `datastore` (the Go field/type keep the
  `Clickhouse` name — internal). YAML key `telemetrystore.datastore`. Provider
  selector value stays `clickhouse`.
- **Canonical DSN key `O11Y_DATASTORE_DSN`** (flat — THE operator knob): wired as an
  override alias in `pkg/o11y/config.go` (`mergeAndEnsureBackwardCompatibility`),
  mapping into `telemetrystore.datastore.dsn`. Value → Hanzo Datastore
  (`tcp://datastore.hanzo.svc:9000/?database=o11y`, set at deploy time). It takes
  precedence over the structured `O11Y_TELEMETRYSTORE_DATASTORE_DSN`, which stays as
  an internal fallback (don't document/set the long form). Keep operator knobs flat and
  short — no `O11Y_A_B_C_D` compounds where a flat alias reads better.
- **Legacy override aliases** in `pkg/o11y/config.go` (`mergeAndEnsureBackwardCompatibility`)
  debranded too: `O11Y_LOCAL_DB_PATH`, `DatastoreUrl`, `O11Y_SAAS_SEGMENT_KEY`,
  `O11Y_JWT_SECRET`; and headless web via `O11Y_WEB_ENABLED` / `O11Y_WEB_DIRECTORY`.
- **Deliberately NOT renamed** (implementation / not app-config surface): the `clickhouse`
  provider value + `MustNewName("clickhouse")` registration, `clickhouse-go` driver + mock,
  SQL/table/DB names (`o11y_traces` etc.) and query-template vars
  (`O11Y_START_TIME`/`O11Y_END_TIME`), the ClickHouse **server** container in `deploy/`
  (service/volume names, its own `CLICKHOUSE_SKIP_USER_SETUP` env), the `O11Y_E2E_*`
  Playwright test-harness vars (separate `tests/e2e` subsystem). The
  `O11Y_OTEL_COLLECTOR_DATASTORE_*` keys in `deploy/` are consumed by the
  `hanzoai/otel-collector` fork — renamed here for consistency; that fork must
  accept the `_DATASTORE_` segment (coordinated cross-repo rename).

## Native datastore metrics driver (`pkg/datastoremetrics`) — the fork unblock

- **What**: the o11y-native write path for metrics. `Writer.WriteMetrics` satisfies
  `zapmetricreceiver.Handler`, decoding a ZAP `MetricBatch` straight into the datastore
  `time_series_v4` + `samples_v4` tables over **upstream** `clickhouse-go` v2 (via
  `telemetrystore.TelemetryStore.ClickhouseDB()`), reusing the `telemetrymetrics`
  table-name constants so a series written here is immediately queryable.
- **Why it exists**: the stock `o11yclickhousemetrics` exporter serialises OTLP
  exponential histograms as a DDSketch into `exp_hist.sketch`, which needs the FORKED
  ch-go (`proto.DD/.Store/.IndexMapping`). That fork conflicts with the upstream ch-go
  the query plane pins — the reason metrics ingest could not move in-process. This driver
  sidesteps the fork: the ZAP wire carries CLASSIC Prometheus shapes, so histograms
  decompose into `<name>.bucket{le=…}` / `.count` / `.sum` samples and summaries into
  `.quantile{quantile=…}` — no sketch, no `exp_hist`, NO fork. Only the two tables the
  query plane already reads.
- **Fingerprint parity**: the labels→fingerprint hash (FNV-1a, hierarchical
  resource→scope→point, salted with `__name__`) is ported verbatim from the fork's
  `internal/common/fingerprint` (which is import-walled), so join keys are byte-identical
  and the existing reader joins samples to series unchanged. Column strings match too:
  temporality `Cumulative`/`Unspecified`, type `Sum`/`Gauge`/`Histogram`/`Summary`.
- **Wire-in**: `datastoremetrics.NewWriter(store.ClickhouseDB())` +
  `zapmetricreceiver.New(Config{OnBatch: w.WriteMetrics})`. Consumed by `hanzoai/cloud`'s
  embedded o11y runtime (opt-in `O11Y_METRICS_ZAP_LISTEN`, fail-soft) so the standalone
  `otel-collector` metrics path can later repoint to cloud (verify-then-cutover).
- **Boundary unchanged**: the physical `o11y_metrics.*_v4` names still live in
  `telemetrymetrics` (the source of truth this driver reuses) — renaming them is a
  datastore migration, out of scope here.
