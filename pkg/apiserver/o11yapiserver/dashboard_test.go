package o11yapiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The two anonymous public dashboard routes have no claims to name their
// subject: the gate resolves it from the {id} PATH segment before any identity
// exists. So what has to hold is that the id the ROUTER matched is the id the
// gate asks about — read the wrong half of the URL here and the gate authorizes
// against the zero uuid.
type recordingDashboardModule struct {
	dashboard.Module // only the one method the gate reaches is implemented

	gotID  valuer.UUID
	called bool
}

func (module *recordingDashboardModule) GetPublicDashboardSelectorsAndOrg(_ context.Context, id valuer.UUID, _ []*types.Organization) ([]coretypes.Selector, valuer.UUID, error) {
	module.gotID, module.called = id, true
	return nil, valuer.UUID{}, nil
}

// servePublicDashboard drives one request through a REAL router registered at
// the REAL template and hands back what the gate asked the module.
func servePublicDashboard(t *testing.T, template, target string) *recordingDashboardModule {
	t.Helper()

	module := &recordingDashboardModule{}
	prov := &provider{dashboardModule: module}

	var err error
	router := routing.Serve(http.MethodGet, template, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		_, _, err = prov.publicDashboardSelectors(req, nil)
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the read")
	require.NoError(t, err)
	return module
}

func TestPublicDashboardSelectorsReadTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module := servePublicDashboard(t, "/v1/o11y/public/dashboards/{id}", "/v1/o11y/public/dashboards/"+want.String())

	require.True(t, module.called)
	assert.Equal(t, want, module.gotID)
}

// The widget route carries a second segment, and the gate still names the
// dashboard by {id} — never by {idx}, which is a widget index.
func TestPublicDashboardSelectorsIgnoreTheWidgetSegment(t *testing.T) {
	want := valuer.GenerateUUID()

	module := servePublicDashboard(t, "/v1/o11y/public/dashboards/{id}/widgets/{idx}/query_range", "/v1/o11y/public/dashboards/"+want.String()+"/widgets/3/query_range")

	require.True(t, module.called)
	assert.Equal(t, want, module.gotID)
}

// A segment that is not a uuid is refused before the module is asked: the gate
// must never resolve selectors for the zero uuid.
func TestPublicDashboardSelectorsRefuseANonUUIDSegment(t *testing.T) {
	module := &recordingDashboardModule{}
	prov := &provider{dashboardModule: module}

	var err error
	router := routing.Serve(http.MethodGet, "/v1/o11y/public/dashboards/{id}", http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		_, _, err = prov.publicDashboardSelectors(req, nil)
	}))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/o11y/public/dashboards/not-a-uuid", http.NoBody))

	require.Error(t, err)
	assert.False(t, module.called, "a malformed id must not reach the module")
}
