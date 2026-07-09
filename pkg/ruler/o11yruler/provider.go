package o11yruler

import (
	"context"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/alertmanager/alertmanagerstore/sqlalertmanagerstore"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/query-service/rules"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/ruler/rulestore/sqlrulestore"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/types/alertmanagertypes"
	"github.com/hanzoai/o11y/pkg/types/ruletypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type provider struct {
	manager   *rules.Manager
	ruleStore ruletypes.RuleStore
	stopC     chan struct{}
	healthyC  chan struct{}
}

func NewFactory(
	cache cache.Cache,
	alertmanager alertmanager.Alertmanager,
	sqlstore sqlstore.SQLStore,
	telemetryStore telemetrystore.TelemetryStore,
	metadataStore telemetrytypes.MetadataStore,
	prometheus prometheus.Prometheus,
	orgGetter organization.Getter,
	ruleStateHistoryModule rulestatehistory.Module,
	querier querier.Querier,
	queryParser queryparser.QueryParser,
	prepareTaskFunc func(rules.PrepareTaskOptions) (rules.Task, error),
	prepareTestRuleFunc func(rules.PrepareTestRuleOptions) (int, error),
) factory.ProviderFactory[ruler.Ruler, ruler.Config] {
	return factory.NewProviderFactory(factory.MustNewName("o11y"), func(ctx context.Context, providerSettings factory.ProviderSettings, config ruler.Config) (ruler.Ruler, error) {
		ruleStore := sqlrulestore.NewRuleStore(sqlstore, queryParser, providerSettings)
		maintenanceStore := sqlalertmanagerstore.NewMaintenanceStore(sqlstore, providerSettings)

		managerOpts := &rules.ManagerOptions{
			TelemetryStore:         telemetryStore,
			MetadataStore:          metadataStore,
			Prometheus:             prometheus,
			Context:                context.Background(),
			Querier:                querier,
			Logger:                 providerSettings.Logger,
			Cache:                  cache,
			EvalDelay:              valuer.MustParseTextDuration(config.EvalDelay.String()),
			PrepareTaskFunc:        prepareTaskFunc,
			PrepareTestRuleFunc:    prepareTestRuleFunc,
			Alertmanager:           alertmanager,
			OrgGetter:              orgGetter,
			RuleStore:              ruleStore,
			MaintenanceStore:       maintenanceStore,
			SQLStore:               sqlstore,
			QueryParser:            queryParser,
			RuleStateHistoryModule: ruleStateHistoryModule,
		}

		manager, err := rules.NewManager(managerOpts)
		if err != nil {
			return nil, err
		}

		return &provider{manager: manager, ruleStore: ruleStore, stopC: make(chan struct{}), healthyC: make(chan struct{})}, nil
	})
}

func (provider *provider) Start(ctx context.Context) error {
	provider.manager.Start(ctx)
	close(provider.healthyC)
	<-provider.stopC
	return nil
}

func (provider *provider) Healthy() <-chan struct{} {
	return provider.healthyC
}

func (provider *provider) Stop(ctx context.Context) error {
	close(provider.stopC)
	provider.manager.Stop(ctx)
	return nil
}

func (provider *provider) Collect(ctx context.Context, orgID valuer.UUID) (map[string]any, error) {
	rules, err := provider.ruleStore.GetStoredRules(ctx, orgID.String())
	if err != nil {
		return nil, err
	}

	stats := ruletypes.NewStatsFromRules(rules)

	alertStats := provider.manager.AlertStats(ctx)
	stats["alert.firing.count"] = alertStats.FiringRules
	if !alertStats.LastFiredAt.IsZero() {
		stats["alert.last_fired.time"] = alertStats.LastFiredAt.UTC()
		stats["alert.last_fired.time_unix"] = alertStats.LastFiredAt.Unix()
	}

	return stats, nil
}

func (provider *provider) ListRuleStates(ctx context.Context) (*ruletypes.GettableRules, error) {
	return provider.manager.ListRuleStates(ctx)
}

func (provider *provider) GetRule(ctx context.Context, id valuer.UUID) (*ruletypes.GettableRule, error) {
	return provider.manager.GetRule(ctx, id)
}

func (provider *provider) CreateRule(ctx context.Context, ruleStr string) (*ruletypes.GettableRule, error) {
	return provider.manager.CreateRule(ctx, ruleStr)
}

func (provider *provider) EditRule(ctx context.Context, ruleStr string, id valuer.UUID) error {
	return provider.manager.EditRule(ctx, ruleStr, id)
}

func (provider *provider) DeleteRule(ctx context.Context, idStr string) error {
	return provider.manager.DeleteRule(ctx, idStr)
}

func (provider *provider) PatchRule(ctx context.Context, ruleStr string, id valuer.UUID) (*ruletypes.GettableRule, error) {
	return provider.manager.PatchRule(ctx, ruleStr, id)
}

func (provider *provider) TestNotification(ctx context.Context, orgID valuer.UUID, ruleStr string) (int, error) {
	return provider.manager.TestNotification(ctx, orgID, ruleStr)
}

func (provider *provider) MaintenanceStore() alertmanagertypes.MaintenanceStore {
	return provider.manager.MaintenanceStore()
}
