package o11yruler

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/ruler/rulestore/sqlrulestore"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/ruletypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type provider struct {
	manager   *rules.Manager
	ruleStore ruletypes.RuleStore
	stopC     chan struct{}
	healthyC  chan struct{}
}

func NewFactory(sqlstore sqlstore.SQLStore, queryParser queryparser.QueryParser) factory.ProviderFactory[ruler.Ruler, ruler.Config] {
	return factory.NewProviderFactory(factory.MustNewName("observe"), func(ctx context.Context, settings factory.ProviderSettings, config ruler.Config) (ruler.Ruler, error) {
		return New(ctx, settings, config, sqlstore, queryParser)
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

	return ruletypes.NewStatsFromRules(rules), nil
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
