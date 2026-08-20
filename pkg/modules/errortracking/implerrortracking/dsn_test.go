package implerrortracking

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testSecret = []byte("kms-platform-ingest-secret")

// publicKeyAt is the test's own shorthand for the first key version. Production
// derives the version from the project's rotation watermark, so the package itself
// has no default-version helper.
func publicKeyAt(secret []byte, segment string) string {
	return publicKeyForVersion(secret, segment, 1)
}

func TestVerifyKey_RoundTrip(t *testing.T) {
	key := publicKeyAt(testSecret, "acme")
	assert.True(t, verifyKey(testSecret, "acme", key, 0), "the derived key must verify for its segment")
}

func TestVerifyKey_RejectsWrongSegment(t *testing.T) {
	key := publicKeyAt(testSecret, "acme")
	assert.False(t, verifyKey(testSecret, "evil", key, 0), "a key minted for acme must not verify for another segment")
}

func TestVerifyKey_RejectsWrongSecret(t *testing.T) {
	key := publicKeyAt(testSecret, "acme")
	assert.False(t, verifyKey([]byte("different-secret"), "acme", key, 0))
}

// Version handling and the rotation watermark are covered by
// TestVerifyKey_VersionedAndRevocation in hardening_test.go; this asserts the two
// cases that carry no version at all.
func TestVerifyKey_FailsClosed(t *testing.T) {
	assert.False(t, verifyKey(nil, "acme", "anything", 0), "no secret => fail closed")
	assert.False(t, verifyKey(testSecret, "acme", "", 0), "no presented key => fail closed")
}

func TestSentryKeyFromRequest_Header(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/sentry/acme/envelope/", nil)
	r.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_key=pubkey123, sentry_client=sentry.python/1.40")
	assert.Equal(t, "pubkey123", sentryKeyFromRequest(r))
}

func TestSentryKeyFromRequest_QueryFallback(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/sentry/acme/envelope/?sentry_key=qkey456", nil)
	assert.Equal(t, "qkey456", sentryKeyFromRequest(r))
}

func TestSentryKeyFromRequest_HeaderWins(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/sentry/acme/envelope/?sentry_key=qkey", nil)
	r.Header.Set("X-Sentry-Auth", "Sentry sentry_key=hkey")
	assert.Equal(t, "hkey", sentryKeyFromRequest(r))
}
