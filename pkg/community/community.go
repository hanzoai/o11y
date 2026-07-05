// Package community constructs the ONE Hanzo o11y (SigNoz-community) runtime and
// HTTP server used by BOTH the standalone `cmd/community` binary AND the unified
// hanzoai/cloud binary's in-process embed (via app.Server.PublicHandler).
//
// Keeping the whole construction — config resolution, the SigNoz provider set,
// and the app server — behind a single exported builder guarantees the two
// deployments run byte-identical middleware, identity (pkg/identn/iamidentn,
// i.e. Hanzo IAM gateway-header auth), authz (iamauthz), telemetry stores, rule
// manager, dashboards and alerts. One construction, one way: the embed cannot
// drift from the standalone pod's auth or wiring, because they are the same code.
package community

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/auditor"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/iamauthz"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/config/fileprovider"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/gateway/noopgateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/licensing"
	"github.com/hanzoai/o11y/pkg/licensing/nooplicensing"
	"github.com/hanzoai/o11y/pkg/meterreporter"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration/implcloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/dashboard/impldashboard"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule/implmetricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/retention"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/tag"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/query-service/app"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/ruler/signozruler"
	"github.com/hanzoai/o11y/pkg/signoz"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/hanzoai/o11y/pkg/zeus/noopzeus"
)

// NewConfig resolves the SigNoz config from the given YAML files (if any) plus
// the process environment (env:), applying the Hanzo operator-facing aliases
// (e.g. the flat O11Y_DATASTORE_DSN → telemetrystore.datastore.dsn) inside
// signoz.NewConfig. This is THE config path; cmd.NewSigNozConfig delegates here
// so standalone and embed read configuration identically.
func NewConfig(ctx context.Context, logger *slog.Logger, configFiles []string) (signoz.Config, error) {
	uris := make([]string, 0, len(configFiles)+1)
	for _, f := range configFiles {
		uris = append(uris, "file:"+f)
	}
	uris = append(uris, "env:")

	return signoz.NewConfig(
		ctx,
		logger,
		config.ResolverConfig{
			Uris: uris,
			ProviderFactories: []config.ProviderFactory{
				envprovider.NewFactory(),
				fileprovider.NewFactory(),
			},
		},
	)
}

// SQLStoreProviderFactories is the community SQL store provider set (the
// control-plane metadata store — sqlite by default).
func SQLStoreProviderFactories() factory.NamedMap[factory.ProviderFactory[sqlstore.SQLStore, sqlstore.Config]] {
	return signoz.NewSQLStoreProviderFactories()
}

// SQLSchemaProviderFactories is the community SQL schema provider set.
func SQLSchemaProviderFactories(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
	return signoz.NewSQLSchemaProviderFactories(sqlstore)
}

