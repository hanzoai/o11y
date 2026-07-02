package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/cmd"
	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/auditor"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/iamauthz"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/errors"
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
	"github.com/hanzoai/o11y/pkg/ruler/o11yruler"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/hanzoai/o11y/pkg/zapreceiver"
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/hanzoai/o11y/pkg/zeus/noopzeus"
)

func registerServer(parentCmd *cobra.Command, logger *slog.Logger) {
	var flags o11y.DeprecatedFlags
	var configFiles []string

	serverCmd := &cobra.Command{
		Use:                "server",
		Short:              "Run the HanzoO11y server",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(currCmd *cobra.Command, args []string) error {
			config, err := cmd.NewHanzoO11yConfig(currCmd.Context(), logger, configFiles, flags)
			if err != nil {
				return err
			}

			return runServer(currCmd.Context(), config, logger)
		},
	}

	serverCmd.Flags().StringArrayVar(&configFiles, "config", nil, "path to a YAML configuration file (can be specified multiple times, later files override earlier ones)")
	flags.RegisterFlags(serverCmd)
	parentCmd.AddCommand(serverCmd)
}

func runServer(ctx context.Context, config o11y.Config, logger *slog.Logger) error {
	// print the version
	version.Info.PrettyPrint(config.Version)

	o11yInstance, err := o11y.New(
		ctx,
		config,
		zeus.Config{},
		noopzeus.NewProviderFactory(),
		licensing.Config{},
		func(_ sqlstore.SQLStore, _ zeus.Zeus, _ organization.Getter, _ analytics.Analytics) factory.ProviderFactory[licensing.Licensing, licensing.Config] {
			return nooplicensing.NewFactory()
		},
		o11y.NewEmailingProviderFactories(),
		o11y.NewCacheProviderFactories(),
		o11y.NewWebProviderFactories(config.Global),
		func(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
			return o11y.NewSQLSchemaProviderFactories(sqlstore)
		},
		o11y.NewSQLStoreProviderFactories(),
		o11y.NewTelemetryStoreProviderFactories(),
		func(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, licensing licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
			return o11y.NewAuthNs(ctx, providerSettings, store, licensing)
		},
		func(_ context.Context, sqlstore sqlstore.SQLStore, _ authz.Config, _ licensing.Licensing, _ []authz.OnBeforeRoleDelete) (factory.ProviderFactory[authz.AuthZ, authz.Config], error) {
			return iamauthz.NewProviderFactory(sqlstore), nil
		},
		func(store sqlstore.SQLStore, settings factory.ProviderSettings, analytics analytics.Analytics, orgGetter organization.Getter, queryParser queryparser.QueryParser, _ querier.Querier, _ licensing.Licensing, tagModule tag.Module) dashboard.Module {
			return impldashboard.NewModule(impldashboard.NewStore(store), settings, analytics, orgGetter, queryParser, tagModule)
		},
		func(_ licensing.Licensing) factory.ProviderFactory[gateway.Gateway, gateway.Config] {
			return noopgateway.NewProviderFactory()
		},
		func(_ licensing.Licensing) factory.NamedMap[factory.ProviderFactory[auditor.Auditor, auditor.Config]] {
			return o11y.NewAuditorProviderFactories()
		},
		func(_ context.Context, _ factory.ProviderSettings, _ flagger.Flagger, _ licensing.Licensing, _ telemetrystore.TelemetryStore, _ retention.Getter, _ organization.Getter, _ zeus.Zeus) (factory.NamedMap[factory.ProviderFactory[meterreporter.Reporter, meterreporter.Config]], string) {
			return o11y.NewMeterReporterProviderFactories(), "noop"
		},
		func(ps factory.ProviderSettings, q querier.Querier, a analytics.Analytics) querier.Handler {
			return querier.NewHandler(ps, q, a)
		},
		func(_ sqlstore.SQLStore, _ dashboard.Module, _ global.Global, _ zeus.Zeus, _ gateway.Gateway, _ licensing.Licensing, _ serviceaccount.Module, _ cloudintegration.Config) (cloudintegration.Module, error) {
			return implcloudintegration.NewModule(), nil
		},
		func(c cache.Cache, am alertmanager.Alertmanager, ss sqlstore.SQLStore, ts telemetrystore.TelemetryStore, ms telemetrytypes.MetadataStore, p prometheus.Prometheus, og organization.Getter, rsh rulestatehistory.Module, q querier.Querier, qp queryparser.QueryParser) factory.NamedMap[factory.ProviderFactory[ruler.Ruler, ruler.Config]] {
			return factory.MustNewNamedMap(o11yruler.NewFactory(c, am, ss, ts, ms, p, og, rsh, q, qp, nil, nil))
		},
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create o11y", errors.Attr(err))
		return err
	}

	server, err := app.NewServer(config, o11yInstance)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create server", errors.Attr(err))
		return err
	}

	if err := server.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start server", errors.Attr(err))
		return err
	}

	// ZAP-native trace ingestion. Spans shipped by luxfi/trace's
	// Type=ZAP exporter land here and (TODO) get written to the
	// telemetry store. Defaults to :4317; O11Y_ZAP_LISTEN overrides.
	// This is the OTel-on-ZAP path that replaces OTLP-gRPC ingestion
	// — no protobuf, no grpc, just zap envelopes.
	zapRcv, err := zapreceiver.New(zapreceiver.Config{
		Listen: zapReceiverAddr(),
		Logger: logger,
		OnBatch: func(_ context.Context, b *zapreceiver.SpanBatch) error {
			logger.DebugContext(ctx, "zap span batch received",
				"appName", b.AppName,
				"version", b.Version,
				"spans", len(b.Spans),
			)
			// TODO(zap-ingest): write batch.Spans into telemetrystore.TelemetryStore
			// once the SpanBatch → Datastore adapter lands.
			return nil
		},
	})
	if err != nil {
		logger.WarnContext(ctx, "zap-native receiver disabled", "error", err)
	} else {
		defer zapRcv.Stop()
		logger.InfoContext(ctx, "zap-native trace receiver listening", "addr", zapReceiverAddr())
	}

	o11yInstance.Start(ctx)

	if err := o11yInstance.Wait(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start o11y", errors.Attr(err))
		return err
	}

	err = server.Stop(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to stop server", errors.Attr(err))
		return err
	}

	err = o11yInstance.Stop(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to stop o11y", errors.Attr(err))
		return err
	}

	return nil
}

// zapReceiverAddr returns the bind address for the ZAP span receiver.
// O11Y_ZAP_LISTEN overrides; default is :4317 (canonical o11y ZAP port).
func zapReceiverAddr() string {
	if v := os.Getenv("O11Y_ZAP_LISTEN"); v != "" {
		return v
	}
	return ":4317"
}
