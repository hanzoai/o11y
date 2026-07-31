package o11y_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/cloud"
	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

func TestMountWithoutHandlerReturns503(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app, cloud.Deps{}); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	o11y.SetHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/o11y/anything", nil)
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", resp.StatusCode)
	}
}

// TestMountDelegatesPathVerbatim proves the mount seam does not touch the path.
// Routes are registered at their full public /v1/o11y/<resource> names, so the
// runtime handler must receive exactly what the client sent — any rewrite here
// can only move a request off its own route. A mangler is invisible to the
// compiler, so this is the only thing that catches one coming back.
func TestMountDelegatesPathVerbatim(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app, cloud.Deps{}); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	var sawPath string
	o11y.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer o11y.SetHandler(nil)

	// Each is a real registered route literal (grep them: routes_*.go,
	// pkg/apiserver/o11yapiserver/*.go). What arrives is what dispatches.
	paths := []string{
		"/v1/o11y/services",
		"/v1/o11y/query_range",
		"/v1/o11y/settings/ttl",
		"/v1/o11y/global/config",
		"/v1/o11y/users",
		"/v1/o11y/llm/observations",
		"/v1/o11y/llm/score/abc123",
		"/v1/o11y/errortracking/issues",
		"/v1/o11y/api/hanzo/envelope/", // Sentry SDK wire path, received as-is
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			sawPath = ""
			resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, p, nil))
			if err != nil {
				t.Fatalf("Test: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d want 200", resp.StatusCode)
			}
			if sawPath != p {
				t.Fatalf("runtime received %q, want %q verbatim", sawPath, p)
			}
		})
	}
}
