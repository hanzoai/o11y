package singlesharder

import (
	"context"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sharder"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type provider struct {
	settings factory.ScopedProviderSettings
	orgID    valuer.UUID
	orgIDKey uint32
}

func NewFactory() factory.ProviderFactory[sharder.Sharder, sharder.Config] {
	return factory.NewProviderFactory(factory.MustNewName("single"), New)
}

func New(ctx context.Context, providerSettings factory.ProviderSettings, config sharder.Config) (sharder.Sharder, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/sharder/singlesharder")

	return &provider{
		settings: settings,
		orgID:    config.Single.OrgID,
		orgIDKey: types.NewOrganizationKey(config.Single.OrgID),
	}, nil
}

func (provider *provider) GetMyOwnedKeyRange(ctx context.Context) (uint32, uint32, error) {
	return provider.orgIDKey, provider.orgIDKey, nil
}

func (provider *provider) IsMyOwnedKey(ctx context.Context, key uint32) error {
	if key == provider.orgIDKey {
		return nil
	}

	return errors.Newf(errors.TypeForbidden, errors.CodeForbidden, "key %d for org %s is not owned by my current instance", key, provider.orgID)
}