// NewSigNoz constructs the SigNoz runtime with the community provider set: noop
// zeus/licensing/gateway, Hanzo IAM authz (iamauthz — the sole authorizer), the
// ClickHouse (Hanzo Datastore) telemetry store, sqlite control-plane store, the
// full dashboard/cloudintegration/metricreductionrule/ruler modules, and (wired
// internally by signoz.New) the identN provider set including iamidentn — the
// gateway-header human identity the running pod trusts. The provider list is the
// single source of truth for how o11y boots.
func NewSigNoz(ctx context.Context, config signoz.Config) (*signoz.SigNoz, error) {
	return signoz.New(
		ctx,
		config,
		zeus.Config{},
		noopzeus.NewProviderFactory(),
		licensing.Config{},
		func(_ sqlstore.SQLStore, _ zeus.Zeus, _ organization.Getter, _ analytics.Analytics) factory.ProviderFactory[licensing.Licensing, licensing.Config] {
			return nooplicensing.NewFactory()
		},
		signoz.NewEmailingProviderFactories(),
		signoz.NewCacheProviderFactories(),
		signoz.NewWebProviderFactories(config.Global),
		SQLSchemaProviderFactories,
		SQLStoreProviderFactories(),
		signoz.NewTelemetryStoreProviderFactories(),
		func(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, licensing licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
			return signoz.NewAuthNs(ctx, providerSettings, store, licensing, config.Global)
		},
		func(_ context.Context, sqlstore sqlstore.SQLStore, _ authz.Config, _ licensing.Licensing, _ []authz.OnBeforeRoleDelete) (factory.ProviderFactory[authz.AuthZ, authz.Config], error) {
			// Hanzo IAM is the sole authorization provider — every decision is
			// delegated to IAM's Casbin enforce endpoint. No OpenFGA, no fallback.
			return iamauthz.NewProviderFactory(sqlstore), nil
		},
		func(store sqlstore.SQLStore, settings factory.ProviderSettings, analytics analytics.Analytics, orgGetter organization.Getter, queryParser queryparser.QueryParser, _ querier.Querier, _ licensing.Licensing, tagModule tag.Module) dashboard.Module {
			return impldashboard.NewModule(impldashboard.NewStore(store), settings, analytics, orgGetter, queryParser, tagModule)
		},
		func(_ licensing.Licensing) factory.ProviderFactory[gateway.Gateway, gateway.Config] {
			return noopgateway.NewProviderFactory()
		},
		func(_ licensing.Licensing) factory.NamedMap[factory.ProviderFactory[auditor.Auditor, auditor.Config]] {
			return signoz.NewAuditorProviderFactories()
		},
		func(_ context.Context, _ factory.ProviderSettings, _ flagger.Flagger, _ licensing.Licensing, _ telemetrystore.TelemetryStore, _ retention.Getter, _ organization.Getter, _ zeus.Zeus) (factory.NamedMap[factory.ProviderFactory[meterreporter.Reporter, meterreporter.Config]], string) {
			return signoz.NewMeterReporterProviderFactories(), "noop"
		},
		func(ps factory.ProviderSettings, q querier.Querier, a analytics.Analytics) querier.Handler {
			return querier.NewHandler(ps, q, a)
		},
		func(_ sqlstore.SQLStore, _ dashboard.Module, _ global.Global, _ zeus.Zeus, _ gateway.Gateway, _ licensing.Licensing, _ serviceaccount.Module, _ cloudintegration.Config) (cloudintegration.Module, error) {
			return implcloudintegration.NewModule(), nil
		},
		func(_ sqlstore.SQLStore, _ telemetrystore.TelemetryStore, _ dashboard.Module, _ queryparser.QueryParser, _ licensing.Licensing, _ flagger.Flagger, _ telemetrytypes.MetadataStore, _ factory.ProviderSettings, _ int) metricreductionrule.Module {
			return implmetricreductionrule.NewModule()
		},
		func(c cache.Cache, am alertmanager.Alertmanager, ss sqlstore.SQLStore, ts telemetrystore.TelemetryStore, ms telemetrytypes.MetadataStore, p prometheus.Prometheus, og organization.Getter, rsh rulestatehistory.Module, q querier.Querier, qp queryparser.QueryParser) factory.NamedMap[factory.ProviderFactory[ruler.Ruler, ruler.Config]] {
			return factory.MustNewNamedMap(signozruler.NewFactory(c, am, ss, ts, ms, p, og, rsh, q, qp, nil, nil))
		},
	)
}

// NewServer constructs the SigNoz runtime and its HTTP server together. Callers
// choose the serving mode:
//
//   - standalone (cmd/community): server.Start binds the listeners; SigNoz.Start
//     runs background evaluation; SigNoz.Wait blocks.
//   - embedded (hanzoai/cloud): SigNoz.Start runs background evaluation, and
//     server.PublicHandler() is installed via o11y.SetHandler — cloud's own HTTP
//     stack serves /v1/o11y/*; the listeners are never bound.
//
// Both paths share this ONE construction, so identity and authz are identical.
func NewServer(ctx context.Context, config signoz.Config) (*app.Server, *signoz.SigNoz, error) {
	sn, err := NewSigNoz(ctx, config)
	if err != nil {
		return nil, nil, err
	}

	server, err := app.NewServer(config, sn)
	if err != nil {
		return nil, nil, err
	}

	return server, sn, nil
}
