package implerrortracking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The ingest endpoints authenticate with the Sentry-native DSN model: the caller
// presents a public key that proves it holds an org's ingest credential. We make
// that key STATELESS and KMS-backed rather than adding a key table for the MVP:
//
//	publicKey(org) = HMAC-SHA256(platformIngestSecret, "org:"+org)
//
// The platform secret comes from KMS (never plaintext, never committed). Verifying
// is a constant-time compare; there is nothing to look up, and rotating the KMS
// secret revokes every DSN at once. Per-org revocable keys are the fast-follow.
//
// The org travels in the DSN project segment; the key proves the caller may write
// to THAT org. Resolution reuses iamidentn's exact UUIDv5 mapping so the row
// written here is read back by exactly that tenant.

// orgUUIDFromProject maps a DSN project segment to the o11y org UUID. It mirrors
// iamidentn.toUUID("org", …) BYTE-FOR-BYTE (a raw UUID passes through; a slug is
// UUIDv5 over the URL namespace with the "hanzo:o11y:org:" prefix) so ingest and
// the IAM read path resolve the SAME tenant id.
func orgUUIDFromProject(project string) (valuer.UUID, bool) {
	project = strings.TrimSpace(project)
	if project == "" {
		return valuer.UUID{}, false
	}
	if u, err := valuer.NewUUID(project); err == nil {
		return u, true
	}
	derived := uuid.NewSHA1(uuid.NameSpaceURL, []byte("hanzo:o11y:org:"+project))
	return valuer.MustNewUUID(derived.String()), true
}

// publicKeyFor derives the deterministic ingest public key for a project.
func publicKeyFor(secret []byte, project string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("org:" + strings.TrimSpace(project)))
	return hex.EncodeToString(m.Sum(nil))
}

// verifyKey constant-time compares a presented key against the expected one for a
// project. An empty secret or key never verifies (fail closed).
func verifyKey(secret []byte, project, presented string) bool {
	if len(secret) == 0 || presented == "" {
		return false
	}
	want := publicKeyFor(secret, project)
	return hmac.Equal([]byte(want), []byte(presented))
}

// sentryKeyFromRequest extracts the presented public key from the Sentry auth
// surface, in precedence order: the X-Sentry-Auth header, then the ?sentry_key
// query param. (The envelope-header DSN is intentionally NOT trusted as an auth
// source — it is client body, not a credential channel.)
func sentryKeyFromRequest(r *http.Request) string {
	if k := parseSentryAuthHeader(r.Header.Get("X-Sentry-Auth")); k != "" {
		return k
	}
	return strings.TrimSpace(r.URL.Query().Get("sentry_key"))
}

// parseSentryAuthHeader pulls sentry_key out of a header like:
//
//	Sentry sentry_version=7, sentry_key=abc123, sentry_client=sentry.python/1.2
func parseSentryAuthHeader(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	h = strings.TrimPrefix(h, "Sentry ")
	for _, part := range strings.Split(h, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == "sentry_key" {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

// MintDSN builds the DSN an operator hands to an app to report into a given org.
// host is the ingest origin (e.g. "o11y.hanzo.ai"); the resulting DSN is
//
//	https://<publicKey>@<host>/v1/o11y/<org>
//
// from which the Sentry SDK derives its endpoint as
// https://<host>/v1/o11y/api/<org>/envelope/ — which the existing /v1/o11y mount
// forwards to this module's /api/{project}/envelope/ route. No gateway change.
func MintDSN(secret []byte, host, org string) string {
	return "https://" + publicKeyFor(secret, org) + "@" + host + "/v1/o11y/" + org
}
