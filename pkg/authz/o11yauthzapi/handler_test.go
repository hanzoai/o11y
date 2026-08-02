package o11yauthzapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const roleRoute = "/v1/o11y/roles/{id}"

// The role routes address a role by a PATH segment, so what has to hold is that
// the segment the ROUTER matched is the role the handler acts on. Embedding the
// interface keeps the double to the two methods these routes are allowed to
// reach — every other call is a nil call, which states "this route must not
// touch that".
type recordingAuthZ struct {
	authz.AuthZ
	roleID valuer.UUID
}

func (a *recordingAuthZ) GetWithTransactionGroups(_ context.Context, _ valuer.UUID, roleID valuer.UUID) (*authtypes.RoleWithTransactionGroups, error) {
	a.roleID = roleID
	return &authtypes.RoleWithTransactionGroups{}, nil
}

func (a *recordingAuthZ) Delete(_ context.Context, _ valuer.UUID, roleID valuer.UUID) error {
	a.roleID = roleID
	return nil
}

// servedRole drives one request through a REAL router registered at the
// template the service registers, so the segment under assertion is one the
// router matched — injecting it by hand would prove only that the injector and
// the reader agree with each other.
func servedRole(t *testing.T, method, target string, pick func(authz.Handler) http.HandlerFunc) (*recordingAuthZ, *httptest.ResponseRecorder) {
	t.Helper()

	authZ := new(recordingAuthZ)
	router := routing.Serve(method, roleRoute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pick(NewHandler(authZ))(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{
			UserID: valuer.GenerateUUID().String(),
			OrgID:  valuer.GenerateUUID().String(),
		})))
	}))

	req := httptest.NewRequest(method, target, http.NoBody)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return authZ, rec
}

func TestGetActsOnTheRoleTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	authZ, rec := servedRole(t, http.MethodGet, "/v1/o11y/roles/"+want.String(), func(h authz.Handler) http.HandlerFunc { return h.Get })

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, authZ.roleID)
}

func TestDeleteActsOnTheRoleTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	authZ, rec := servedRole(t, http.MethodDelete, "/v1/o11y/roles/"+want.String(), func(h authz.Handler) http.HandlerFunc { return h.Delete })

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, want, authZ.roleID)
}

// A segment that is not a uuid is refused before the role store is reached — an
// empty or malformed id must never arrive as a lookup for "whatever the zero
// value matches".
func TestGetRefusesANonUUIDSegment(t *testing.T) {
	authZ, rec := servedRole(t, http.MethodGet, "/v1/o11y/roles/not-a-uuid", func(h authz.Handler) http.HandlerFunc { return h.Get })

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid uuid not-a-uuid")
	assert.Equal(t, valuer.UUID{}, authZ.roleID, "a refused id must not reach the store")
}
