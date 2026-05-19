package o11y

import (
	"context"

	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authn/callbackauthn/googlecallbackauthn"
	"github.com/hanzoai/o11y/pkg/authn/passwordauthn/emailpasswordauthn"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/licensing"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

func NewAuthNs(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, licensing licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
	emailPasswordAuthN := emailpasswordauthn.New(store)

	googleCallbackAuthN, err := googlecallbackauthn.New(ctx, store, providerSettings)
	if err != nil {
		return nil, err
	}

	return map[authtypes.AuthNProvider]authn.AuthN{
		authtypes.AuthNProviderEmailPassword: emailPasswordAuthN,
		authtypes.AuthNProviderGoogleAuth:    googleCallbackAuthN,
	}, nil
}
