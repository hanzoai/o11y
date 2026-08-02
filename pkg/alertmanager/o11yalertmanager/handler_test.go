package o11yalertmanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/types/alertmanagertypes"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// stubAlertmanager answers only the calls under test; the embedded interface
// carries the rest, so this fake says exactly which method the route reaches.
type stubAlertmanager struct {
	alertmanager.Alertmanager
	gotOrgID     string
	gotChannelID valuer.UUID
	gotPolicyID  string
}

func (s *stubAlertmanager) GetChannelByID(_ context.Context, orgID string, id valuer.UUID) (*alertmanagertypes.Channel, error) {
	s.gotOrgID, s.gotChannelID = orgID, id
	return &alertmanagertypes.Channel{}, nil
}

func (s *stubAlertmanager) GetRoutePolicyByID(_ context.Context, policyID string) (*alertmanagertypes.GettableRoutePolicy, error) {
	s.gotPolicyID = policyID
	return &alertmanagertypes.GettableRoutePolicy{}, nil
}

// serve drives one request through a REAL router registered at the REAL template.
// Injecting the segment instead would prove only that the two halves of coretypes
// agree with each other; what has to hold is that the handler acts on the segment
// the ROUTER matched.
func serve(t *testing.T, template, target string, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	router := routing.Serve(http.MethodGet, template, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{OrgID: "org-1"})))
	}))

	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// A channel is addressed by a uuid PATH segment, and the org it belongs to comes
// from the caller's identity — never from the URL. Both must arrive at the store.
func TestGetChannelActsOnTheChannelTheRouterMatched(t *testing.T) {
	am := &stubAlertmanager{}
	want := valuer.GenerateUUID()

	rec := serve(t, "/v1/o11y/channels/{id}", "/v1/o11y/channels/"+want.String(), NewHandler(am).GetChannelByID)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, am.gotChannelID)
	assert.Equal(t, "org-1", am.gotOrgID)
}

// A segment that is not a uuid is refused before the store is touched: a
// malformed id must never become a lookup for the zero uuid.
func TestGetChannelRefusesANonUUIDSegment(t *testing.T) {
	am := &stubAlertmanager{}

	rec := serve(t, "/v1/o11y/channels/{id}", "/v1/o11y/channels/not-a-uuid", NewHandler(am).GetChannelByID)

	assert.Contains(t, rec.Body.String(), "id is not a valid uuid-v7")
	assert.Equal(t, valuer.UUID{}, am.gotChannelID, "the store must not be asked about a malformed id")
}

// A route policy is named by a raw PATH segment rather than a uuid, and it is the
// same read — one param model, whatever the value's shape.
func TestGetRoutePolicyActsOnThePolicyTheRouterMatched(t *testing.T) {
	am := &stubAlertmanager{}

	rec := serve(t, "/v1/o11y/route_policies/{id}", "/v1/o11y/route_policies/policy-7?id=policy-9", NewHandler(am).GetRoutePolicyByID)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "policy-7", am.gotPolicyID, "the path segment answers, not the query value of the same name")
}
