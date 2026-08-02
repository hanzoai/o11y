package impldashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/dashboardtypes"
	"github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Every per-dashboard route addresses the dashboard by a PATH segment, and the
// public widget route carries a second one ({idx}), so what has to hold is that
// the segments the ROUTER matched are the values the module acts on. These drive
// the REAL handlers through a REAL router registered at the REAL templates —
// asserting a reader against its own injector would prove only that the two
// halves of coretypes agree with each other, not that a segment survives the
// trip from URL to module call.
type recordingModule struct {
	dashboard.Module // only the methods these routes reach are implemented

	gotID        valuer.UUID
	gotWidgetIdx uint64
	called       bool
}

func (module *recordingModule) Get(_ context.Context, _ valuer.UUID, id valuer.UUID) (*dashboardtypes.Dashboard, error) {
	return &dashboardtypes.Dashboard{ID: id.String()}, nil
}

func (module *recordingModule) DeletePublic(_ context.Context, _ valuer.UUID, id valuer.UUID) error {
	module.gotID, module.called = id, true
	return nil
}

func (module *recordingModule) DeleteV2(_ context.Context, _ valuer.UUID, id valuer.UUID) error {
	module.gotID, module.called = id, true
	return nil
}

func (module *recordingModule) DeleteView(_ context.Context, _ valuer.UUID, id valuer.UUID) error {
	module.gotID, module.called = id, true
	return nil
}

func (module *recordingModule) GetDashboardByPublicID(_ context.Context, id valuer.UUID) (*dashboardtypes.Dashboard, error) {
	module.gotID = id
	return &dashboardtypes.Dashboard{ID: valuer.GenerateUUID().String(), OrgID: valuer.GenerateUUID()}, nil
}

func (module *recordingModule) GetPublic(_ context.Context, _ valuer.UUID, _ valuer.UUID) (*dashboardtypes.PublicDashboard, error) {
	return &dashboardtypes.PublicDashboard{TimeRangeEnabled: false, DefaultTimeRange: "1h"}, nil
}

func (module *recordingModule) GetPublicWidgetQueryRange(_ context.Context, _ valuer.UUID, widgetIdx uint64, _ uint64, _ uint64) (*querybuildertypesv5.QueryRangeResponse, error) {
	module.gotWidgetIdx, module.called = widgetIdx, true
	return &querybuildertypesv5.QueryRangeResponse{}, nil
}

// serve registers pick's handler at its REAL template, drives one request
// through the router, and answers with what reached the module.
func serve(t *testing.T, template, method, target string, pick func(dashboard.Handler) http.HandlerFunc) (*recordingModule, *httptest.ResponseRecorder) {
	t.Helper()

	module := &recordingModule{}
	router := mux.NewRouter()
	router.HandleFunc(template, pick(NewHandler(module, factory.ProviderSettings{}, nil))).Methods(method)

	req := httptest.NewRequest(method, target, http.NoBody)
	req = req.WithContext(authtypes.NewContextWithClaims(req.Context(), authtypes.Claims{
		UserID:    valuer.GenerateUUID().String(),
		OrgID:     valuer.GenerateUUID().String(),
		Principal: authtypes.PrincipalUser,
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return module, rec
}

func TestDeletePublicActsOnTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := serve(t, "/v1/o11y/dashboards/{id}/public", http.MethodDelete, "/v1/o11y/dashboards/"+want.String()+"/public", func(handler dashboard.Handler) http.HandlerFunc { return handler.DeletePublic })

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID)
}

func TestDeleteV2ActsOnTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := serve(t, "/v1/o11y/dashboards/{id}", http.MethodDelete, "/v1/o11y/dashboards/"+want.String(), func(handler dashboard.Handler) http.HandlerFunc { return handler.DeleteV2 })

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID)
}

func TestDeleteViewActsOnTheIDTheRouterMatched(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := serve(t, "/v1/o11y/dashboard_views/{id}", http.MethodDelete, "/v1/o11y/dashboard_views/"+want.String(), func(handler dashboard.Handler) http.HandlerFunc { return handler.DeleteView })

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID)
}

// The public widget route carries TWO path segments, and they are different
// values: {id} names the dashboard, {idx} names the widget within it. A reader
// that answered for the wrong one would still return a plausible string.
func TestPublicWidgetQueryRangeReadsBothPathSegments(t *testing.T) {
	want := valuer.GenerateUUID()

	module, rec := serve(t, "/v1/o11y/public/dashboards/{id}/widgets/{idx}/query_range", http.MethodGet, "/v1/o11y/public/dashboards/"+want.String()+"/widgets/3/query_range", func(handler dashboard.Handler) http.HandlerFunc {
		return handler.GetPublicWidgetQueryRange
	})

	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, want, module.gotID, "{id} names the dashboard")
	assert.Equal(t, uint64(3), module.gotWidgetIdx, "{idx} names the widget")
}

// A widget index that is not a number is refused with the public dashboard's own
// code before the module is asked — it must never arrive as widget zero.
func TestPublicWidgetQueryRangeRefusesANonNumericIndex(t *testing.T) {
	module, rec := serve(t, "/v1/o11y/public/dashboards/{id}/widgets/{idx}/query_range", http.MethodGet, "/v1/o11y/public/dashboards/"+valuer.GenerateUUID().String()+"/widgets/not-a-number/query_range", func(handler dashboard.Handler) http.HandlerFunc {
		return handler.GetPublicWidgetQueryRange
	})

	assert.False(t, module.called, "a malformed widget index must not reach the module")
	assert.Contains(t, rec.Body.String(), "invalid widget index")
}
