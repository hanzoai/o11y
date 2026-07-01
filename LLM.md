# o11y

<h1 align="center" style="border-bottom: none">
    <a href="https://o11y.hanzo.ai" target="_blank">
        <img alt="Hanzo O11y" src="https://github.com/user-attachments/assets/ef9a33f7-12d7-4c94-8908-0a02b22f0c18" width="100" height="100">
    </a>
    <br>Hanzo O11y
</h1>

Hanzo's observability spine — OTLP in, Hanzo Datastore (ClickHouse OLAP) on disk,
serving o11y.hanzo.ai. Fork of SigNoz, being turned into a proper full fork.

## Dependency ownership (fork boundary)

All SigNoz-branded platform deps are OWNED as public `hanzoai/*` forks — never
consumed as upstream-branded modules. Do NOT reintroduce `github.com/SigNoz/*`.

| upstream | fork | tag pinned | fork module path |
|---|---|---|---|
| SigNoz/signoz-otel-collector | hanzoai/signoz-otel-collector | v0.144.6 | `github.com/hanzoai/signoz-otel-collector` (renamed) |
| SigNoz/clickhouse-go-mock | hanzoai/clickhouse-go-mock | v0.14.1 | `github.com/hanzoai/clickhouse-go-mock` (renamed) |
| SigNoz/govaluate | hanzoai/govaluate | v0.1.0 | `github.com/hanzoai/govaluate` (renamed) |
| SigNoz/expr | hanzoai/expr | v1.17.8 | `github.com/expr-lang/expr` (KEEP upstream path — consumed via `replace`) |
| SigNoz/signoz | hanzoai/signoz | (tracks 76ee2986) | owned base for the vendored `pkg/ ee/ cmd/` tree |

The vendored SigNoz source in `pkg/ ee/ cmd/` self-references as
`github.com/hanzoai/o11y/...` (NOT `github.com/SigNoz/signoz/...`). Generic
ecosystem libs (`golang.org/x`, `google.golang.org`, prometheus, otel, gin,
cobra, `luxfi/*`) are NOT forked — only the SigNoz platform.

## Known pre-existing gap (blocks `go build ./...`)

`origin/main` does not compile: 22 packages are imported but absent from the
vendored tree. 8 exist upstream and can be vendored from `hanzoai/signoz@76ee2986`
(`pkg/signoz`, `pkg/query-service/app/clickhouseReader`, `ee/query-service/{constants,model}`,
`ee/gateway/httpgateway`, `pkg/ruler/signozruler`, `ee/authn/callbackauthn/samlcallbackauthn`,
`ee/licensing/licensingstore/sqllicensingstore`). 14 exist in NO SigNoz version and
are Hanzo-side WIP to be written (`roletypes`, `parser/grammar`, `cloudintegrations`(+`/services`),
`querybuilder/{resourcefilter,agg_rewrite}`, `types/integrationtypes`,
`query-service/app/metricsexplorer`, `query-service/model/metrics_explorer`,
`query-service/utils/times`, `querier/bucket_cache`, `sqlitesqlstore`,
`sqlschema/postgressqlschema`, `sqlstore/postgressqlstore`). Also pre-existing:
unused imports (`pkg/http/render/render.go`), self-import cycle
(`pkg/types/authtypes/role.go`), duplicate package decl (`pkg/telemetrystore/datastoretelemetrystore`).
