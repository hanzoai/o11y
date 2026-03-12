package emailpasswordauthn

import (
	"context"

	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

var _ authn.PasswordAuthN = (*AuthN)(nil)

type AuthN struct {
	store authtypes.AuthNStore
}

func New(store authtypes.AuthNStore) *AuthN {
	return &AuthN{store: store}
}

func (a *AuthN) Authenticate(ctx context.Context, email string, password string, orgID valuer.UUID) (*authtypes.Identity, error) {
	user, factorPassword, err := a.store.GetActiveUserAndFactorPasswordByEmailAndOrgID(ctx, email, orgID)
	if err != nil {
		return nil, err
	}

	if !factorPassword.Equals(password) {
		return nil, errors.New(errors.TypeUnauthenticated, types.ErrCodeIncorrectPassword, "invalid email or password")
	}

	return authtypes.NewIdentity(user.ID, orgID, user.Email, user.Role), nil
}
