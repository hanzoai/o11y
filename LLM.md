# o11y

<h1 align="center" style="border-bottom: none">
    <a href="https://o11y.hanzo.ai" target="_blank">
        <img alt="Hanzo O11y" src="https://github.com/user-attachments/assets/ef9a33f7-12d7-4c94-8908-0a02b22f0c18" width="100" height="100">
    </a>
    <br>Hanzo O11y
</h1>

Hanzo's observability spine — OTLP in, Hanzo Datastore (columnar OLAP) on disk,
serving o11y.hanzo.ai. A clean full fork of **latest O11y** (synced to upstream
`main`), building green (`go build ./...` = 0), MIT-only (no `ee/`).

## How this ships

One way, and it runs on our own stack:

    push  ->  github.com/hanzoai/o11y         (a mirror)
      ->  git.hanzo.ai/hanzoai/o11y            CANONICAL
              .hanzo/workflows/ci.yaml         go build + go test
              .hanzo/workflows/docker.yaml     builds ghcr.io/hanzoai/o11y
              .hanzo/workflows/docker-site.yaml builds ghcr.io/hanzoai/o11y-site
      ->  hanzoai/universe crs/o11y-site.yaml  names the tag that is live
      ->  hanzoai/operator                     reconciles the App
      ->  hanzoai/static behind hanzoai/ingress serves the SPA

**git.hanzo.ai is canonical; GitHub is a mirror.** Every build, check and deploy
is a workflow under `.hanzo/workflows/`, which the forge reads. `.hanzo/workflows`
uses GitHub Actions syntax, so a workflow moves between the two by changing
directory and nothing else. There is no `.github/workflows/` — refs reach the
forge by push, not by an Actions job.

No GitHub Pages and no Cloudflare Pages. `Dockerfile.site` already ends at
`FROM ghcr.io/hanzoai/static:v0.5.1` serving `frontend/build` on :3000, so the SPA
is an image the operator runs, like every other Hanzo surface.

These three workflows were briefly deleted from `.github/workflows/` with no
replacement anywhere, which left the promoted `o11y-site` App with **nothing that
could build its image** — and no failing run to show it, because a workflow that
does not exist cannot go red. They are restored here at their canonical path. That
failure mode is the whole reason a migration is `git mv` and never a delete.

Also load-bearing via the **Go module tag**: `hanzoai/cloud` `go.mod` pins
`github.com/hanzoai/o11y`, because the query plane now lives in cloud at `/v1/o11y`.

## The document — ONE emitter, and it is the typed op registry

`identity.go` (45) + `infra.go` (44) + `logs.go` (9) + `telemetry.go` (5) = **103
typed ops**, registered on the `cloud.Router` this module is handed at their full
public `/v1/o11y/...` paths. Cloud's `describe.go` reads that registry from a real
mount and weaves the result into the one published document. Change a struct and
the document follows; there is nothing to keep in sync.

There was a second emitter and it is **deleted**: `cmd/openapi.go` ran a swaggest
reflector over the runtime's gorilla/mux tree and wrote a committed 629 KB
`docs/api/openapi.yml` — 120 paths, 174 ops, spelled `/api/v1` ×59, `/api/v2` ×57,
`/api/v3`, `/api/v4`, `/api/v5` ×2, and **zero** `/v1/o11y`. It described the
fork's internal tree behind the delegation wildcard's `/v1/o11y/* -> /api/*`
rewrite, so it documented a surface no customer can address, in a shape that
breaks two house rules on its face. No CI regenerated or verified it, so it could
only drift. Gone with it: `pkg/o11y/openapi.go` (the reflector — unreachable once
the command went) and `registerGenerateOpenAPI` from `cmd/generate.go`.

**If a route is not in the typed registry it is in no document.** That is the only
place to fix it; do not add a second writer.

## The console (`pkg/web`) is router-agnostic

`web.Web` is `http.Handler` and nothing else. It used to also carry
`AddToRouter(*mux.Router)` — the console's mounting rule braided into one
router's type, and the last reason `pkg/web` imported gorilla. It bought nothing:
every host registered the same terminal catch-all, so the rule was the HOST's.
The host now spells its own one line, and both hosts spell the SAME one —
`app.All("/*", zip.AdaptNetHTTP(web))` in `pkg/query-service/app/server.go` and
on a zip host (the `hanzoai/cloud` `webui` idiom).

