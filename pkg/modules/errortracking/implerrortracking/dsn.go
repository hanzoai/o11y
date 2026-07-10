package implerrortracking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The ingest endpoints authenticate with the Sentry-native DSN model: the caller
// presents a public key that proves it holds an org's ingest credential. We make
// that key STATELESS and KMS-backed rather than adding a per-key secret table:
//
//	publicKey(org, v) = "<v>:" + hex(HMAC-SHA256(platformIngestSecret, "org:"+org+":v"+v))
//
// The platform secret comes from KMS (never plaintext, never committed). The key
// carries its VERSION so ONE org can be rotated in isolation: bump that org's
// min-version (RevocationStore.Rotate) and only its below-min DSNs stop verifying —
// no global secret roll. Verifying is a version check + a constant-time compare.
//
// The org travels in the DSN project segment; the key proves the caller may write
// to THAT org at THAT version. Resolution reuses iamidentn's exact UUIDv5 mapping so
// the row written here is read back by exactly that tenant.

const defaultKeyVersion = 1

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

// publicKeyForVersion derives the versioned ingest public key for a project.
func publicKeyForVersion(secret []byte, project string, version int) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("org:" + strings.TrimSpace(project) + ":v" + strconv.Itoa(version)))
	return strconv.Itoa(version) + ":" + hex.EncodeToString(m.Sum(nil))
}

// publicKeyFor derives the default (v1) key.
func publicKeyFor(secret []byte, project string) string {
	return publicKeyForVersion(secret, project, defaultKeyVersion)
}

// verifyKey constant-time compares a presented "<v>:<hmac>" key against the expected
// one for its project, rejecting versions below the org's revocation watermark. An
// empty secret or key, a malformed version, or a below-min version never verify
// (fail closed).
func verifyKey(secret []byte, project, presented string, minVersion int) bool {
	if len(secret) == 0 || presented == "" {
		return false
	}
	i := strings.IndexByte(presented, ':')
	if i <= 0 {
		return false
	}
	version, err := strconv.Atoi(presented[:i])
	if err != nil || version <= 0 {
		return false
	}
	if version < minVersion {
		return false // revoked by rotation
	}
	expected := publicKeyForVersion(secret, project, version)
	return hmac.Equal([]byte(expected), []byte(presented))
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
//	Sentry sentry_version=7, sentry_key=1:abc123, sentry_client=sentry.python/1.2
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

// MintDSN builds the default-version DSN an operator hands to an app to report into
// an org. host is the ingest origin (e.g. "o11y.hanzo.ai"); the SDK derives its
// endpoint as https://<host>/v1/o11y/api/<org>/envelope/, which the existing
// /v1/o11y mount forwards to this module's ingest route.
func MintDSN(secret []byte, host, org string) string {
	return MintDSNVersion(secret, host, org, defaultKeyVersion)
}

// MintDSNVersion builds a DSN at a specific key version (used after rotating an org).
func MintDSNVersion(secret []byte, host, org string, version int) string {
	return "https://" + publicKeyForVersion(secret, org, version) + "@" + host + "/v1/o11y/" + org
}
