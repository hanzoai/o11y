package impllmobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// validOrgUUID is any well-formed UUID for the claims (authz consistency); the span
// views scope on the SLUG, not this.
const validOrgUUID = "0196f794-ff30-7bee-a5f4-ef5ad315715e"

func claimsCtx() context.Context {
	return authtypes.NewContextWithClaims(context.Background(), authtypes.Claims{
		OrgID: validOrgUUID, UserID: "user-1", Email: "u@acme", Principal: authtypes.PrincipalUser,
	})
}

// TestViewRequest_TenantScopeFromHeader proves the span-view tenant boundary is set
// SERVER-SIDE from the validated X-Org-Id and cannot be spoofed by a client query
// param.
func TestViewRequest_TenantScopeFromHeader(t *testing.T) {
	// A client tries to inject the tenant via ?orgSlug=evil; the header is the real
	// tenant. ViewQuery.OrgSlug has no `query` tag, so the param is ignored.
	r := httptest.NewRequest(http.MethodGet, "/api/observations?orgSlug=evil&traceId=t1", nil)
	r.Header.Set("X-Org-Id", "acme")

	_, q, err := viewRequest(claimsCtx(), r)
	if err != nil {
		t.Fatalf("viewRequest err: %v", err)
	}
	if q.OrgSlug != "acme" {
		t.Fatalf("OrgSlug = %q, want acme (from validated X-Org-Id, not the ?orgSlug=evil param)", q.OrgSlug)
	}
	if q.TraceID != "t1" {
		t.Fatalf("TraceID = %q, want t1", q.TraceID)
	}
}

// TestViewRequest_FailsClosedWithoutTenant proves a span-view request with no
// gateway-asserted tenant is REFUSED — never run as an un-scoped, all-orgs query.
func TestViewRequest_FailsClosedWithoutTenant(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/observations", nil) // no X-Org-Id

	if _, _, err := viewRequest(claimsCtx(), r); err == nil {
		t.Fatal("viewRequest must FAIL CLOSED when X-Org-Id is absent (would otherwise read every tenant)")
	}

	// A blank header is likewise refused.
	r2 := httptest.NewRequest(http.MethodGet, "/api/observations", nil)
	r2.Header.Set("X-Org-Id", "   ")
	if _, _, err := viewRequest(claimsCtx(), r2); err == nil {
		t.Fatal("viewRequest must FAIL CLOSED on a blank X-Org-Id")
	}
}

// scoreRoute is the registered template for the two per-score routes. The id is
// a PATH segment, so what has to hold is that the segment the ROUTER matched is
// the score the handler acts on — hence a real router at the real template, not
// injected vars, which would prove only that the two halves of coretypes agree.
const scoreRoute = "/v1/o11y/llm/score/{id}"

func servedScoreID(t *testing.T, target string) (valuer.UUID, error) {
	t.Helper()

	var (
		id  valuer.UUID
		err error
	)
	router := mux.NewRouter()
	router.HandleFunc(scoreRoute, func(_ http.ResponseWriter, req *http.Request) {
		id, err = idFromPath(req)
	}).Methods(http.MethodGet)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("the route did not match (%d) — the test is not exercising the read", rec.Code)
	}
	return id, err
}

func TestIDFromPathReadsTheMatchedSegment(t *testing.T) {
	want := valuer.GenerateUUID()

	id, err := servedScoreID(t, "/v1/o11y/llm/score/"+want.String())
	if err != nil {
		t.Fatalf("idFromPath: %v", err)
	}
	if id != want {
		t.Fatalf("id = %s, want %s (the segment the router matched)", id, want)
	}
}

// A segment that is not a uuid is refused with the module's own code, not
// answered with the zero uuid — a malformed or unread id must never reach the
// store as a lookup for whatever the zero value matches.
func TestIDFromPathRefusesANonUUIDSegment(t *testing.T) {
	id, err := servedScoreID(t, "/v1/o11y/llm/score/not-a-uuid")
	if err == nil {
		t.Fatal("a non-uuid segment must be refused")
	}
	if !errors.Asc(err, llmobstypes.ErrCodeLLMObsInvalidInput) {
		t.Fatalf("want the llmobs invalid-input code, got %v", err)
	}
	if id != (valuer.UUID{}) {
		t.Fatalf("id = %s, want the zero uuid on refusal", id)
	}
}
