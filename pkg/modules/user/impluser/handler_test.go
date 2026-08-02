package impluser

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	root "github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The per-user routes address a user by a PATH segment, and one of them,
// /users/{id}/roles/{roleId}, carries TWO. A handler that reads the wrong name
// still compiles and still returns 2xx, it just acts on the wrong user — so
// each test drives DISTINCT uuids through a REAL router and asserts which value
// landed where.
//
// Embedding the interface keeps the doubles to the methods these routes are
// allowed to reach — every other call is a nil call, which states "this route
// must not touch that".
type recordingSetter struct {
	root.Setter
	userID valuer.UUID
	roleID valuer.UUID
}

func (s *recordingSetter) UpdateUser(_ context.Context, _ valuer.UUID, userID valuer.UUID, _ *types.UpdatableUser) (*types.User, error) {
	s.userID = userID
	return &types.User{}, nil
}

func (s *recordingSetter) RemoveUserRole(_ context.Context, _ valuer.UUID, userID valuer.UUID, roleID valuer.UUID) error {
	s.userID, s.roleID = userID, roleID
	return nil
}

type recordingGetter struct {
	root.Getter
	userID valuer.UUID
}

func (g *recordingGetter) GetResetPasswordTokenByOrgIDAndUserID(_ context.Context, _ valuer.UUID, userID valuer.UUID) (*types.ResetPasswordToken, error) {
	g.userID = userID
	return &types.ResetPasswordToken{}, nil
}

// servedUser drives one request through a REAL router registered at the
// template the service registers, so the segments under assertion are ones the
// router matched — injecting them by hand would prove only that the injector
// and the reader agree with each other.
func servedUser(t *testing.T, template, method, target, body string, claims authtypes.Claims, pick func(root.Handler) http.HandlerFunc) (*recordingSetter, *recordingGetter, *httptest.ResponseRecorder) {
	t.Helper()

	setter, getter := new(recordingSetter), new(recordingGetter)
	router := mux.NewRouter()
	router.HandleFunc(template, pick(NewHandler(setter, getter))).Methods(method)

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req = req.WithContext(authtypes.NewContextWithClaims(req.Context(), claims))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return setter, getter, rec
}

func someoneElse(t *testing.T) authtypes.Claims {
	t.Helper()
	return authtypes.Claims{UserID: valuer.GenerateUUID().String(), OrgID: valuer.GenerateUUID().String()}
}

func TestUpdateUserActsOnTheUserTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	setter, _, rec := servedUser(t, "/v1/o11y/users/{id}", http.MethodPut, "/v1/o11y/users/"+want.String(), `{"displayName":"renamed"}`, someoneElse(t), func(h root.Handler) http.HandlerFunc { return h.UpdateUser })

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, want, setter.userID)
}

// The self-modification guard compares the PATH segment against the caller's
// own id, so the guard only holds while that segment is read correctly: read
// the wrong name and it compares "" to the caller and lets the edit through.
func TestUpdateUserRefusesTheCallerEditingSelfByPath(t *testing.T) {
	self := valuer.GenerateUUID()
	claims := authtypes.Claims{UserID: self.String(), OrgID: valuer.GenerateUUID().String()}

	setter, _, rec := servedUser(t, "/v1/o11y/users/{id}", http.MethodPut, "/v1/o11y/users/"+self.String(), `{"displayName":"renamed"}`, claims, func(h root.Handler) http.HandlerFunc { return h.UpdateUser })

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "users cannot call this api on self")
	assert.Equal(t, valuer.UUID{}, setter.userID, "a refused edit must not reach the store")
}

// {id} and {roleId} are two segments of one path: the user, and the role they
// lose. Swapping them would revoke by the wrong identifier.
func TestRemoveUserRoleSeparatesTheUserFromTheRole(t *testing.T) {
	user, role := valuer.GenerateUUID(), valuer.GenerateUUID()

	setter, _, rec := servedUser(t, "/v1/o11y/users/{id}/roles/{roleId}", http.MethodDelete, "/v1/o11y/users/"+user.String()+"/roles/"+role.String(), "", someoneElse(t), func(h root.Handler) http.HandlerFunc { return h.RemoveUserRoleByRoleID })

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, user, setter.userID)
	assert.Equal(t, role, setter.roleID)
}

func TestGetResetPasswordTokenReadsTheUserTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	_, getter, rec := servedUser(t, "/v1/o11y/users/{id}/reset_password_tokens", http.MethodGet, "/v1/o11y/users/"+want.String()+"/reset_password_tokens", "", someoneElse(t), func(h root.Handler) http.HandlerFunc { return h.GetResetPasswordToken })

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, getter.userID)
}

// A segment that is not a uuid is refused before the store is reached — an
// empty or malformed id must never arrive as a lookup for "whatever the zero
// value matches".
func TestGetResetPasswordTokenRefusesANonUUIDSegment(t *testing.T) {
	_, getter, rec := servedUser(t, "/v1/o11y/users/{id}/reset_password_tokens", http.MethodGet, "/v1/o11y/users/not-a-uuid/reset_password_tokens", "", someoneElse(t), func(h root.Handler) http.HandlerFunc { return h.GetResetPasswordToken })

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid uuid not-a-uuid")
	assert.Equal(t, valuer.UUID{}, getter.userID, "a refused id must not reach the store")
}
