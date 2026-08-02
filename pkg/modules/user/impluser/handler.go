package impluser

import (
	"context"
	"net/http"
	"time"

	"github.com/hanzoai/o11y/pkg/http/render"
	root "github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type handler struct{}

// NewHandler takes nothing, and that is the point. The user handler used to
// hold a Setter and a Getter because it wrote passwords, minted invites, listed
// members and edited roles. It performs none of those now, and the one route it
// still answers reads no table at all.
func NewHandler() root.Handler {
	return &handler{}
}

// GetMyUser answers with the identity the EDGE asserted, projected into the
// shape the console already reads.
//
// It used to SELECT the caller's row and 404 when it was missing — which made
// o11y's own bookkeeping a precondition for rendering the app. A person the
// gateway had authenticated, whose row had not been seated yet or had been
// removed, got "user not found" on the very call the console uses to learn who
// it is rendering for, and the whole console was unreachable. The identity was
// never in doubt; the ROW was, and the row is not the identity.
//
// So it reads the claims. Same fields, one source, and no failure mode between
// "the edge says you are signed in" and "the console shows you signed in":
//
//	id           the IAM subject          — claims.UserID
//	orgId        the IAM org (owner)      — claims.OrgID
//	email        the asserted address     — claims.Email
//	displayName  the same address, which is exactly what the seated row carried
//	             (iamidentn builds it as NewUserWithID(id, email, email, …)), so
//	             this is not a downgrade — it is the same value without the hop
//	isRoot       false. Root is a local-single-user notion; an IAM session is
//	             never root, and the row never set it either
//	status       active. The edge does not forward a session for a user who is
//	             not, and o11y has no status of its own to disagree with
//
// createdAt/updatedAt are left zero: they described the ROW's lifetime, never
// the person's, and the console renders neither. Inventing a timestamp here
// would be inventing a fact.
//
// No roles ride along. They used to, and nothing read them: the console resolves
// what it may do through POST /v1/o11y/authz/check, which is the one way. A
// second copy of the answer here could only ever disagree with it.
func (handler *handler) GetMyUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(w, err)
		return
	}

	userID, err := valuer.NewUUID(claims.UserID)
	if err != nil {
		render.Error(w, err)
		return
	}

	orgID, err := valuer.NewUUID(claims.OrgID)
	if err != nil {
		render.Error(w, err)
		return
	}

	email, err := valuer.NewEmail(claims.Email)
	if err != nil {
		email = valuer.Email{}
	}

	render.Success(w, http.StatusOK, &types.User{
		Identifiable: types.Identifiable{ID: userID},
		DisplayName:  claims.Email,
		Email:        email,
		OrgID:        orgID,
		IsRoot:       false,
		Status:       types.UserStatusActive,
	})
}
