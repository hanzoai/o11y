package implsentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// stubSentry answers only the calls under test; the embedded interface carries
// the rest, so this fake says exactly which method the route reaches.
type stubSentry struct {
	sentry.Module
	gotEventID    string
	gotEventOrg   valuer.UUID
	gotProject    valuer.UUID
	gotIngestProj valuer.UUID
}

func (s *stubSentry) GetEvent(_ context.Context, orgID, projectID valuer.UUID, eventID string) (*sentrytypes.Event, error) {
	s.gotEventOrg, s.gotProject, s.gotEventID = orgID, projectID, eventID
	return &sentrytypes.Event{}, nil
}

// ResolveIngest fails closed, which is what an unknown key does in production —
// the read under test happens before it.
func (s *stubSentry) ResolveIngest(_ context.Context, projectID valuer.UUID, _ string) (valuer.UUID, bool) {
	s.gotIngestProj = projectID
	return valuer.UUID{}, false
}

// The event route carries BOTH kinds of value: the event id is a PATH segment and
// the project is a QUERY value. Conflating them would read the wrong half of the
// URL and answer another project's event, so one request pins both.
func TestGetEventReadsThePathIDAndTheQueryProject(t *testing.T) {
	module := &stubSentry{}
	org, project := valuer.GenerateUUID(), valuer.GenerateUUID()

	router := routing.Serve(http.MethodGet, "/v1/sentinel/events/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		NewHandler(module, true, false).GetEvent(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{OrgID: org.String()})))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sentinel/events/ev-42?project="+project.String(), http.NoBody)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ev-42", module.gotEventID)
	assert.Equal(t, project, module.gotProject)
	assert.Equal(t, org, module.gotEventOrg)
}

// ingestRoute is the template the runtime registers, guid constraint and all.
// It is spelled here exactly as sentry.go spells it: a stale copy is not a
// smaller version of this test, it is a different route (see the constraint
// test below).
const ingestRoute = "/v1/sentinel/{project:guid}/envelope/"

// The ingest route is the one segment read that decides TENANCY: the project in
// the path is what the DSN key is verified against, so reading the wrong one
// would verify a key against the wrong project.
func TestIngestVerifiesTheProjectTheRouterMatched(t *testing.T) {
	module := &stubSentry{}
	project := valuer.GenerateUUID()

	router := routing.Serve(http.MethodPost, ingestRoute, http.HandlerFunc(NewHandler(module, true, false).EnvelopeIngest))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sentinel/"+project.String()+"/envelope/", strings.NewReader("{}")))

	// Fails closed on the key, which is the stub's answer — and it can only have
	// been asked about the project the router matched.
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "invalid ingest key")
	assert.Equal(t, project, module.gotIngestProj)
}

// The constraint is the whole reason the ingest wildcard can never shadow a
// static /v1/sentinel resource word, so a non-uuid segment must not reach the
// handler at all. This is pinned because the constraint is silently OPTIONAL:
// the router names its constraints, an unrecognised name is DROPPED rather than
// refused, and a dropped constraint matches everything. Spelling this route with
// the hand-written character class the runtime used to carry produces exactly
// that — a route that answers /v1/sentinel/projects/envelope/ with 418 instead of
// 404 — which is why the template above is the runtime's own `guid` and is
// asserted here rather than trusted.
func TestIngestConstraintRefusesANonUUIDProject(t *testing.T) {
	module := &stubSentry{}

	router := routing.Serve(http.MethodPost, ingestRoute, http.HandlerFunc(NewHandler(module, true, false).EnvelopeIngest))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sentinel/projects/envelope/", strings.NewReader("{}")))

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, valuer.UUID{}, module.gotIngestProj, "a segment the constraint refuses must not reach the handler")
}
