package o11y_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// THE WIRE PROOF for the span-mapper face. Same harness as every other slice.
// The four writes answer 204 with no body, so those cases assert the STATUS the
// op declared as well as the path it reached — a declared status that does not
// match the wire is the one lie a generated client cannot recover from.

func fullGroup() *spantypes.SpanMapperGroup {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	return &spantypes.SpanMapperGroup{
		ID:    valuer.MustNewUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		OrgID: valuer.MustNewUUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8"),
		Name:  "promote-k8s",
		Condition: spantypes.SpanMapperGroupCondition{
			Attributes: []string{"k8s.pod.name"},
			Resource:   []string{"service.name"},
		},
		Enabled:       true,
		TimeAuditable: types.TimeAuditable{CreatedAt: at, UpdatedAt: at},
		UserAuditable: types.UserAuditable{CreatedBy: "z", UpdatedBy: "z"},
	}
}

func fullMapper() *spantypes.SpanMapper {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	return &spantypes.SpanMapper{
		ID:            valuer.MustNewUUID("6ba7b812-9dad-11d1-80b4-00c04fd430c8"),
		GroupID:       valuer.MustNewUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		Name:          "pod-name",
		FieldContext:  spantypes.FieldContextSpanAttribute,
		Enabled:       true,
		TimeAuditable: types.TimeAuditable{CreatedAt: at, UpdatedAt: at},
		UserAuditable: types.UserAuditable{CreatedBy: "z", UpdatedBy: "z"},
	}
}

func TestSpanMapperReadsAnswerTheRuntimeAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		payload    any
	}{
		{"groups", "/v1/o11y/span_mapper_groups",
			spantypes.NewGettableSpanMapperGroups([]*spantypes.SpanMapperGroup{fullGroup()})},
		{"mappers", "/v1/o11y/span_mapper_groups/g-1/span_mappers",
			spantypes.NewGettableSpanMappers([]*spantypes.SpanMapper{fullMapper()})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			want := rendered(t, http.StatusOK, tc.payload)
			asked := logsRuntime(t, http.StatusOK, want)

			status, got := call(t, app, member(http.MethodGet, tc.path, nil))
			if status != http.StatusOK {
				t.Fatalf("status=%d body=%s", status, got)
			}
			if string(got) != string(want) {
				t.Fatalf("op answered %s, runtime wrote %s", got, want)
			}
			if r := *asked; r.Method != http.MethodGet || r.URL.Path != tc.path {
				t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
			}
		})
	}
}

// The enabled filter reaches the runtime as a query parameter, and an UNSET
// filter stays absent rather than arriving as "false" — the runtime has always
// read a missing parameter as "all groups", so sending false would narrow a
// listing the caller did not narrow.
func TestSpanMapperGroupFilterStaysAbsentWhenUnset(t *testing.T) {
	payload := spantypes.NewGettableSpanMapperGroups([]*spantypes.SpanMapperGroup{fullGroup()})

	for _, tc := range []struct{ target, want string }{
		{"/v1/o11y/span_mapper_groups", ""},
		{"/v1/o11y/span_mapper_groups?enabled=true", "enabled=true"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			app := mounted(t)
			asked := logsRuntime(t, http.StatusOK, rendered(t, http.StatusOK, payload))
			call(t, app, member(http.MethodGet, tc.target, nil))
			if got := (*asked).URL.RawQuery; got != tc.want {
				t.Fatalf("runtime saw query %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpanMapperCreatesAnswer201(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		payload    any
		body       string
	}{
		{"group", "/v1/o11y/span_mapper_groups", fullGroup(),
			`{"name":"promote-k8s","condition":{"attributes":["k8s.pod.name"],"resource":["service.name"]},"enabled":true}`},
		{"mapper", "/v1/o11y/span_mapper_groups/g-1/span_mappers", fullMapper(),
			`{"name":"pod-name","fieldContext":"span","config":{},"enabled":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			want := rendered(t, http.StatusCreated, tc.payload)
			asked := logsRuntime(t, http.StatusCreated, want)

			status, got := call(t, app, member(http.MethodPost, tc.path, strings.NewReader(tc.body)))
			if status != http.StatusCreated {
				t.Fatalf("status=%d want 201, body=%s", status, got)
			}
			if string(got) != string(want) {
				t.Fatalf("op answered %s, runtime wrote %s", got, want)
			}
			if r := *asked; r.Method != http.MethodPost || r.URL.Path != tc.path {
				t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
			}
		})
	}
}

// The four writes answer 204. Their ops declare it, and each must reach its own
// path with the two ids on VERBATIM.
func TestSpanMapperWritesAnswer204(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPatch, "/v1/o11y/span_mapper_groups/g.1", `{"enabled":false}`},
		{http.MethodDelete, "/v1/o11y/span_mapper_groups/g.1", ""},
		{http.MethodPatch, "/v1/o11y/span_mapper_groups/g.1/span_mappers/m.1", `{"enabled":false}`},
		{http.MethodDelete, "/v1/o11y/span_mapper_groups/g.1/span_mappers/m.1", ""},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			app := mounted(t)
			asked := logsRuntime(t, http.StatusNoContent, nil)

			var req *http.Request
			if tc.body == "" {
				req = member(tc.method, tc.path, nil)
			} else {
				req = member(tc.method, tc.path, strings.NewReader(tc.body))
			}
			status, got := call(t, app, req)
			if status != http.StatusNoContent {
				t.Fatalf("status=%d want 204, body=%s", status, got)
			}
			if r := *asked; r.Method != tc.method || r.URL.Path != tc.path {
				t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
			}
		})
	}
}

func TestSpanMapperRoutesAreTheSameEight(t *testing.T) {
	want := map[string]bool{
		"GET /v1/o11y/span_mapper_groups":                                    true,
		"POST /v1/o11y/span_mapper_groups":                                   true,
		"PATCH /v1/o11y/span_mapper_groups/:groupId":                         true,
		"DELETE /v1/o11y/span_mapper_groups/:groupId":                        true,
		"GET /v1/o11y/span_mapper_groups/:groupId/span_mappers":              true,
		"POST /v1/o11y/span_mapper_groups/:groupId/span_mappers":             true,
		"PATCH /v1/o11y/span_mapper_groups/:groupId/span_mappers/:mapperId":  true,
		"DELETE /v1/o11y/span_mapper_groups/:groupId/span_mappers/:mapperId": true,
	}
	if len(want) != 8 {
		t.Fatalf("the census itself is wrong: %d", len(want))
	}
	assertRoutes(t, want, "/v1/o11y/span_mapper_groups")
}
