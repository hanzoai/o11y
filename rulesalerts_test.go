package o11y

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// TestRulesAlertsOpsRelayVerbatim pins the rules & alerting typed face: every op
// dispatches ahead of the wildcard, rebuilds the SAME method and path (path
// params and query parameters included) the runtime has always seen, and
// answers with the status its mux registration declared. The runtime is stubbed
// so the test observes exactly what the relay forwards; the wire, not the work,
// is what these ops own.
func TestRulesAlertsOpsRelayVerbatim(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	mountRulesAlerts(app)

	var (
		sawMethod string
		sawPath   string
		sawBody   []byte
	)
	SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod, sawPath = r.Method, r.URL.Path
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer SetHandler(nil)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantMethod string
		wantPath   string
		wantStatus int
	}{
		{"list rules", http.MethodGet, "/v1/o11y/rules", "", http.MethodGet, "/v1/o11y/rules", http.StatusOK},
		{"get rule", http.MethodGet, "/v1/o11y/rules/abc", "", http.MethodGet, "/v1/o11y/rules/abc", http.StatusOK},
		{"create rule", http.MethodPost, "/v1/o11y/rules", `{"version":"v5","condition":{}}`, http.MethodPost, "/v1/o11y/rules", http.StatusCreated},
		{"update rule", http.MethodPut, "/v1/o11y/rules/abc", `{"version":"v5","condition":{}}`, http.MethodPut, "/v1/o11y/rules/abc", http.StatusNoContent},
		{"delete rule", http.MethodDelete, "/v1/o11y/rules/abc", "", http.MethodDelete, "/v1/o11y/rules/abc", http.StatusNoContent},
		{"patch rule", http.MethodPatch, "/v1/o11y/rules/abc", `{"version":"v5","condition":{}}`, http.MethodPatch, "/v1/o11y/rules/abc", http.StatusOK},
		{"legacy test rule", http.MethodPost, "/v1/o11y/testRule", `{"version":"v5","condition":{}}`, http.MethodPost, "/v1/o11y/testRule", http.StatusOK},
		{"create channel", http.MethodPost, "/v1/o11y/channels", `{"name":"c"}`, http.MethodPost, "/v1/o11y/channels", http.StatusCreated},
		{"update channel", http.MethodPut, "/v1/o11y/channels/xy", `{"name":"r"}`, http.MethodPut, "/v1/o11y/channels/xy", http.StatusNoContent},
		{"test channel", http.MethodPost, "/v1/o11y/channels/test", `{"name":"r"}`, http.MethodPost, "/v1/o11y/channels/test", http.StatusNoContent},
		{"delete route policy", http.MethodDelete, "/v1/o11y/route_policies/rp", "", http.MethodDelete, "/v1/o11y/route_policies/rp", http.StatusNoContent},
		{"get alerts", http.MethodGet, "/v1/o11y/alerts", "", http.MethodGet, "/v1/o11y/alerts", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sawMethod, sawPath, sawBody = "", "", nil
			var reqBody io.Reader = http.NoBody
			if tc.body != "" {
				reqBody = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, reqBody)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Test: %v", err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status=%d want %d", resp.StatusCode, tc.wantStatus)
			}
			if sawMethod != tc.wantMethod || sawPath != tc.wantPath {
				t.Fatalf("runtime saw %s %q, want %s %q", sawMethod, sawPath, tc.wantMethod, tc.wantPath)
			}
			if tc.body != "" && len(sawBody) == 0 {
				t.Fatalf("runtime saw empty body for %s %s, want the posted payload", tc.method, tc.path)
			}
		})
	}
}

// TestRulesAlertsHistoryQueryForwarded proves a v2 history GET carries its path
// id and its window query parameters through to the runtime unchanged, and that
// a downtime-list filter is forwarded only when set.
func TestRulesAlertsHistoryQueryForwarded(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	mountRulesAlerts(app)

	var sawPath string
	var sawQuery url.Values
	SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath, sawQuery = r.URL.Path, r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer SetHandler(nil)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/o11y/rules/r1/history/stats?start=10&end=20", http.NoBody))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	if sawPath != "/v1/o11y/rules/r1/history/stats" {
		t.Fatalf("path=%q want /v1/o11y/rules/r1/history/stats", sawPath)
	}
	if sawQuery.Get("start") != "10" || sawQuery.Get("end") != "20" {
		t.Fatalf("query=%v want start=10 end=20", sawQuery)
	}

	// An unset downtime filter must not be forwarded (tri-state preserved).
	sawQuery = nil
	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/o11y/downtime_schedules", http.NoBody)); err != nil {
		t.Fatalf("Test: %v", err)
	}
	if sawQuery.Has("active") || sawQuery.Has("recurring") {
		t.Fatalf("unset filters forwarded: %v", sawQuery)
	}
}
