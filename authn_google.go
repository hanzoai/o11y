//go:build google

package o11y

import (
	"context"

	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authn/callbackauthn/googlecallbackauthn"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// registerOptionalAuthNs (google build) wires the Google OIDC callback
// provider. This file is the only entry point that imports the
// google-sdk-pulling subtree (cloud.google.com/go/auth → s2a-go → grpc),
// keeping the default-tag dep graph grpc-free.
func registerOptionalAuthNs(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, providers map[authtypes.AuthNProvider]authn.AuthN) error {
	googleCallbackAuthN, err := googlecallbackauthn.New(ctx, store, providerSettings)
	if err != nil {
		return err
	}
	providers[authtypes.AuthNProviderGoogleAuth] = googleCallbackAuthN
	return nil
}
