package implpreference

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/preference"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/preferencetypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The four per-preference routes name the preference by a PATH segment, so what
// has to hold is that the name the ROUTER matched is the name the module is
// asked for. These drive the REAL handler through a REAL router registered at
// the REAL template — asserting a reader against its own injector would prove
// only that the two halves of coretypes agree with each other, not that the
// segment survives the trip from URL to module call.
type recordingModule struct {
	preference.Module // only the four methods these routes reach are implemented

	gotName preferencetypes.Name
	called  bool
}

func (module *recordingModule) GetByOrg(_ context.Context, _ valuer.UUID, name preferencetypes.Name) (*preferencetypes.Preference, error) {
	module.gotName, module.called = name, true
	return &preferencetypes.Preference{Name: name}, nil
}

func (module *recordingModule) GetByUser(_ context.Context, _ valuer.UUID, name preferencetypes.Name) (*preferencetypes.Preference, error) {
	module.gotName, module.called = name, true
	return &preferencetypes.Preference{Name: name}, nil
}

func (module *recordingModule) UpdateByOrg(_ context.Context, _ valuer.UUID, name preferencetypes.Name, _ any) error {
	module.gotName, module.called = name, true
	return nil
}

func (module *recordingModule) UpdateByUser(_ context.Context, _ valuer.UUID, name preferencetypes.Name, _ any) error {
	module.gotName, module.called = name, true
	return nil
}

// serve registers pick's handler at its REAL template, drives one request
// through the router, and answers with what reached the module.
func serve(t *testing.T, template, method, target, body string, pick func(preference.Handler) http.HandlerFunc) (*recordingModule, *httptest.ResponseRecorder) {
	t.Helper()

	module := &recordingModule{}
	router := routing.Serve(method, template, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestOrgPreferenceRoutesReadTheNameTheRouterMatched(t *testing.T) {
	module, rec := serve(t, "/v1/o11y/org/preferences/{name}", http.MethodGet, "/v1/o11y/org/preferences/sidenav_pinned", "", func(handler preference.Handler) http.HandlerFunc { return handler.GetByOrg })
	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, preferencetypes.NameSidenavPinned, module.gotName)
	assert.Contains(t, rec.Body.String(), "sidenav_pinned")

	module, rec = serve(t, "/v1/o11y/org/preferences/{name}", http.MethodPut, "/v1/o11y/org/preferences/nav_shortcuts", `{"value":true}`, func(handler preference.Handler) http.HandlerFunc { return handler.UpdateByOrg })
	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, preferencetypes.NameNavShortcuts, module.gotName)
	assert.JSONEq(t, `{"status":"success"}`, rec.Body.String())
}

func TestUserPreferenceRoutesReadTheNameTheRouterMatched(t *testing.T) {
	module, rec := serve(t, "/v1/o11y/user/preferences/{name}", http.MethodGet, "/v1/o11y/user/preferences/nav_shortcuts", "", func(handler preference.Handler) http.HandlerFunc { return handler.GetByUser })
	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, preferencetypes.NameNavShortcuts, module.gotName)
	assert.Contains(t, rec.Body.String(), "nav_shortcuts")

	module, rec = serve(t, "/v1/o11y/user/preferences/{name}", http.MethodPut, "/v1/o11y/user/preferences/sidenav_pinned", `{"value":true}`, func(handler preference.Handler) http.HandlerFunc { return handler.UpdateByUser })
	require.True(t, module.called, "the module was never reached: %s", rec.Body.String())
	assert.Equal(t, preferencetypes.NameSidenavPinned, module.gotName)
	assert.JSONEq(t, `{"status":"success"}`, rec.Body.String())
}

// A segment the route matches but the type refuses is answered with the type's
// own error, and the module is never asked — an unknown name must not arrive as
// the zero Name and read whatever that matches.
func TestPreferenceRouteRefusesANameTheTypeDoesNotKnow(t *testing.T) {
	module, rec := serve(t, "/v1/o11y/org/preferences/{name}", http.MethodGet, "/v1/o11y/org/preferences/not_a_preference", "", func(handler preference.Handler) http.HandlerFunc { return handler.GetByOrg })

	assert.False(t, module.called, "an unknown name must not reach the module")
	assert.Contains(t, rec.Body.String(), "invalid name: not_a_preference")
}
