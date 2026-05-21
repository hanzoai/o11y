package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/hanzoai/o11y/cmd"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/iamauthz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/gateway/noopgateway"
	"github.com/hanzoai/o11y/pkg/licensing"
	"github.com/hanzoai/o11y/pkg/licensing/nooplicensing"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/dashboard/impldashboard"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/query-service/app"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/hanzoai/o11y/pkg/zapreceiver"
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/hanzoai/o11y/pkg/zeus/noopzeus"
	"github.com/spf13/cobra"
)

func registerServer(parentCmd *cobra.Command, logger *slog.Logger) {
	var flags o11y.DeprecatedFlags

	serverCmd := &cobra.Command{
		Use:                "server",
		Short:              "Run the HanzoO11y server",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(currCmd *cobra.Command, args []string) error {
			config, err := cmd.NewHanzoO11yConfig(currCmd.Context(), logger, flags)
			if err != nil {
				return err
			}

			return runServer(currCmd.Context(), config, logger)
		},
	}

	flags.RegisterFlags(serverCmd)
	parentCmd.AddCommand(serverCmd)
}

func runServer(ctx context.Context, config o11y.Config, logger *slog.Logger) error {
	// print the version
	version.Info.PrettyPrint(config.Version)

	o11y, err := o11y.New(
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
		o11y.NewWebProviderFactories(),
		func(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
			return o11y.NewSQLSchemaProviderFactories(sqlstore)
		},
		o11y.NewSQLStoreProviderFactories(),
		o11y.NewTelemetryStoreProviderFactories(),
		func(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, licensing licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
			return o11y.NewAuthNs(ctx, providerSettings, store, licensing)
		},
		func(_ context.Context, sqlstore sqlstore.SQLStore, _ licensing.Licensing, _ dashboard.Module) factory.ProviderFactory[authz.AuthZ, authz.Config] {
			return iamauthz.NewProviderFactory(sqlstore)
		},
		func(store sqlstore.SQLStore, settings factory.ProviderSettings, analytics analytics.Analytics, orgGetter organization.Getter, queryParser queryparser.QueryParser, _ querier.Querier, _ licensing.Licensing) dashboard.Module {
			return impldashboard.NewModule(impldashboard.NewStore(store), settings, analytics, orgGetter, queryParser)
		},
		func(_ licensing.Licensing) factory.ProviderFactory[gateway.Gateway, gateway.Config] {
			return noopgateway.NewProviderFactory()
		},
		func(ps factory.ProviderSettings, q querier.Querier, a analytics.Analytics) querier.Handler {
			return querier.NewHandler(ps, q, a)
		},
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create o11y", "error", err)
		return err
	}

	server, err := app.NewServer(config, o11y)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create server", "error", err)
		return err
	}

	if err := server.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start server", "error", err)
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

	o11y.Start(ctx)

	if err := o11y.Wait(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start o11y", "error", err)
		return err
	}

	err = server.Stop(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to stop server", "error", err)
		return err
	}

	err = o11y.Stop(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to stop o11y", "error", err)
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
