package impluser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// GetMyUser ANSWERS FROM THE CLAIMS, and this is the test that says so.
//
// The handler used to hold a Getter and SELECT the caller's row, which made
// o11y's own bookkeeping a precondition for rendering the console: a person the
// edge had authenticated, whose row was missing, got "user not found" on the one
// call the console uses to learn who it is. NewHandler now takes NOTHING — there
// is no store to reach — so the coupling is gone at the type level and this test
// pins the values that come out.
//
// The mutation that must turn this red: read any field from anywhere but the
// claims (a row, a config, a default) and the assertions below stop matching the
// claims they were built from.
func TestGetMyUserIsTheClaims(t *testing.T) {
	claims := authtypes.Claims{
		UserID:    "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		OrgID:     "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		Email:     "ada@example.com",
		Principal: authtypes.PrincipalUser,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/o11y/users/me", http.NoBody)
	NewHandler().GetMyUser(rec, req.WithContext(authtypes.NewContextWithClaims(req.Context(), claims)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", rec.Code, rec.Body)
	}

	var got struct {
		Status string     `json:"status"`
		Data   types.User `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body)
	}

	if got.Data.ID.StringValue() != claims.UserID {
		t.Errorf("id=%q, want the IAM subject %q", got.Data.ID.StringValue(), claims.UserID)
	}
	if got.Data.OrgID.StringValue() != claims.OrgID {
		t.Errorf("orgId=%q, want the asserted org %q", got.Data.OrgID.StringValue(), claims.OrgID)
	}
	if got.Data.Email.StringValue() != claims.Email {
		t.Errorf("email=%q, want the asserted address %q", got.Data.Email.StringValue(), claims.Email)
	}
	if got.Data.DisplayName != claims.Email {
		t.Errorf("displayName=%q, want the asserted address %q — the seated row carried the same value", got.Data.DisplayName, claims.Email)
	}
	if got.Data.IsRoot {
		t.Error("isRoot=true — an IAM session is never the local root user")
	}
	if got.Data.Status != types.UserStatusActive {
		t.Errorf("status=%q, want active", got.Data.Status)
	}
}

// No claims, no answer. The route is OpenAccess — "is anyone authenticated" —
// so the refusal has to come from here, and it has to be a refusal.
func TestGetMyUserWithoutClaimsIsUnauthenticated(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler().GetMyUser(rec, httptest.NewRequest(http.MethodGet, "/v1/o11y/users/me", http.NoBody))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s, want 401", rec.Code, rec.Body)
	}
}
