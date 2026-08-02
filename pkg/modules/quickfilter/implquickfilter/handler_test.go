package implquickfilter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/quickfilter"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/quickfiltertypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The signal is a PATH segment of the registered template, and it selects which
// of the org's five filter sets is returned. Driving a real router at the real
// template is what proves the handler reads the name the route declares.
const signalRoute = "/v1/o11y/orgs/me/filters/{signal}"

// recordingModule records the signal the handler resolved. Every other method is
// left to the nil embedded interface, so an unexpected call panics rather than
// passing quietly.
type recordingModule struct {
	quickfilter.Module
	signal quickfiltertypes.Signal
}

func (m *recordingModule) GetSignalFilters(_ context.Context, _ valuer.UUID, signal quickfiltertypes.Signal) (*quickfiltertypes.SignalFilters, error) {
	m.signal = signal
	return &quickfiltertypes.SignalFilters{Signal: signal}, nil
}

func serveSignal(t *testing.T, module quickfilter.Module, target string) *httptest.ResponseRecorder {
	t.Helper()

	router := mux.NewRouter()
	router.HandleFunc(signalRoute, NewHandler(module).GetSignalFilters).Methods(http.MethodGet)

	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	req = req.WithContext(authtypes.NewContextWithClaims(req.Context(), authtypes.Claims{
		OrgID:     valuer.GenerateUUID().String(),
		UserID:    valuer.GenerateUUID().String(),
		Email:     "u@acme",
		Principal: authtypes.PrincipalUser,
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGetSignalFiltersActsOnTheSegmentTheRouterMatched(t *testing.T) {
	module := new(recordingModule)

	rec := serveSignal(t, module, "/v1/o11y/orgs/me/filters/logs")

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, quickfiltertypes.SignalLogs, module.signal)
}

// A signal the service does not know is refused before the lookup — an unread
// segment must never be answered as some default signal's filters.
func TestGetSignalFiltersRefusesAnUnknownSignalSegment(t *testing.T) {
	module := new(recordingModule)

	rec := serveSignal(t, module, "/v1/o11y/orgs/me/filters/notasignal")

	require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, quickfiltertypes.Signal{}, module.signal, "the module must not be reached")
}
