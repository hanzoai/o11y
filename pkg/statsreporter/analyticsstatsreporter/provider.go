package analyticsstatsreporter

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/analytics/segmentanalytics"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/version"
)

type provider struct {
	// settings
	settings factory.ScopedProviderSettings

	// config
	config statsreporter.Config

	// used to aggregate stats for an organization
	aggregator statsreporter.Aggregator

	// used to get organizations
	orgGetter organization.Getter

	// used to get users
	userGetter user.Getter


	// used to send stats to an analytics backend
	analytics analytics.Analytics

	// used to get build information
	build version.Build

	// used to get deployment information
	deployment version.Deployment

	// used to stop the provider
	stopC chan struct{}
}

func NewFactory(aggregator statsreporter.Aggregator, orgGetter organization.Getter, userGetter user.Getter, build version.Build, analyticsConfig analytics.Config) factory.ProviderFactory[statsreporter.StatsReporter, statsreporter.Config] {
	return factory.NewProviderFactory(factory.MustNewName("analytics"), func(ctx context.Context, settings factory.ProviderSettings, config statsreporter.Config) (statsreporter.StatsReporter, error) {
		return New(ctx, settings, config, aggregator, orgGetter, userGetter, build, analyticsConfig)
	})
}

func New(
	ctx context.Context,
	providerSettings factory.ProviderSettings,
	config statsreporter.Config,
	aggregator statsreporter.Aggregator,
	orgGetter organization.Getter,
	userGetter user.Getter,
	build version.Build,
	analyticsConfig analytics.Config,
) (statsreporter.StatsReporter, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/statsreporter/analyticsstatsreporter")
	deployment := version.NewDeployment()
	analytics, err := segmentanalytics.New(ctx, providerSettings, analyticsConfig)
	if err != nil {
		return nil, err
	}

	return &provider{
		settings:   settings,
		config:     config,
		aggregator: aggregator,
		orgGetter:  orgGetter,
		userGetter: userGetter,
		analytics:  analytics,
		build:      build,
		deployment: deployment,
		stopC:      make(chan struct{}),
	}, nil
}

func (provider *provider) Start(ctx context.Context) error {
	go func() {
		if err := provider.analytics.Start(ctx); err != nil {
			provider.settings.Logger().ErrorContext(ctx, "failed to start analytics", errors.Attr(err))
		}
	}()

	ticker := time.NewTicker(provider.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-provider.stopC:
			return nil
		case <-ticker.C:
			ctx, span := provider.settings.Tracer().Start(ctx, "statsreporter.Report", trace.WithAttributes(attribute.String("statsreporter.provider", "analytics")))

			if err := provider.Report(ctx); err != nil {
				span.RecordError(err)
				provider.settings.Logger().WarnContext(ctx, "failed to report stats", errors.Attr(err))
			}

			span.End()
		}
	}
}

func (provider *provider) Report(ctx context.Context) error {
	orgs, err := provider.orgGetter.ListByOwnedKeyRange(ctx)
	if err != nil {
		return err
	}

	for _, org := range orgs {
		stats, err := provider.aggregator.Aggregate(ctx, org.ID)
		if err != nil {
			provider.settings.Logger().WarnContext(ctx, "failed to aggregate stats", errors.Attr(err), slog.Any("org_id", org.ID))
			continue
		}

		if len(stats) == 0 {
			provider.settings.Logger().WarnContext(ctx, "no stats collected", slog.Any("org_id", org.ID))
			continue
		}

		// Add build and deployment stats
		stats["build.version"] = provider.build.Version()
		stats["build.branch"] = provider.build.Branch()
		stats["build.hash"] = provider.build.Hash()
		stats["build.variant"] = provider.build.Variant()
		stats["deployment.mode"] = provider.deployment.Mode()
		stats["deployment.platform"] = provider.deployment.Platform()
		stats["deployment.os"] = provider.deployment.OS()
		stats["deployment.arch"] = provider.deployment.Arch()

		// Add org stats
		stats["display_name"] = org.DisplayName
		stats["name"] = org.Name
		stats["created_at"] = org.CreatedAt
		stats["alias"] = org.Alias

		provider.settings.Logger().DebugContext(ctx, "reporting stats", slog.Any("stats", stats))

		provider.analytics.IdentifyGroup(ctx, org.ID.String(), stats)
		provider.analytics.TrackGroup(ctx, org.ID.String(), "Stats Reported", stats)

		if !provider.config.Collect.Identities {
			continue
		}

		users, err := provider.userGetter.ListUsersByOrgID(ctx, org.ID)
		if err != nil {
			provider.settings.Logger().WarnContext(ctx, "failed to list users", errors.Attr(err), slog.Any("org_id", org.ID))
			continue
		}

		// The "last observed at" traits are gone with the tokenizer: they were
		// derived from o11y's OWN access tokens, and o11y no longer mints one.
		// Last-seen belongs to whoever mints the session — Hanzo IAM.
		for _, user := range users {
			provider.analytics.IdentifyUser(ctx, org.ID.String(), user.ID.String(), types.NewTraitsFromUser(user))
		}
	}

	return nil
}

func (provider *provider) Stop(ctx context.Context) error {
	close(provider.stopC)
	// report stats on stop
	if err := provider.Report(ctx); err != nil {
		provider.settings.Logger().WarnContext(ctx, "failed to report stats", errors.Attr(err))
	}

	if err := provider.analytics.Stop(ctx); err != nil {
		provider.settings.Logger().ErrorContext(ctx, "failed to stop analytics", errors.Attr(err))
	}

	return nil
}
