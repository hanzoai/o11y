package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/hanzoai/o11y/cmd"
	"github.com/hanzoai/o11y/ee/authn/callbackauthn/oidccallbackauthn"
	"github.com/hanzoai/o11y/ee/authn/callbackauthn/samlcallbackauthn"
	"github.com/hanzoai/o11y/ee/authz/openfgaauthz"
	eequerier "github.com/hanzoai/o11y/ee/querier"
	"github.com/hanzoai/o11y/ee/authz/openfgaschema"
	"github.com/hanzoai/o11y/ee/gateway/httpgateway"
	enterpriselicensing "github.com/hanzoai/o11y/ee/licensing"
	"github.com/hanzoai/o11y/ee/licensing/httplicensing"
	"github.com/hanzoai/o11y/ee/modules/dashboard/impldashboard"
	enterpriseapp "github.com/hanzoai/o11y/ee/query-service/app"
	"github.com/hanzoai/o11y/ee/sqlschema/postgressqlschema"
	"github.com/hanzoai/o11y/ee/sqlstore/postgressqlstore"
	enterprisezeus "github.com/hanzoai/o11y/ee/zeus"
	"github.com/hanzoai/o11y/ee/zeus/httpzeus"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/licensing"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	pkgimpldashboard "github.com/hanzoai/o11y/pkg/modules/dashboard/impldashboard"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/o11y"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstorehook"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/hanzoai/o11y/pkg/zeus"
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

	// add enterprise sqlstore factories to the community sqlstore factories
	sqlstoreFactories := o11y.NewSQLStoreProviderFactories()
	if err := sqlstoreFactories.Add(postgressqlstore.NewFactory(sqlstorehook.NewLoggingFactory(), sqlstorehook.NewInstrumentationFactory())); err != nil {
		logger.ErrorContext(ctx, "failed to add postgressqlstore factory", "error", err)
		return err
	}

	o11y, err := o11y.New(
		ctx,
		config,
		enterprisezeus.Config(),
		httpzeus.NewProviderFactory(),
		enterpriselicensing.Config(24*time.Hour, 3),
		func(sqlstore sqlstore.SQLStore, zeus zeus.Zeus, orgGetter organization.Getter, analytics analytics.Analytics) factory.ProviderFactory[licensing.Licensing, licensing.Config] {
			return httplicensing.NewProviderFactory(sqlstore, zeus, orgGetter, analytics)
		},
		o11y.NewEmailingProviderFactories(),
		o11y.NewCacheProviderFactories(),
		o11y.NewWebProviderFactories(),
		func(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
			existingFactories := o11y.NewSQLSchemaProviderFactories(sqlstore)
			if err := existingFactories.Add(postgressqlschema.NewFactory(sqlstore)); err != nil {
				panic(err)
			}

			return existingFactories
		},
		sqlstoreFactories,
		o11y.NewTelemetryStoreProviderFactories(),
		func(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, licensing licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
			samlCallbackAuthN, err := samlcallbackauthn.New(ctx, store, licensing)
			if err != nil {
				return nil, err
			}

			oidcCallbackAuthN, err := oidccallbackauthn.New(store, licensing, providerSettings)
			if err != nil {
				return nil, err
			}

			authNs, err := o11y.NewAuthNs(ctx, providerSettings, store, licensing)
			if err != nil {
				return nil, err
			}

			authNs[authtypes.AuthNProviderSAML] = samlCallbackAuthN
			authNs[authtypes.AuthNProviderOIDC] = oidcCallbackAuthN

			return authNs, nil
		},
		func(ctx context.Context, sqlstore sqlstore.SQLStore, licensing licensing.Licensing, dashboardModule dashboard.Module) factory.ProviderFactory[authz.AuthZ, authz.Config] {
			return openfgaauthz.NewProviderFactory(sqlstore, openfgaschema.NewSchema().Get(ctx), licensing, dashboardModule)
		},
		func(store sqlstore.SQLStore, settings factory.ProviderSettings, analytics analytics.Analytics, orgGetter organization.Getter, queryParser queryparser.QueryParser, querier querier.Querier, licensing licensing.Licensing) dashboard.Module {
			return impldashboard.NewModule(pkgimpldashboard.NewStore(store), settings, analytics, orgGetter, queryParser, querier, licensing)
		},
		func(licensing licensing.Licensing) factory.ProviderFactory[gateway.Gateway, gateway.Config] {
			return httpgateway.NewProviderFactory(licensing)
		},
		func(ps factory.ProviderSettings, q querier.Querier, a analytics.Analytics) querier.Handler {
			communityHandler := querier.NewHandler(ps, q, a)
			return eequerier.NewHandler(ps, q, communityHandler)
		},
	)

	if err != nil {
		logger.ErrorContext(ctx, "failed to create o11y", "error", err)
		return err
	}

	server, err := enterpriseapp.NewServer(config, o11y)
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
