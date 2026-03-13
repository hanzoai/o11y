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
	ruleStore ruletypes.RuleStore
}

func NewFactory(sqlstore sqlstore.SQLStore, queryParser queryparser.QueryParser) factory.ProviderFactory[ruler.Ruler, ruler.Config] {
	return factory.NewProviderFactory(factory.MustNewName("observe"), func(ctx context.Context, settings factory.ProviderSettings, config ruler.Config) (ruler.Ruler, error) {
		return New(ctx, settings, config, sqlstore, queryParser)
	})
}

func New(ctx context.Context, settings factory.ProviderSettings, config ruler.Config, sqlstore sqlstore.SQLStore, queryParser queryparser.QueryParser) (ruler.Ruler, error) {
	return &provider{ruleStore: sqlrulestore.NewRuleStore(sqlstore, queryParser, settings)}, nil
}

func (provider *provider) Collect(ctx context.Context, orgID valuer.UUID) (map[string]any, error) {
	rules, err := provider.ruleStore.GetStoredRules(ctx, orgID.String())
	if err != nil {
		return nil, err
	}

	return ruletypes.NewStatsFromRules(rules), nil
}
