package o11y_test

import (
	"net/http"
	"strings"
	"testing"

	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	tf "github.com/hanzoai/o11y/pkg/types/tracefunneltypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// THE WIRE PROOF for the trace-funnels face. Same harness as every other slice:
// the bytes the RUNTIME writes go in, the op's answer comes out, and they must
// be the same bytes — plus the method and path the op asked the runtime for.

// fullFunnel is one funnel with every field populated, so a dropped or renamed
// field cannot hide behind a zero value.
func fullFunnel() tf.GettableFunnel {
	return tf.GettableFunnel{
		FunnelID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", FunnelName: "checkout",
		Description: "cart → pay → done", CreatedAt: 1_760_000_000_000, CreatedBy: "z",
		UpdatedAt: 1_760_000_009_000, UpdatedBy: "z", OrgID: "maxpower",
		UserEmail: "z@hanzo.ai",
		Steps: []*tf.FunnelStep{{
			ID:   valuer.MustNewUUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8"),
			Name: "cart", Description: "add to cart", Order: 1,
			ServiceName: "web", SpanName: "POST /cart",
			Filters:        &v3.FilterSet{Operator: "AND"},
			LatencyPointer: "start", LatencyType: "p99", HasErrors: false,
		}},
	}
}

func TestFunnelReadsAnswerTheRuntimeAnswer(t *testing.T) {
	one := fullFunnel()
	for _, tc := range []struct {
		name, method, path, target string
		payload                    any
		body                       string
	}{
		{"create", http.MethodPost, "/v1/o11y/trace-funnels/new", "/v1/o11y/trace-funnels/new",
			one, `{"funnel_name":"checkout","timestamp":1760000000000}`},
		{"list", http.MethodGet, "/v1/o11y/trace-funnels/list", "/v1/o11y/trace-funnels/list",
			[]tf.GettableFunnel{one}, ""},
		{"get", http.MethodGet, "/v1/o11y/trace-funnels/f-1", "/v1/o11y/trace-funnels/f-1",
			one, ""},
		{"update", http.MethodPut, "/v1/o11y/trace-funnels/f-1", "/v1/o11y/trace-funnels/f-1",
			one, `{"funnel_name":"checkout v2"}`},
		{"steps", http.MethodPut, "/v1/o11y/trace-funnels/steps/update", "/v1/o11y/trace-funnels/steps/update",
			one, `{"funnel_id":"f-1","steps":[]}`},
		{"delete", http.MethodDelete, "/v1/o11y/trace-funnels/f-1", "/v1/o11y/trace-funnels/f-1",
			nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			want := rendered(t, http.StatusOK, tc.payload)
			asked := logsRuntime(t, http.StatusOK, want)

			var body *strings.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			var req *http.Request
			if body == nil {
				req = member(tc.method, tc.path, nil)
			} else {
				req = member(tc.method, tc.path, body)
			}
			status, got := call(t, app, req)
			if status != http.StatusOK {
				t.Fatalf("status=%d body=%s", status, got)
			}
			if string(got) != string(want) {
				t.Fatalf("op answered %s, runtime wrote %s", got, want)
			}
			if r := *asked; r.Method != tc.method || r.URL.Path != tc.target {
				t.Fatalf("runtime asked %s %s, want %s %s", r.Method, r.URL.Path, tc.method, tc.target)
			}
		})
	}
}

