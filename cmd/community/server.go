package main

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/cmd"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/openfgaauthz"
	"github.com/hanzoai/o11y/pkg/authz/openfgaschema"
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
	"github.com/hanzoai/o11y/pkg/o11y"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/hanzoai/o11y/pkg/zeus/noopzeus"
	"github.com/spf13/cobra"
)

func registerServer(parentCmd *cobra.Command, logger *slog.Logger) {
	var flags o11y.DeprecatedFlags

	serverCmd := &cobra.Command{
		Use:                "server",
		Short:              "Run the Hanzo O11y server",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(currCmd *cobra.Command, args []string) error {
			config, err := cmd.NewHanzo O11yConfig(currCmd.Context(), logger, flags)
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
		func(ctx context.Context, sqlstore sqlstore.SQLStore, _ licensing.Licensing, _ dashboard.Module) factory.ProviderFactory[authz.AuthZ, authz.Config] {
			return openfgaauthz.NewProviderFactory(sqlstore, openfgaschema.NewSchema().Get(ctx))
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
