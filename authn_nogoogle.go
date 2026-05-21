//go:build !google

package o11y

import (
	"context"

	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// registerOptionalAuthNs is a no-op in the default build — Google OIDC
// happens at the Hanzo IAM layer, not inside o11y.
func registerOptionalAuthNs(_ context.Context, _ factory.ProviderSettings, _ authtypes.AuthNStore, _ map[authtypes.AuthNProvider]authn.AuthN) error {
	return nil
}
