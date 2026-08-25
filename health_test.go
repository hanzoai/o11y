package o11y_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	return app
}

func get(t *testing.T, app *zip.App, path string) (*http.Response, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("Test %s: %v", path, err)
	}
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// THE PROBE GROUP'S WHOLE WIRE, in each of the four states a host can leave it in.
//
// The probes have two sources — the health handler a host installs and the
// runtime's own answer at the same address — and the three routes pick between
// them per request. That makes the exact answer a function of the state, so the
// state is enumerated and every answer is written down: status, content type,
// the body, and whether X-Content-Type-Options rides along.
//
// The last column is not decoration. It is what says which side of the bridge an
// answer came from: net/http's http.Error writes text/plain and nosniff, and a
// zip answer carries neither unless a handler sets it. The refusal below is
// net/http's, verbatim — and the two content types on the same JSON envelope
// (application/json from render.Success, application/json; charset=utf-8 from
// c.JSON) are the same fact from the other direction.
type probeAnswer struct {
	path    string
	status  int
	ctype   string
	nosniff bool
	body    string
}

const (
	livezPath    = "/v1/o11y/livez"
	healthzPath  = "/v1/o11y/healthz"
	readyzPath   = "/v1/o11y/readyz"
	livetailPath = "/v1/o11y/logs/livetail" // a hatch, reached through the same bridge

	refused     = "o11y runtime not installed\n"
	refusedType = "text/plain; charset=utf-8"
	success     = `{"status":"success"}`
)

// fromRuntime is what the runtime installed below answers with, which names the
// path it was handed — the delegation is verbatim or this string is wrong.
func fromRuntime(path string) probeAnswer {
	return probeAnswer{path, 200, "application/json", false, `{"from":"runtime","path":"` + path + `"}`}
}

func refusal(path string) probeAnswer {
	return probeAnswer{path, http.StatusServiceUnavailable, refusedType, true, refused}
}

func TestProbesAnswerVerbatim(t *testing.T) {
	healthy := `{"status":"success","data":{"healthy":true,"services":null}}`
	unhealthy := `{"status":"success","data":{"healthy":false,"services":null}}`

	for _, world := range []struct {
		name    string
		enter   func()
		answers []probeAnswer
	}{
		{
			// A host installed a health handler. livez is rendered here, healthz
			// and readyz are the handler's own bytes.
			name:  "health installed",
			enter: func() { o11y.SetHealth(fakeHealth{healthy: true}); o11y.SetRuntime(nil) },
			answers: []probeAnswer{
				{livezPath, 200, "application/json; charset=utf-8", false, success},
				{healthzPath, 200, "application/json", false, healthy},
				{readyzPath, 200, "application/json", false, healthy},
				refusal(livetailPath),
			},
		},
		{
			// The handler's status rides through unchanged: unhealthy is a 503
			// carrying a success envelope, which is the shape it has always had.
			name:  "health installed, unhealthy",
			enter: func() { o11y.SetHealth(fakeHealth{healthy: false}); o11y.SetRuntime(nil) },
			answers: []probeAnswer{
				{livezPath, 200, "application/json; charset=utf-8", false, success},
				{healthzPath, 503, "application/json", false, unhealthy},
				{readyzPath, 503, "application/json", false, unhealthy},
				refusal(livetailPath),
			},
		},
		{
			// No health handler: every probe is the runtime's own answer at that
			// probe's address, which is what a hatch is.
			name: "runtime only",
			enter: func() {
				o11y.SetHealth(nil)
				o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"from":"runtime","path":"`+r.URL.Path+`"}`)
				})))
			},
			answers: []probeAnswer{
				fromRuntime(livezPath), fromRuntime(healthzPath),
				fromRuntime(readyzPath), fromRuntime(livetailPath),
			},
		},
		{
			// Neither: 503, and the refusal names which of the two reasons it is.
			name:  "nothing installed",
			enter: func() { o11y.SetHealth(nil); o11y.SetRuntime(nil) },
			answers: []probeAnswer{
				refusal(livezPath), refusal(healthzPath), refusal(readyzPath), refusal(livetailPath),
			},
		},
	} {
		t.Run(world.name, func(t *testing.T) {
			world.enter()
			defer func() { o11y.SetHealth(nil); o11y.SetRuntime(nil) }()

			app := newMounted(t)
			for _, want := range world.answers {
				resp, body := get(t, app, want.path)
				if resp.StatusCode != want.status {
					t.Errorf("GET %s status=%d, want %d", want.path, resp.StatusCode, want.status)
				}
				if ct := resp.Header.Get("Content-Type"); ct != want.ctype {
					t.Errorf("GET %s content-type=%q, want %q", want.path, ct, want.ctype)
				}
				if got := resp.Header.Get("X-Content-Type-Options") == "nosniff"; got != want.nosniff {
					t.Errorf("GET %s nosniff=%v, want %v", want.path, got, want.nosniff)
				}
				if body != want.body {
					t.Errorf("GET %s body=%q, want %q", want.path, body, want.body)
				}
			}
		})
	}
}

// Each probe was also asserted one at a time — healthz 200 and 503, readyz 200,
// livez's envelope, and the fall-through reaching the runtime at its own path.
// Every one of those is a row of the table above, stated there with the content
// type and the headers the single assertions never read, so they are not
// repeated here: two proofs of one fact drift, and the weaker one is the one
// that goes stale unnoticed.

// TestNativeParamAndMethod proves the native router extracts path params
// (c.Param — the mux.Vars replacement the staged migration depends on) and
// matches on method, using synthetic routes so the mechanism is validated
// independent of any runtime.
func TestNativeParamAndMethod(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Get("/thing/:id", func(c *zip.Ctx) error { return c.String(http.StatusOK, "get:"+c.Param("id")) })
	app.Post("/thing/:id", func(c *zip.Ctx) error { return c.String(http.StatusCreated, "post:"+c.Param("id")) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/thing/abc", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "get:abc" {
		t.Fatalf("GET => %d %q, want 200 get:abc", resp.StatusCode, string(b))
	}

	resp, err = app.Test(httptest.NewRequest(http.MethodPost, "/thing/xyz", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	b, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated || string(b) != "post:xyz" {
		t.Fatalf("POST => %d %q, want 201 post:xyz", resp.StatusCode, string(b))
	}
}
