package user

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The user module is a PROJECTION of a Hanzo IAM identity, not an identity.
//
// Nothing here authenticates. There is no password, no invite, no reset token,
// no session — those were deleted with pkg/authn, pkg/tokenizer and
// pkg/modules/session, because o11y asserting who you are is a second answer to
// a question IAM already answered, and two answers is one too many.
//
// What remains exists for one reason: o11y's OWN rows — a dashboard's author, a
// saved view, a preference, a role grant — are keyed by user id, and a foreign
// key needs something to point at. iamidentn writes that row on first sight of
// an IAM subject, keyed by IAM's own subject id. Everything below is either
// that write, a read of it by o11y's own bookkeeping (stats, the role-deletion
// guard), or the one read the console makes of ITSELF — and that one is served
// from the CLAIMS, not from the row.
type Setter interface {
	// CreateUser writes the projection row and grants it the roles named in
	// opts. The ONLY caller is identn/iamidentn on first sight of an IAM subject;
	// the grant and the durable user_role entries happen in this one call, which
	// is what localauthz rehydrates from after a restart.
	CreateUser(ctx context.Context, user *types.User, opts ...CreateUserOption) error

	statsreporter.StatsCollector
}

type Getter interface {
	// GetRootUserByOrgID resolves the org's root row — the single principal the
	// impersonation identN resolver stands in for in local single-user mode.
	GetRootUserByOrgID(context.Context, valuer.UUID) (*types.User, []*authtypes.UserRole, error)

	// GetUserByOrgIDAndID answers whether iamidentn has already seated this
	// subject, so the founding write happens once.
	GetUserByOrgIDAndID(ctx context.Context, orgID valuer.UUID, userID valuer.UUID) (*types.User, error)

	// ListUsersByOrgID, CountByOrgID and CountByOrgIDAndStatuses feed the stats
	// reporter — how many members a tenant has, not who they are.
	ListUsersByOrgID(ctx context.Context, orgID valuer.UUID) ([]*types.User, error)
	CountByOrgID(context.Context, valuer.UUID) (int64, error)
	CountByOrgIDAndStatuses(context.Context, valuer.UUID, []string) (map[valuer.String]int64, error)

	// GetUsersByOrgIDAndRoleID backs OnBeforeRoleDelete.
	GetUsersByOrgIDAndRoleID(ctx context.Context, orgID valuer.UUID, roleID valuer.UUID) ([]*types.User, error)

	// OnBeforeRoleDelete refuses to delete a role that principals still hold.
	OnBeforeRoleDelete(ctx context.Context, orgID valuer.UUID, roleID valuer.UUID, roleName string) error
}

type Handler interface {
	// GetMyUser answers GET /v1/o11y/users/me from the request's CLAIMS. It is
	// the console's identity provider and the only user route o11y still serves.
	GetMyUser(http.ResponseWriter, *http.Request)
}
