package o11y

import (
	"context"

	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authn/passwordauthn/emailpasswordauthn"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/licensing"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// NewAuthNs returns the per-provider AuthN map for o11y.
//
// Only the email/password provider is registered in o11y itself. All
// external-identity providers (Google OIDC, SAML, OIDC discovery)
// happen at the Hanzo IAM layer — o11y delegates via pkg/authz/iamauthz.
// No Google SDK in the o11y dep graph.
func NewAuthNs(_ context.Context, _ factory.ProviderSettings, store authtypes.AuthNStore, _ licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
	return map[authtypes.AuthNProvider]authn.AuthN{
		authtypes.AuthNProviderEmailPassword: emailpasswordauthn.New(store),
	}, nil
}
