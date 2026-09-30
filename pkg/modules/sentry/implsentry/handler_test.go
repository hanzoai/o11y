package implsentry

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	gotEventID  string
	gotEventOrg valuer.UUID
	gotProject  valuer.UUID
}

func (s *stubSentry) GetEvent(_ context.Context, orgID, projectID valuer.UUID, eventID string) (*sentrytypes.Event, error) {
	s.gotEventOrg, s.gotProject, s.gotEventID = orgID, projectID, eventID
	return &sentrytypes.Event{}, nil
}

// The event route carries BOTH kinds of value: the event id is a PATH segment and
// the project is a QUERY value. Conflating them would read the wrong half of the
// URL and answer another project's event, so one request pins both.
func TestGetEventReadsThePathIDAndTheQueryProject(t *testing.T) {
	module := &stubSentry{}
	org, project := valuer.GenerateUUID(), valuer.GenerateUUID()

	router := routing.Serve(http.MethodGet, "/v1/o11y/sentinel/events/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		NewHandler(module).GetEvent(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{OrgID: org.String()})))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/o11y/sentinel/events/ev-42?project="+project.String(), http.NoBody)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ev-42", module.gotEventID)
	assert.Equal(t, project, module.gotProject)
	assert.Equal(t, org, module.gotEventOrg)
}
