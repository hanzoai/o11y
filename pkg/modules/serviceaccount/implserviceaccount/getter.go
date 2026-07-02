package implserviceaccount

import (
	"context"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/serviceaccounttypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type getter struct {
	store serviceaccounttypes.Store
}

func NewGetter(store serviceaccounttypes.Store) serviceaccount.Getter {
	return &getter{store: store}
}

func (getter *getter) OnBeforeRoleDelete(ctx context.Context, orgID valuer.UUID, roleID valuer.UUID, _ string) error {
	serviceAccounts, err := getter.store.GetServiceAccountsByOrgIDAndRoleID(ctx, orgID, roleID)
	if err != nil {
		return err
	}
	if len(serviceAccounts) > 0 {
		return errors.New(errors.TypeInvalidInput, authtypes.ErrCodeRoleHasServiceAccountAssignees, "role has active service account assignments, remove them before deleting")
	}
	return nil
}
