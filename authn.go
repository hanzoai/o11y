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
// Default builds register only the email/password provider. Optional
// external-identity providers (Google OIDC via googlecallbackauthn) sit
// behind build tags because their SDKs (google.golang.org/api,
// cloud.google.com/go/auth) transitively pull google.golang.org/grpc.
// Hanzo IAM handles OIDC at the IAM layer for the canonical deployment.
//
// To re-enable Google OIDC inside o11y: build with -tags google. The
// extra providers register in authn_google.go.
func NewAuthNs(ctx context.Context, providerSettings factory.ProviderSettings, store authtypes.AuthNStore, licensing licensing.Licensing) (map[authtypes.AuthNProvider]authn.AuthN, error) {
	emailPasswordAuthN := emailpasswordauthn.New(store)

	providers := map[authtypes.AuthNProvider]authn.AuthN{
		authtypes.AuthNProviderEmailPassword: emailPasswordAuthN,
	}

	if err := registerOptionalAuthNs(ctx, providerSettings, store, providers); err != nil {
		return nil, err
	}
	return providers, nil
}
