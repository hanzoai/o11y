package implserviceaccount

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/serviceaccounttypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// These routes name THREE different path segments — {id}, {rid} and {fid} — and
// a handler that reads the wrong one still compiles and still returns 2xx, it
// just acts on the wrong resource. So each test drives a request whose segments
// are DISTINCT uuids and asserts which value landed where.
//
// Embedding the interface keeps the double to the methods these routes are
// allowed to reach — every other call is a nil call, which states "this route
// must not touch that".
type recordingModule struct {
	serviceaccount.Module
	serviceAccountID valuer.UUID
	roleID           valuer.UUID
	factorAPIKeyID   valuer.UUID
}

func (m *recordingModule) GetWithRoles(_ context.Context, _ valuer.UUID, id valuer.UUID) (*serviceaccounttypes.ServiceAccountWithRoles, error) {
	m.serviceAccountID = id
	return &serviceaccounttypes.ServiceAccountWithRoles{}, nil
}

func (m *recordingModule) Get(_ context.Context, _ valuer.UUID, id valuer.UUID) (*serviceaccounttypes.ServiceAccount, error) {
	m.serviceAccountID = id
	return &serviceaccounttypes.ServiceAccount{}, nil
}

func (m *recordingModule) DeleteRole(_ context.Context, _ valuer.UUID, id valuer.UUID, roleID valuer.UUID) error {
	m.serviceAccountID, m.roleID = id, roleID
	return nil
}

func (m *recordingModule) RevokeFactorAPIKey(_ context.Context, _ valuer.UUID, factorAPIKeyID valuer.UUID) error {
	m.factorAPIKeyID = factorAPIKeyID
	return nil
}

// servedServiceAccount drives one request through a REAL router registered at
// the template the service registers, so the segments under assertion are ones
// the router matched — injecting them by hand would prove only that the
// injector and the reader agree with each other.
func servedServiceAccount(t *testing.T, template, method, target string, pick func(serviceaccount.Handler) http.HandlerFunc) (*recordingModule, *httptest.ResponseRecorder) {
	t.Helper()

	module := new(recordingModule)
	router := routing.Serve(method, template, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pick(NewHandler(module))(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{
			UserID: valuer.GenerateUUID().String(),
			OrgID:  valuer.GenerateUUID().String(),
		})))
	}))

	req := httptest.NewRequest(method, target, http.NoBody)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return module, rec
}

func TestGetActsOnTheServiceAccountTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := servedServiceAccount(t, "/v1/o11y/service_accounts/{id}", http.MethodGet, "/v1/o11y/service_accounts/"+want.String(), func(h serviceaccount.Handler) http.HandlerFunc { return h.Get })

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, module.serviceAccountID)
}

// {id} and {rid} are two segments of one path: the account and the role it
// loses. Swapping them would revoke by the wrong identifier.
func TestDeleteRoleSeparatesTheAccountFromTheRole(t *testing.T) {
	account, role := valuer.GenerateUUID(), valuer.GenerateUUID()

	module, rec := servedServiceAccount(t, "/v1/o11y/service_accounts/{id}/roles/{rid}", http.MethodDelete, "/v1/o11y/service_accounts/"+account.String()+"/roles/"+role.String(), func(h serviceaccount.Handler) http.HandlerFunc { return h.DeleteRole })

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, account, module.serviceAccountID)
	assert.Equal(t, role, module.roleID)
}

// Same for {id} and {fid}: the account owning the key, and the key revoked.
func TestRevokeFactorAPIKeySeparatesTheAccountFromTheKey(t *testing.T) {
	account, key := valuer.GenerateUUID(), valuer.GenerateUUID()

	module, rec := servedServiceAccount(t, "/v1/o11y/service_accounts/{id}/keys/{fid}", http.MethodDelete, "/v1/o11y/service_accounts/"+account.String()+"/keys/"+key.String(), func(h serviceaccount.Handler) http.HandlerFunc { return h.RevokeFactorAPIKey })

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, account, module.serviceAccountID)
	assert.Equal(t, key, module.factorAPIKeyID)
}

// A segment that is not a uuid is refused before the store is reached — an
// empty or malformed id must never arrive as a lookup for "whatever the zero
// value matches".
func TestGetRefusesANonUUIDSegment(t *testing.T) {
	module, rec := servedServiceAccount(t, "/v1/o11y/service_accounts/{id}", http.MethodGet, "/v1/o11y/service_accounts/not-a-uuid", func(h serviceaccount.Handler) http.HandlerFunc { return h.Get })

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid uuid not-a-uuid")
	assert.Equal(t, valuer.UUID{}, module.serviceAccountID, "a refused id must not reach the store")
}
