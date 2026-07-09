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

// TestMountNormalizesExternalPath proves the one public contract /v1/o11y/<resource>
// (one /v1/, no nested version, no /api/) is normalized at the mount seam onto o11y's
// internal /api/ namespace — where a version-less alias (SigNoz, highest-version) or an
// llmobs route answers.
func TestMountNormalizesExternalPath(t *testing.T) {
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

	cases := []struct {
		name, external, internal string
	}{
		// Canonical version-less contract → internal /api/<resource> (no nested version).
		{"versionless health", "/v1/o11y/health", "/api/health"},
		{"versionless query_range", "/v1/o11y/query_range", "/api/query_range"},
		// Hanzo llmobs resources (own their version-less names).
		{"llmobs traces", "/v1/o11y/traces", "/api/traces"},
		{"llmobs observations", "/v1/o11y/observations", "/api/observations"},
		{"llmobs score by id", "/v1/o11y/score/abc123", "/api/score/abc123"},
		// Explicit-version form still resolves to its exact version route.
		{"explicit version", "/v1/o11y/v3/query_range", "/api/v3/query_range"},
		// Leaked /api/ form kept working for the not-yet-migrated SigNoz SPA.
		{"legacy api alias", "/v1/o11y/api/v1/health", "/api/v1/health"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sawPath = ""
			req := httptest.NewRequest(http.MethodGet, tc.external, nil)
			resp, err := app.Fiber().Test(req)
			if err != nil {
				t.Fatalf("Test: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d want 200", resp.StatusCode)
			}
			if sawPath != tc.internal {
				t.Fatalf("external %s → internal %q, want %q", tc.external, sawPath, tc.internal)
			}
		})
	}
}