Serving is stdlib (`http.FileServer` + the shell rendered once at boot), NOT
`zip.Static`: `zip.Static` serves bytes out of an `fs.FS`, and the shell is not a
file in the tree — it is templated at startup with this deployment's base href
and settings, so `WithIndex`/`WithFallback` would hand the browser a raw
`index.html` with no boot data. One handler serves both host kinds identically.

`noopweb` (`web.enabled=false`, the shipped headless image) answers **404** —
the bytes a default `NotFoundHandler` writes when a null provider registers no
route at all — so the host mounts the console unconditionally.

The console refuses `/v1` and `/ws` (`routerweb.apiPrefixes`, segment-exact),
because a router that resumes its parent when a prefix matches but no route does
sent **every miss inside `/v1/o11y` through to the console as 200 `text/html`** —
JSON clients got the SPA shell.

## One router: `pkg/http/routing`

**There is no web framework in this service but zip.** `gorilla/mux`, `otelmux`,
`gin`, `gorilla/handlers` and `rs/cors` are all out of `go.mod`; every route the
runtime serves is a zip route on a zip group, matched by one router with one param
model and one middleware chain.

`routing.Router` is the ONE registration surface — `Get/Post/Put/Patch/Delete`,
`Handle(method, path, h)`, `Group(prefix)` — and it takes an `http.Handler`, so
the 366 handler bodies did not change shape. It exists because a registration
carries three facts zip has no slot for, each of which the old tree RECOVERED per
request by asking the router what it had just matched:

| fact | old | now |
|---|---|---|
| bound path segments | `mux.Vars(req)` in 140 handler bodies | bound at the leaf, read once via `coretypes.Param` |
| the path TEMPLATE (audit keys, span names) | `mux.CurrentRoute(req).GetPathTemplate()` | recorded at registration, read via `coretypes.RoutePath` |
| the route's declared resources | `route.GetHandler()` + type assertion, in TWO packages | handed to `middleware.Resource.For(defs)` by the registrar |

Paths are declared in the public brace spelling (`/v1/o11y/rules/{id}`) and
respelled for the router in one function; a constrained segment
(`/v1/event/{project:guid}/envelope/`) becomes fiber's own `guid` — a UUID parse,
replacing a hand-written character class. A **duplicate registration panics at
boot**; the tree this replaces silently let the first one win.

**Count the table.** `routing.Table` is the census of what registration actually
did: **233** routes from `pkg/apiserver/o11yapiserver` + **133** from
`pkg/query-service/app` = **366**, the same 366 the mount publishes (353 typed ops
= 353 OpenAPI operations = 353 MCP tools, 10 hatches, 3 probes). This repo has
shipped the other outcome — 83 typed ops in files nothing called, building green
and serving nothing — so the arithmetic is a test, not a comment
(`wantAPIServerRoutes` in `pkg/apiserver/o11yapiserver/routes_test.go`,
`wantQueryServiceRoutes` in `pkg/query-service/app/address_test.go`, and 366 in
`routes_test.go`). All three terms are asserted, because the sum is what turns
"registered ⊆ declared, on both halves" into set equality — the query-service term was a sentence
for one release, and a sentence cannot fail.

## The table IS the seam — `Table.Handler`, and there is no second registration

The service's surface used to be stated twice over: once here as the
implementation, and once in this module's own declaration of it (the 353 typed
ops), which named the same 366 addresses with an input, an output and their
prose. The declaration had to REACH the implementation, and the only way it could
was to speak HTTP to the WHOLE service — build a request, hand it to one
`http.Handler` that was the entire router, and let that router match the path a
second time to find the handler the declaration had already named. An in-process
round trip, across a `net/http`↔fasthttp bridge, through an `httptest` recorder,
353 times over.

