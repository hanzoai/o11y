# RFC: Decouple o11y from Prometheus Alertmanager

**Status:** Draft &middot; **Owner:** o11y team &middot; **Tracking:** part of the
broader "rip out Prometheus" initiative

## tl;dr

o11y's alerting plane currently sits on top of 23 imported sub-packages of
`github.com/prometheus/alertmanager`. This is the largest remaining
Prometheus dep in the Hanzo codebase. Replacing it is genuinely
architectural work, not a sed-and-go.mod migration. This RFC scopes the
work, lists the options with honest trade-offs, and proposes a phased
migration.

## What we actually depend on

Inventory of Alertmanager sub-packages imported by o11y:

| Sub-package                 | What we use it for                                            | Bytes of pain |
|-----------------------------|---------------------------------------------------------------|---------------|
| `config`                    | YAML config schema (routes, receivers, inhibits, templates)   | the schema is the API contract |
| `config/receiver`           | Per-channel receiver structs (Slack, PagerDuty, webhook, ...) | every channel is a struct |
| `dispatch`                  | Route tree evaluation, alert grouping                          | core dispatcher logic |
| `provider`, `provider/mem`  | Active-alert storage interface + in-memory impl                | swap-able |
| `notify`, `notify/test`     | Notification pipeline, retry, dedup, throttle                  | the meat |
| `silence`, `nflog`          | Silence storage + notification log (idempotency)              | persistent state |
| `inhibit`                   | Inhibition rules (suppress dependent alerts)                   | edge feature |
| `template`, `pkg/labels`    | Go-template engine + label matcher primitives                  | very widely referenced |
| `matcher/compat`            | Legacy matcher parser                                          | parser |
| `timeinterval`              | Mute windows                                                  | calendar math |
| `types`                     | `Alert`, `Marker`, `MultiError`                               | dozens of references |
| `api/v2`, `api/v2/models`   | OpenAPI generated handlers/models                              | wire format |
| `cluster`                   | Gossip (HA)                                                   | not actually used in our deploys |
| `featurecontrol`            | Feature flag struct                                            | trivial |
| `store`                     | Alert store interface                                          | swap-able |

Files in `pkg/types/alertmanagertypes/` (Hanzo's wrapper layer):
`alert, channel, config, expression, expressionroute, maintenance, marker,
matcher, notification, receiver, recurrence, route, schedule, template`.

## Why it's hard

1. **The schema IS the API.** SigNoz's frontend, our k8s CRDs, and any
   external alert source all speak the Alertmanager YAML schema. Replacing
   the parser without preserving the schema breaks every consumer.
2. **Routing is non-trivial.** Alertmanager's route tree (with `continue`,
   inhibit rules, grouping wait/interval/repeat) is small in code but
   subtle in semantics. A clean-room reimplementation has to match
   behavior bug-for-bug.
3. **Persisted state.** Silences and the notification log are persisted
   structures. A migration has to read existing on-disk data or accept a
   one-time wipe.
4. **23 import paths.** Not just a top-level dep — every replacement
   sub-package needs an equivalent.

## Options

### Option 1 — Native rewrite (clean room)

Build `pkg/alerting/` from scratch. Use the existing YAML schema verbatim
but parse it with our own structs. Reimplement dispatcher + notify
pipeline + silence store on top of `luxfi/metric` (for any metrics) and
the existing o11y persistence.

- **Pro:** zero Prometheus deps. Aligned with the broader rip-out.
- **Pro:** can shed Alertmanager's quirks (e.g. gossip cluster, OpenAPI
  generated handlers) we don't use.
- **Con:** ~4-6 weeks of focused work. Highest risk.
- **Con:** Behavior parity is a moving target — existing alerts deployed
  in prod silently change behavior unless we test exhaustively.

### Option 2 — Hard fork at upstream HEAD (`hanzoai/alertmanager-lite`)

Fork `prometheus/alertmanager`, strip the parts we don't use (cluster,
generated v2 API surface, mesh), keep config + dispatch + notify +
silence + nflog as a single internal package under our control. Replace
`prometheus/client_golang` with `luxfi/metric` *inside* the fork using
the script we already wrote.

