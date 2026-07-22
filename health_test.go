package o11y_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/cloud"
	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/zap-proto/zip"
)

// fakeHealth stands in for the runtime's factory.Handler. It renders through the
// SAME render + factory types the real handler uses, so the native route's
// net/http bridge is exercised faithfully.
type fakeHealth struct{ healthy bool }

func (f fakeHealth) Healthz(w http.ResponseWriter, r *http.Request) {
	code := http.StatusOK
	if !f.healthy {
		code = http.StatusServiceUnavailable
	}
	render.Success(w, code, factory.Response{Healthy: f.healthy})
}

func (f fakeHealth) Readyz(w http.ResponseWriter, r *http.Request) { f.Healthz(w, r) }

func (f fakeHealth) Livez(w http.ResponseWriter, r *http.Request) {
	render.Success(w, http.StatusOK, nil)
}

func newMounted(t *testing.T) *zip.App {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app, cloud.Deps{}); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	return app
}

func get(t *testing.T, app *zip.App, path string) (*http.Response, string) {
	t.Helper()
	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("Test %s: %v", path, err)
	}
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// TestHealthzDispatchesNatively proves the healthz probe is served by the native
// router (the runtime handler is reached over the bridge), returning the runtime
// status and body — no mux involved.
func TestHealthzDispatchesNatively(t *testing.T) {
	app := newMounted(t)
	o11y.SetHealth(fakeHealth{healthy: true})
	defer o11y.SetHealth(nil)

	resp, body := get(t, app, "/v1/o11y/api/v2/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	if !strings.Contains(body, `"status":"success"`) || !strings.Contains(body, `"healthy":true`) {
		t.Fatalf("body=%q missing success/healthy", body)
	}
}

// TestHealthzUnhealthyReturns503 proves the native route preserves the runtime's
// status code (unhealthy → 503).
func TestHealthzUnhealthyReturns503(t *testing.T) {
	app := newMounted(t)
	o11y.SetHealth(fakeHealth{healthy: false})
	defer o11y.SetHealth(nil)

	resp, _ := get(t, app, "/v1/o11y/api/v2/healthz")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", resp.StatusCode)
	}
}

// TestReadyzDispatchesNatively covers the readiness probe on the native path.
func TestReadyzDispatchesNatively(t *testing.T) {
	app := newMounted(t)
	o11y.SetHealth(fakeHealth{healthy: true})
	defer o11y.SetHealth(nil)

	resp, _ := get(t, app, "/v1/o11y/api/v2/readyz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
}

// TestLivezRendersNatively proves livez is rendered natively (c.JSON over the
// shared render types) with the exact empty-success envelope.
func TestLivezRendersNatively(t *testing.T) {
	app := newMounted(t)
	o11y.SetHealth(fakeHealth{healthy: true})
	defer o11y.SetHealth(nil)

	resp, body := get(t, app, "/v1/o11y/api/v2/livez")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	if strings.TrimSpace(body) != `{"status":"success"}` {
		t.Fatalf("body=%q want empty-success envelope", body)
	}
}

// TestHealthFallsThroughWhenUnset proves the group degrades to the delegated
// wildcard when no runtime handler is registered: the probe request reaches the
// mux-tree handler installed via SetHandler, so behavior is unchanged until the
// native path is wired.
func TestHealthFallsThroughWhenUnset(t *testing.T) {
	app := newMounted(t)
	o11y.SetHealth(nil)

	var reached string
	o11y.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer o11y.SetHandler(nil)

	resp, _ := get(t, app, "/v1/o11y/api/v2/livez")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	// The wildcard rewrites /v1/o11y/api/v2/livez → /api/v2/livez before delegating.
	if reached != "/api/v2/livez" {
		t.Fatalf("fell through to %q, want /api/v2/livez", reached)
	}
}

// TestNativeParamAndMethod proves the native router extracts path params
// (c.Param — the mux.Vars replacement the staged migration depends on) and
// matches on method, using synthetic routes so the mechanism is validated
// independent of any runtime.
func TestNativeParamAndMethod(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Get("/thing/:id", func(c *zip.Ctx) error { return c.String(http.StatusOK, "get:"+c.Param("id")) })
	app.Post("/thing/:id", func(c *zip.Ctx) error { return c.String(http.StatusCreated, "post:"+c.Param("id")) })

	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/thing/abc", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "get:abc" {
		t.Fatalf("GET => %d %q, want 200 get:abc", resp.StatusCode, string(b))
	}

	resp, err = app.Fiber().Test(httptest.NewRequest(http.MethodPost, "/thing/xyz", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated || string(b) != "post:xyz" {
		t.Fatalf("POST => %d %q, want 201 post:xyz", resp.StatusCode, string(b))
	}
}
