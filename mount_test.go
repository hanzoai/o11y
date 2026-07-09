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
// (one /v1/, no /api/) is normalized at the mount seam onto the two internal route
// families: SigNoz native (/api/vN/*) and Hanzo llmobs (/v1/o11y/*, passed through).
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
		// SigNoz native — canonical Hanzo form; the /api/ never surfaces externally.
		{"signoz canonical v1", "/v1/o11y/v1/health", "/api/v1/health"},
		{"signoz canonical v3", "/v1/o11y/v3/query_range", "/api/v3/query_range"},
		{"signoz canonical v5", "/v1/o11y/v5/query_range", "/api/v5/query_range"},
		// SigNoz native — deprecated /api/ alias still resolves during migration.
		{"signoz legacy alias", "/v1/o11y/api/v1/health", "/api/v1/health"},
		// Hanzo llmobs — registered natively at /v1/o11y/*, passed through untouched.
		{"llmobs traces", "/v1/o11y/traces", "/v1/o11y/traces"},
		{"llmobs observations", "/v1/o11y/observations", "/v1/o11y/observations"},
		{"llmobs score by id", "/v1/o11y/score/abc123", "/v1/o11y/score/abc123"},
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
