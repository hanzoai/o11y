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

## Hanzo layer to re-apply on the green base

The re-sync reverted these Hanzo-original packages to SigNoz canonical to kill the
skew; re-layer them on top (they live in git history at `c9ab975`): `pkg/authz/iamauthz`
(Hanzo IAM authz — REQUIRED per the house "always use Hanzo IAM" rule; SigNoz's
OpenFGA is the current default and must be replaced), `pkg/zapreceiver` +
`pkg/zapmetricreceiver` (ZAP-native OTLP receivers), `pkg/billing` + `pkg/types/billingtypes`.
Preserved through the sync: module path, `mount.go` (the `cloud.Register`/zip mount
adapter), `NOTICE`, `LICENSE`.
