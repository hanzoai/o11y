package impllmobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/types/authtypes"
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
