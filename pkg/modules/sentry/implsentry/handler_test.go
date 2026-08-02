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

	router := routing.Serve(http.MethodGet, "/v1/sentry/events/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		NewHandler(module, true, false).GetEvent(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{OrgID: org.String()})))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sentry/events/ev-42?project="+project.String(), http.NoBody)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ev-42", module.gotEventID)
	assert.Equal(t, project, module.gotProject)
	assert.Equal(t, org, module.gotEventOrg)
}

// The ingest route is the one segment read that decides TENANCY: the project in
// the path is what the DSN key is verified against, so reading the wrong one
// would verify a key against the wrong project. The template is the runtime's,
// uuid constraint and all — that constraint is the router's business, the segment
// NAME is this read's.
func TestIngestVerifiesTheProjectTheRouterMatched(t *testing.T) {
	module := &stubSentry{}
	project := valuer.GenerateUUID()
	template := "/v1/sentry/{project:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}}/envelope/"

	router := routing.Serve(http.MethodPost, template, http.HandlerFunc(NewHandler(module, true, false).EnvelopeIngest))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sentry/"+project.String()+"/envelope/", strings.NewReader("{}")))

	// Fails closed on the key, which is the stub's answer — and it can only have
	// been asked about the project the router matched.
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "invalid ingest key")
	assert.Equal(t, project, module.gotIngestProj)
}
