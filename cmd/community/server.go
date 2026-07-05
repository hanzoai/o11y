package main

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/hanzoai/o11y/cmd"
	"github.com/hanzoai/o11y/pkg/community"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/signoz"
	"github.com/hanzoai/o11y/pkg/version"
)

func registerServer(parentCmd *cobra.Command, logger *slog.Logger) {
	var configFiles []string

	serverCmd := &cobra.Command{
		Use:                "server",
		Short:              "Run the SigNoz server",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(currCmd *cobra.Command, args []string) error {
			config, err := cmd.NewSigNozConfig(currCmd.Context(), logger, configFiles)
			if err != nil {
				return err
			}

			return runServer(currCmd.Context(), config, logger)
		},
	}

	serverCmd.Flags().StringArrayVar(&configFiles, "config", nil, "path to a YAML configuration file (can be specified multiple times, later files override earlier ones)")
	parentCmd.AddCommand(serverCmd)
}

func runServer(ctx context.Context, config signoz.Config, logger *slog.Logger) error {
	// print the version
	version.Info.PrettyPrint(config.Version)

	// community.NewServer is the ONE construction shared with the hanzoai/cloud
	// embed — same providers, same identity (iamidentn gateway-header auth), same
	// wiring. Standalone owns the process: bind listeners, run background
	// evaluation, block until shutdown.
	server, signoz, err := community.NewServer(ctx, config)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create signoz server", errors.Attr(err))
		return err
	}

	if err := server.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start server", errors.Attr(err))
		return err
	}

	signoz.Start(ctx)

	if err := signoz.Wait(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start signoz", errors.Attr(err))
		return err
	}

	if err := server.Stop(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to stop server", errors.Attr(err))
		return err
	}

	if err := signoz.Stop(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to stop signoz", errors.Attr(err))
		return err
	}

	return nil
}
