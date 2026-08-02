package implsavedview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/savedview"
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The three per-view routes address a view by the {viewId} PATH segment, so what
// has to hold is that the id the ROUTER matched is the id the module acts on.
// These drive the REAL handler through a REAL router registered at the REAL
// template — asserting a reader against its own injector would prove only that
// the two halves of coretypes agree with each other.
type recordingModule struct {
	savedview.Module // only the three methods these routes reach are implemented

	gotID  valuer.UUID
	called bool
}

func (module *recordingModule) GetView(_ context.Context, _ string, id valuer.UUID) (*v3.SavedView, error) {
	module.gotID, module.called = id, true
	return &v3.SavedView{ID: id, Name: "service latency"}, nil
}

func (module *recordingModule) UpdateView(_ context.Context, _ string, id valuer.UUID, _ v3.SavedView) error {
	module.gotID, module.called = id, true
	return nil
}

func (module *recordingModule) DeleteView(_ context.Context, _ string, id valuer.UUID) error {
	module.gotID, module.called = id, true
	return nil
}

const viewRoute = "/v1/o11y/explorer/views/{viewId}"

func serve(t *testing.T, method, target, body string, pick func(savedview.Handler) http.HandlerFunc) (*recordingModule, *httptest.ResponseRecorder) {
	t.Helper()

	module := &recordingModule{}
	router := routing.Serve(method, viewRoute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pick(NewHandler(module))(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{
			UserID:    valuer.GenerateUUID().String(),
			OrgID:     valuer.GenerateUUID().String(),
			Principal: authtypes.PrincipalUser,
		})))
	}))

	req := httptest.NewRequest(method, target, strings.NewReader(body))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return module, rec
}

func TestGetViewActsOnTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := serve(t, http.MethodGet, "/v1/o11y/explorer/views/"+want.String(), "", func(handler savedview.Handler) http.HandlerFunc { return handler.Get })

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID)
	assert.Contains(t, rec.Body.String(), want.String())
	assert.Contains(t, rec.Body.String(), "service latency")
}

func TestUpdateViewActsOnTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()
	body := `{"name":"service latency","compositeQuery":{"queryType":"promql","panelType":"graph","promQueries":{"A":{"query":"up"}}}}`

	module, rec := serve(t, http.MethodPut, "/v1/o11y/explorer/views/"+want.String(), body, func(handler savedview.Handler) http.HandlerFunc { return handler.Update })

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID)
}

func TestDeleteViewActsOnTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := serve(t, http.MethodDelete, "/v1/o11y/explorer/views/"+want.String(), "", func(handler savedview.Handler) http.HandlerFunc { return handler.Delete })

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID)
}

// A segment that is not a uuid is refused before the module is asked — an empty
// or malformed id must never arrive as the zero uuid and delete whatever that
// matches.
func TestViewRouteRefusesANonUUIDSegment(t *testing.T) {
	module, rec := serve(t, http.MethodDelete, "/v1/o11y/explorer/views/not-a-uuid", "", func(handler savedview.Handler) http.HandlerFunc { return handler.Delete })

	assert.False(t, module.called, "a malformed id must not reach the module")
	assert.Contains(t, rec.Body.String(), "failed to parse view id")
}
