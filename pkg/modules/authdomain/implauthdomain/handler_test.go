package implauthdomain

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

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/authdomain"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const domainRoute = "/v1/o11y/domains/{id}"

// The per-domain routes address a domain by a PATH segment, so what has to hold
// is that the segment the ROUTER matched is the domain the handler acts on.
// Embedding the interface keeps the double to the methods these routes are
// allowed to reach — every other call is a nil call, which states "this route
// must not touch that".
type recordingModule struct {
	authdomain.Module
	domainID valuer.UUID
}

func (m *recordingModule) Delete(_ context.Context, _ valuer.UUID, domainID valuer.UUID) error {
	m.domainID = domainID
	return nil
}

// A domain the store does not hold is a real answer for a real id, and it lets
// the read be asserted without standing up an auth domain.
func (m *recordingModule) GetByOrgIDAndID(_ context.Context, _ valuer.UUID, domainID valuer.UUID) (*authtypes.AuthDomain, error) {
	m.domainID = domainID
	return nil, errors.New(errors.TypeNotFound, errors.CodeNotFound, "no such auth domain")
}

// servedDomain drives one request through a REAL router registered at the
// template the service registers, so the segment under assertion is one the
// router matched — injecting it by hand would prove only that the injector and
// the reader agree with each other.
func servedDomain(t *testing.T, method, target, body string, pick func(authdomain.Handler) http.HandlerFunc) (*recordingModule, *httptest.ResponseRecorder) {
	t.Helper()

	module := new(recordingModule)
	router := mux.NewRouter()
	router.HandleFunc(domainRoute, pick(NewHandler(module))).Methods(method)

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req = req.WithContext(authtypes.NewContextWithClaims(req.Context(), authtypes.Claims{
		UserID: valuer.GenerateUUID().String(),
		OrgID:  valuer.GenerateUUID().String(),
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return module, rec
}

func TestDeleteActsOnTheDomainTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := servedDomain(t, http.MethodDelete, "/v1/o11y/domains/"+want.String(), "", func(h authdomain.Handler) http.HandlerFunc { return h.Delete })

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, want, module.domainID)
}

func TestGetLooksUpTheDomainTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, _ := servedDomain(t, http.MethodGet, "/v1/o11y/domains/"+want.String(), "", func(h authdomain.Handler) http.HandlerFunc { return h.Get })

	assert.Equal(t, want, module.domainID)
}

func TestUpdateLooksUpTheDomainTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, _ := servedDomain(t, http.MethodPut, "/v1/o11y/domains/"+want.String(), `{}`, func(h authdomain.Handler) http.HandlerFunc { return h.Update })

	assert.Equal(t, want, module.domainID)
}

// A segment that is not a uuid is refused before the store is reached — an
// empty or malformed id must never arrive as a lookup for "whatever the zero
// value matches".
func TestDeleteRefusesANonUUIDSegment(t *testing.T) {
	module, rec := servedDomain(t, http.MethodDelete, "/v1/o11y/domains/not-a-uuid", "", func(h authdomain.Handler) http.HandlerFunc { return h.Delete })

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid uuid not-a-uuid")
	assert.Equal(t, valuer.UUID{}, module.domainID, "a refused id must not reach the store")
}