- **Pro:** ~1-2 weeks. Lowest risk on behavior parity.
- **Pro:** No prometheus/* in *our* dep graph. The fork's go.mod still
  has them as indirect — that's fine, they're encapsulated.
- **Con:** Long-term maintenance burden — every upstream alertmanager
  CVE means a manual catch-up.
- **Con:** Doesn't satisfy "no Prometheus in our codebase" at a
  philosophical level. It just hides Prometheus behind our package
  boundary.

### Option 3 — Swap out the notification pipeline only

Keep Alertmanager's config schema + parser. Replace the dispatcher +
notify pipeline with a thin native one (we already wrap notify in
`pkg/types/alertmanagertypes/notification.go`). Inhibits and silences
get reimplemented; routing tree stays Alertmanager's.

- **Pro:** ~2-3 weeks. Surgical.
- **Pro:** Preserves the config schema as the API contract.
- **Con:** Still imports `alertmanager/config` and `alertmanager/dispatch`.
  Doesn't fully exit Prometheus dep.

### Option 4 — Accept the Alertmanager dep

The o11y team explicitly carves out alerting as the one place we
intentionally use upstream Prometheus types, because SigNoz's contract
*is* the Alertmanager schema. Every other Hanzo repo is now
prometheus-free.

- **Pro:** zero migration cost. The 24-repo migration already shipped.
- **Pro:** Honest: nothing else in our codebase uses Prometheus.
- **Con:** "Rip out Prometheus" reads as "rip it out except where
  semantics require it." Pragmatically true, philosophically weak.

## Recommendation

**Option 4 in the short term, Option 1 in the long term.**

The 24-repo client_golang → luxfi/metric migration that shipped alongside
this RFC already removes Prometheus from 161 files. The remaining
alertmanager dependency is *isolated to o11y's alerting plane* and is
load-bearing for the SigNoz frontend's contract.

A clean-room rewrite (Option 1) is the only path that ends with zero
Prometheus in any Hanzo binary. But it's a 4-6 week initiative that
needs its own RFC for the wire format choices, the dispatcher semantics,
and the silence/nflog storage migration. Doing it half-way leaves o11y
in a worse state than starting position.

## If Option 1 is greenlit, phased plan

| Phase | Scope | Effort | Risk |
|---|---|---|---|
| 1 | Define `pkg/alerting/wire/` — Go structs mirroring Alertmanager YAML schema. Round-trip tested against a fleet of real prod configs. | 3-5 days | low |
| 2 | Build `pkg/alerting/route/` — route tree evaluation with property-based tests against Alertmanager behavior. | 5-7 days | medium |
| 3 | Build `pkg/alerting/notify/` — dispatcher, throttle, retry, dedup. Per-channel receivers (Slack, PagerDuty, webhook, email). | 7-10 days | medium |
| 4 | Build `pkg/alerting/silence/` and `pkg/alerting/nflog/` — persistence layer with a one-way migration from Alertmanager's bbolt format. | 5-7 days | high |
| 5 | Cut over `pkg/types/alertmanagertypes/` to point at the new packages. Delete imports of `prometheus/alertmanager/*`. | 2-3 days | medium |
| 6 | Soak in dev/staging, dual-write notifications behind a flag, cut prod over. | 5-7 days | high |

Total: ~4-6 weeks calendar time for one engineer focused.

## Open questions

1. Does anything outside o11y consume the Alertmanager config schema
   (e.g. k8s CRDs, alert-router)? If yes, schema changes are
   cross-repo and need their own coordination.
2. Is the SigNoz frontend wired to send alerts to o11y via the
   Alertmanager v2 OpenAPI schema, or via SigNoz's own internal RPC?
   If the former, the v2 wire format is part of the contract and we
   need a compatibility shim regardless of which option we pick.
3. Do we actually use Alertmanager's inhibit rules in production? If
   nobody configures them, dropping them simplifies the rewrite.

## Decision required

Pick an option and accept the carry cost.

- If Option 4 wins, close this RFC and update the org README to clarify
  that "no Prometheus" excludes the alertmanager schema dep in o11y.
- If Option 1 wins, this RFC is the design backbone — refine the phased
  plan into concrete tickets and start at Phase 1.
