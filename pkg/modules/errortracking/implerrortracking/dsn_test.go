package implerrortracking

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte("kms-platform-ingest-secret")

func TestVerifyKey_RoundTrip(t *testing.T) {
	key := publicKeyFor(testSecret, "acme")
	assert.True(t, verifyKey(testSecret, "acme", key, 0), "the derived key must verify for its project")
}

func TestVerifyKey_RejectsWrongProject(t *testing.T) {
	key := publicKeyFor(testSecret, "acme")
	assert.False(t, verifyKey(testSecret, "evil", key, 0), "a key minted for acme must not verify for another project")
}

func TestVerifyKey_RejectsWrongSecret(t *testing.T) {
	key := publicKeyFor(testSecret, "acme")
	assert.False(t, verifyKey([]byte("different-secret"), "acme", key, 0))
}

func TestVerifyKey_FailsClosed(t *testing.T) {
	assert.False(t, verifyKey(nil, "acme", "anything", 0), "no secret => fail closed")
	assert.False(t, verifyKey(testSecret, "acme", "", 0), "no presented key => fail closed")
}

// The MOST important parity test: the org UUID the ingest path derives from a DSN
// project MUST equal iamidentn.toUUID("org", slug) — otherwise a row written by
// ingest would be invisible to the org's IAM-authenticated reads. This replicates
// iamidentn's exact formula and asserts equality.
func TestOrgUUIDFromProject_MatchesIAMMapping(t *testing.T) {
	for _, slug := range []string{"hanzo", "acme", "zoo"} {
		want := uuid.NewSHA1(uuid.NameSpaceURL, []byte("hanzo:o11y:org:"+slug))
		got, ok := orgUUIDFromProject(slug)
		require.True(t, ok)
		assert.Equal(t, want.String(), got.String(), "ingest org UUID must match the IAM read-path UUID for slug %q", slug)
	}
}

func TestOrgUUIDFromProject_RawUUIDPassthrough(t *testing.T) {
	u := valuer.GenerateUUID()
	got, ok := orgUUIDFromProject(u.String())
	require.True(t, ok)
	assert.Equal(t, u.String(), got.String(), "a project that is already a UUID is used as-is")
}

func TestOrgUUIDFromProject_EmptyRejected(t *testing.T) {
	_, ok := orgUUIDFromProject("")
	assert.False(t, ok)
	_, ok = orgUUIDFromProject("   ")
	assert.False(t, ok)
}

func TestSentryKeyFromRequest_Header(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/acme/envelope/", nil)
	r.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_key=pubkey123, sentry_client=sentry.python/1.40")
	assert.Equal(t, "pubkey123", sentryKeyFromRequest(r))
}

func TestSentryKeyFromRequest_QueryFallback(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/acme/envelope/?sentry_key=qkey456", nil)
	assert.Equal(t, "qkey456", sentryKeyFromRequest(r))
}

func TestSentryKeyFromRequest_HeaderWins(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/acme/envelope/?sentry_key=qkey", nil)
	r.Header.Set("X-Sentry-Auth", "Sentry sentry_key=hkey")
	assert.Equal(t, "hkey", sentryKeyFromRequest(r))
}

func TestMintDSN(t *testing.T) {
	dsn := MintDSN(testSecret, "o11y.hanzo.ai", "acme")
	assert.Equal(t, "https://"+publicKeyFor(testSecret, "acme")+"@o11y.hanzo.ai/v1/o11y/acme", dsn)
}
