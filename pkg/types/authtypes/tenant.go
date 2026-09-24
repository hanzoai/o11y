package authtypes

import (
	"context"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
)

// The tenant is the org slug every warehouse row carries in its `org` column
// (event.log, event.span). A read of those tables binds `org = ?` to it, so a
// caller only ever reads the rows of the org it was admitted as.
//
// For a request it is the validated X-Org-Id the Hanzo IAM provider resolved
// (Identity.Tenant); for a scheduled rule it is the name of the rule's own org.
type tenantKey struct{}

// NewContextWithTenant attaches the tenant a warehouse read is scoped to.
func NewContextWithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantKey{}, strings.TrimSpace(tenant))
}

// TenantFromContext returns the tenant, or a forbidden error when none was
// attached: a warehouse read without one would read every org's rows.
func TenantFromContext(ctx context.Context) (string, error) {
	tenant, _ := ctx.Value(tenantKey{}).(string)
	if tenant == "" {
		return "", errors.NewForbiddenf(errors.CodeForbidden, "no tenant: a telemetry read must be scoped to one org")
	}
	return tenant, nil
}