A registration is a map from address to handler. `Table.Handler(method, path)`
reads it as one, and `o11y.SetRuntime` hands the whole table over
(`server.go`'s `published.SetRuntime(s.routes)`). What that deletes:

- **the second match**, and with it the entire failure class the reachability
  census exists for — a request cannot lose its route to a router that is never
  consulted;
- **the silence**. A declared address no registration answers is now a `nil` at
  the seam, sayable at boot, instead of the runtime's own 404 — indistinguishable
  from a caller's typo and discovered by a customer. The two sides HAD drifted:
  three addresses the declaration names `{traceId}` were registered `{traceID}`,
  invisible because a router matches by POSITION;
- **the 503 in the process that has all the handlers.** The standalone server
  could not install itself as that one handler without an op relaying into the
  router already serving it, so it installed nothing — and every op on its `/mcp`
  surface and its by-name call plane answered 503 with all 366 handlers one map
  lookup away. It now installs its own table.

The address is stated ONCE, at the registration, and rides the call
(`address.go`'s `addressed` → `zip.Address`); `relay` takes no method and no path.
That retired 706 spellings of 366 addresses down to 366. A host whose runtime is
in ANOTHER process installs `o11y.Whole(proxy)` — one door, honestly, because for
a remote every address really does resolve to the same door.

Reached BY NAME there is no router in front of the handler, so the two chain
members that need the route (`Audit` keys on the template, `Resource` resolves the
declared resources) would read an empty one. `routing.addressed` carries the
template with the handler it was registered at and writes it via
`coretypes.SetRoute` — the same representation `bind` writes for a matched
request, one read on the other side.

**The chain is composed at the LEAF, not registered as ambient middleware.** Two
of its members need the route: Audit keys on the template, Resource resolves the
route's declared resources. Ambient middleware runs before the router has matched
anything, so both would read an empty route. Composing per leaf reproduces the
order the old tree ran in — match, then middleware, then handler.

**Serving.** The standalone server hands its listener straight to the app
(`app.Fiber().Listener(conn)`), so streams and the `/v1/o11y/query_progress` upgrade
run native fasthttp. `PublicHandler()` is the embedding host's door and bridges to
`net/http`: streamed answers pump through it, a connection HIJACK does not, so a
host that needs the websocket serves the listener rather than embedding it.

**Three `zip.AdaptNetHTTP` remain on served paths, and each is one because its
FAR SIDE is net/http** (the others in a grep are tests standing in for a
net/http host). `routing.go`'s leaf — the 366 handler bodies and the chain around
them are `http.Handler`, and `Table.Handler` hands the SAME value to a by-name
caller, so a native leaf would be a second spelling of every route. The console
catch-all — `web.Web` is `http.Handler` and nothing else, deliberately, so one
value serves the net/http chain here and a zip route in cloud. And `claim.go`'s
`bridge` — a host installs an `http.Handler` runtime (`SetRuntime`) and a
net/http `factory.Handler` (`SetHealth`), so there is nothing native to call.
That bridge is now the package's only one: `probe` used to build an adapter
*inside its own handler*, so every liveness poll allocated one and re-wrote zip's
terminal marker into a process-wide map.

The fourth was not on a net/http far side at all. `publish` reached the
declaration app through `zip.AdaptNetHTTP(adaptor.FiberApp(d.Fiber()))` — out
through `net/http` and back into a router that is native at both ends, copying
method, URI, host and every header into a second pooled `fasthttp.Request`,
the body twice, `r.RemoteAddr` through `net.ResolveTCPAddr`, and every response
header back one at a time. `dispatch` hands the live request to that router,
which is what a listener does with one. Composing instead (`app.Use(d)`) is
refused at build — both apps claim the same 367 addresses, and the refusal names
the first one (`TestTheDeclarationCannotBeComposedIn`). The five control paths'
whole answer is pinned byte for byte against the declaration app's own answer in
`TestPublishServesTheDeclarationVerbatim`.

Measured while pinning those answers, unrelated to the conversion and NOT fixed:
`app.Fiber().Use(...)` at `server.go`'s compress and CORS is inert on this zip
version. `Fiber()` builds a generation, and the routes registered after it build
another that the app then serves, so neither an ordinary route nor a control
route sees either middleware — no `Access-Control-Allow-Origin` and no
`Content-Encoding` on any answer, in either registration order.

## Dependency ownership (fork boundary)

All O11y-branded platform deps are OWNED as public `hanzoai/*` forks — never
consumed as upstream-branded modules. Do NOT reintroduce `github.com/SigNoz/*`.

| upstream | fork | tag pinned | fork module path |
|---|---|---|---|
| SigNoz/signoz-otel-collector | hanzoai/otel-collector | v0.144.7 | `github.com/hanzoai/otel-collector` (renamed) |
| SigNoz/clickhouse-go-mock | hanzo-ds/mock | v0.14.4 | `github.com/hanzo-ds/mock` (renamed) |
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

### There is no cloud pin. The dependency is GONE, and it was a cycle

`github.com/hanzoai/o11y` does **not** require `github.com/hanzoai/cloud`. It did,
for one field of one struct on one line: `Mount(app *zip.App, deps cloud.Deps)`
read `deps.Logger` to announce itself. That single line made the module graph a
**cycle** — o11y required cloud, cloud requires o11y — and the cycle is what kept
the conversion out of the shipped binary:

- The community image builds `./cmd/community`, and the route declarations lived
  in a package braided to the host, so that binary could not link them without
  dragging cloud in. It did not link them. **353 typed ops, 353 published
  operations and 353 MCP tools were true of the source and absent from the
  process** — invisible to CI, because the tests import the table directly.
- A whole-module `go mod download` was unresolvable in any image without a cloud
  checkout, so the Dockerfile had to scope itself to one package to route around
  it, and the *reason* got written down as if it were a design decision.

`Mount(app *zip.App) error` takes the router and nothing else. The router already
carries the host's logger (`app.Logger()`), so the argument never carried
information the first argument did not already have. Dropping it removed cloud and
**~20 transitive requirements** behind it (`hanzoai/{iam,commerce,authz,tasks,vfs,orm,ha,s3-go,…}`)
from this module's graph.

Standing proof, and it must stay empty:

```
go list -deps ./cmd/community | grep hanzoai/cloud     # EMPTY
go list -deps ./cmd/community | grep -x github.com/hanzoai/o11y   # PRESENT
```

The second line is the one that matters: it is what says the declarations are in
the binary. `pkg/query-service/app`'s `publish()` mounts them onto the router the
server serves — after the service's own routes (so the handler that always
answered a path still answers it, and streams stay streams) and before the console
catch-all — then calls `app.Prepare()`, which is what turns the registry into
doors: `/.well-known/openapi.json`, `/docs`, `/mcp`, the by-name call plane. This
server serves through `Fiber().Listener`, not `zip.Listen`, so without that call
the document exists in the process and answers on no port.

Since a green build is NOT sufficient evidence (`hanzoai/zip` → `zap-proto/zip` can
boot-panic while CI stays green), smoke-test the binary: it must reach
`Query server started listening on 0.0.0.0:8080`. `TestEmailRejected`
(`alertmanagernotify/email`) fails on any go.mod — upstream string mismatch; ci.yaml
already `-skip`s it by name.

## Container image (`ghcr.io/hanzoai/o11y`)

Root `Dockerfile` + `.hanzo/workflows/docker.yaml` build a standalone community
server image on push to `main` and `v*` tags → `ghcr.io/hanzoai/o11y:<sha>` (+ `:main`).
This replaces an unrelated upstream image that previously squatted the tags.

- Builds `./cmd/community`, the binary the image runs. No cloud sibling, no
  replace, no pin — the module does not name cloud at all any more (see "There is
  no cloud pin" above). The build is scoped to the one package because that is the
  binary, not because a whole-module resolve would fail; it no longer would.
- Its graph pulls **PRIVATE** forks — `hanzoai/sqlite` and `hanzo-ds/go` (the
  sqlite + datastore drivers added by the driver swap) — alongside the public
  ones (otel-collector, govaluate, hanzo-ds/mock, expr). So the module fetch
  DOES need git auth: the
  Dockerfile mounts a `gh_token` build secret (docker.yaml passes
  `secrets.GH_PAT`) and wires `git url.insteadOf` before `go build`. Without it
  the build 128s on `git ls-remote https://github.com/hanzoai/sqlite` ("could
  not read Username … terminal prompts disabled"). `GOPRIVATE=github.com/hanzoai/*
  GOSUMDB=off` still route hanzoai/* direct and skip the sumdb. GHCR push uses
  `GH_PAT || GITHUB_TOKEN` (package linked to hanzoai/o11y, so
  GITHUB_TOKEN+`packages: write` suffices for the push; GH_PAT is needed for the
  cross-repo private module fetch).
- Runs headless: `O11Y_WEB_ENABLED=false`. `routerweb` os.Stat()s its web dir at
  boot and fatals if missing; the SPA is served by hanzoai/static at the edge. The
  `frontend/` tree is NOT bundled — its `pnpm-lock.yaml` is STALE vs `package.json`
  (mid rolldown-vite/oxlint migration), fails `--frozen-lockfile`. TODO: regen lockfile.
- Listens on `0.0.0.0:8080` (constants.HTTPHostPort); sqlstore default = sqlite at
  `/var/lib/o11y/o11y.db`; needs an external Datastore for telemetry.

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
`claims.OrgID`; Datastore telemetry is isolated only insofar as the emit path tags
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

## Config env naming (Hanzo-branded operator surface)

Operator-facing env vars and config keys are Hanzo-branded. What stays untouched is
the wire surface — the native protocol on 9000, the SQL dialect, and the on-disk
table names. Those are interop identifiers other tools match on, not naming surface.

- **Env prefix `O11Y_`** (was `O11Y_`): set once at `pkg/config/envprovider/provider.go`
  (`prefix`). The koanf env provider derives every structured key from it. Single `_`
  is the `::` path delimiter, double `__` is a literal `_`.
- **Store key segment `datastore`** (was `clickhouse`): `telemetrystore.Config.Datastore`
  carries the `mapstructure:"datastore"` tag. YAML key `telemetrystore.datastore`.
  Provider selector value is `datastore` (`MustNewName("datastore")` in
  `pkg/telemetrystore/datastoretelemetrystore`); the store accessor is
  `TelemetryStore.Datastore() datastore.Conn`.
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
- **Deliberately NOT renamed** (implementation / not app-config surface): SQL/table/DB
  names (`o11y_traces` etc.) and query-template vars
  (`O11Y_START_TIME`/`O11Y_END_TIME`), the datastore **server** container in `deploy/`
  (service/volume names, and `CLICKHOUSE_SKIP_USER_SETUP` — the pinned server image's
  own entrypoint reads that name, so renaming it here would leave the container reading
  an unset value), the `O11Y_E2E_*` Playwright test-harness vars (separate `tests/e2e`
  subsystem), and the `clickhouse` builtin-integration id, which identifies the
  third-party ClickHouse deployments users point o11y at and keys the
  `088_migrate_ii_dashboards` dashboards. The `O11Y_OTEL_COLLECTOR_DATASTORE_*` keys in
  `deploy/` are consumed by the `hanzoai/otel-collector` fork, whose exporters are
  `o11ydatastoremetrics`, `o11ydatastoremeter`, `datastorelogsexporter` and
  `datastoretracesexporter`.

## Native datastore metrics driver (`pkg/datastoremetrics`) — the fork unblock

- **What**: the o11y-native write path for metrics. `Writer.WriteMetrics` satisfies
  `zapmetricreceiver.Handler`, decoding a ZAP `MetricBatch` straight into the datastore
  `time_series_v4` + `samples_v4` tables over the `hanzo-ds/go` driver (via
  `telemetrystore.TelemetryStore.Datastore()`), reusing the `telemetrymetrics`
  table-name constants so a series written here is immediately queryable.
- **Why it exists**: the stock `o11ydatastoremetrics` exporter serialises OTLP
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
- **Boundary**: the physical names live in `telemetrymetrics` and ONLY there — it is
  the source of truth this driver reuses. That migration has since happened: the
  tables are `event.metric` and `event.series` (plus `metric_5m`/`metric_30m` and
  `series_6h`/`series_1d`/`series_1w`), NOT `o11y_metrics.*_v4`. There are no
  `distributed_` wrappers on this deployment, so a table's local and distributed
  name are the same string.

  **Read the names from `telemetrymetrics`; never respell them.** `pkg/prometheus/
  datastoreprometheus` kept its own copy of the old spelling and so went on
  addressing `o11y_metrics`, a database that does not exist — every PromQL read
  failed with "Database o11y_metrics does not exist", which is 14 platform alert
  rules (`queryType: promql`: service-down, memory-critical, ingest-drop-rate,
  event-warehouse-write-stopped, alert-egress-failing, …) evaluating against an
  error, plus every PromQL dashboard panel drawing an empty chart. A copy has no
  reason to change when the original does, and the copy the migration does not read
  is the one that rots. Fixed in v1.5.62; `table_test.go` asserts the rendered SQL
  so the next drift fails a test instead of a dashboard.

  Still on their own databases, and still to converge: `telemetrymetadata`
  (`o11y_metadata`), `telemetrymeter` (`o11y_meter`), `telemetryaudit`
  (`o11y_audit`), `rulestatehistory` (`o11y_analytics` — this one EXISTS), and the
  legacy `query-service` builders (`constants.O11Y_METRIC_DBNAME` and
  `datastorereader`'s const block), which are not reached by cloud's embedded
  runtime but would break the same way if they were.
