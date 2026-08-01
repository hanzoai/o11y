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
		// REMOVED as the conversion advanced — each of these became a TYPED op and
		// therefore dispatches off the mux tree AHEAD of the wildcard, which is the
		// whole point of the migration: /query_range (metrics.go), /settings/ttl and
		// /global/config (platform.go). A typed op is proved by its own slice test;
		// asserting it still delegates verbatim would assert the migration failed.
		// What remains here are the genuinely-wildcarded surfaces — the escape
		// hatches — which is exactly what this test should guard.
		"/v1/o11y/query_progress", // long-poll progress: deliberately wildcarded, a stream (querycore.go)
		"/v1/o11y/complete/google", // sign-in callback: 303 redirect, deliberately wildcarded (identity.go)
		// the /v1/o11y/llm* surface is TYPED now (llmobs.go), so it dispatches to
		// ops and takes precedence over this wildcard — proved in llmobs_test.go.
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
