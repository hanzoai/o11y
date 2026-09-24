package querybuilder

import (
	"context"

	"github.com/hanzo-ds/sqlbuilder"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// TenantCondition is the `org = ?` condition that scopes a read of an event
// table (event.log, event.span) to the tenant on ctx. It fails when ctx names no
// tenant, so a read that would span every org is refused rather than run.
func TenantCondition(ctx context.Context, sb *sqlbuilder.SelectBuilder) (string, error) {
	tenant, err := authtypes.TenantFromContext(ctx)
	if err != nil {
		return "", err
	}
	return sb.E("org", tenant), nil
}