// The twelve analytics reads all answer the same table, and each must reach its
// OWN path. The saved family carries the funnel id on the path; the ad-hoc
// family carries the funnel in the body and must NOT invent an id segment.
func TestFunnelAnalyticsReachTheirOwnPaths(t *testing.T) {
	rows := []v3.Row{{Data: map[string]any{"conversion_rate": 0.42, "total": float64(100)}}}
	views := []string{"validate", "overview", "steps", "steps/overview", "slow-traces", "error-traces"}

	for _, view := range views {
		for _, saved := range []bool{true, false} {
			path := "/v1/o11y/trace-funnels/analytics/" + view
			if saved {
				path = "/v1/o11y/trace-funnels/f-1/analytics/" + view
			}
			t.Run(path, func(t *testing.T) {
				app := mounted(t)
				want := enveloped(t, rows)
				asked := logsRuntime(t, http.StatusOK, want)

				status, got := call(t, app, member(http.MethodPost, path,
					strings.NewReader(`{"start_time":1,"end_time":2,"step_start":1,"step_end":2,"steps":[]}`)))
				if status != http.StatusOK {
					t.Fatalf("status=%d body=%s", status, got)
				}
				if string(got) != string(want) {
					t.Fatalf("op answered %s, runtime wrote %s", got, want)
				}
				if r := *asked; r.Method != http.MethodPost || r.URL.Path != path {
					t.Fatalf("runtime asked %s %s, want POST %s", r.Method, r.URL.Path, path)
				}
			})
		}
	}
	if len(views) != 6 {
		t.Fatalf("the census itself is wrong: %d", len(views))
	}
}

// A funnel id is a path segment the router matched, and it goes on VERBATIM.
func TestFunnelIDGoesOnVerbatim(t *testing.T) {
	app := mounted(t)
	asked := logsRuntime(t, http.StatusOK, rendered(t, http.StatusOK, fullFunnel()))

	call(t, app, member(http.MethodGet, "/v1/o11y/trace-funnels/a.b-c_d", nil))
	if got := (*asked).URL.Path; got != "/v1/o11y/trace-funnels/a.b-c_d" {
		t.Fatalf("runtime saw %q", got)
	}
}

// The literal routes must beat the :funnel_id parameter — /list is a listing,
// not a funnel named "list". The mux tree gets this from registration order;
// here it is asserted, because a Fiber tree that ever changed its precedence
// rule would otherwise fail silently at a customer.
func TestFunnelLiteralsBeatTheParameter(t *testing.T) {
	for _, path := range []string{
		"/v1/o11y/trace-funnels/list",
		"/v1/o11y/trace-funnels/analytics/overview",
	} {
		t.Run(path, func(t *testing.T) {
			app := mounted(t)
			asked := logsRuntime(t, http.StatusOK, enveloped(t, []v3.Row{}))
			method := http.MethodGet
			var body *strings.Reader
			if strings.Contains(path, "analytics") {
				method, body = http.MethodPost, strings.NewReader("{}")
			}
			if body == nil {
				call(t, app, member(method, path, nil))
			} else {
				call(t, app, member(method, path, body))
			}
			if *asked == nil {
				t.Fatal("the runtime was never asked — the literal lost to the parameter")
			}
			if got := (*asked).URL.Path; got != path {
				t.Fatalf("runtime saw %q, want %q", got, path)
			}
		})
	}
}

func TestTraceFunnelRoutesAreTheSameEighteen(t *testing.T) {
	want := map[string]bool{
		"POST /v1/o11y/trace-funnels/new":          true,
		"GET /v1/o11y/trace-funnels/list":          true,
		"PUT /v1/o11y/trace-funnels/steps/update":  true,
		"GET /v1/o11y/trace-funnels/:funnel_id":    true,
		"PUT /v1/o11y/trace-funnels/:funnel_id":    true,
		"DELETE /v1/o11y/trace-funnels/:funnel_id": true,
	}
	for _, view := range []string{"validate", "overview", "steps", "steps/overview", "slow-traces", "error-traces"} {
		want["POST /v1/o11y/trace-funnels/analytics/"+view] = true
		want["POST /v1/o11y/trace-funnels/:funnel_id/analytics/"+view] = true
	}
	if len(want) != 18 {
		t.Fatalf("the census itself is wrong: %d", len(want))
	}
	assertRoutes(t, want, "/v1/o11y/trace-funnels")
}
