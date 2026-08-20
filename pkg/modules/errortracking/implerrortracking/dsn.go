package implerrortracking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// This is the Sentry-native DSN credential model, shared by every ingest face: the
// caller presents a public key that proves it holds the ingest credential for one
// segment. The key is STATELESS and KMS-backed rather than a per-key secret table:
//
//	publicKey(segment, v) = "<v>:" + hex(HMAC-SHA256(platformIngestSecret, "org:"+segment+":v"+v))
//
// The platform secret comes from KMS (never plaintext, never committed). The key
// carries its VERSION so ONE segment can be rotated in isolation: bump that segment's
// watermark and only its below-watermark DSNs stop verifying — no global secret roll.
// Verifying is a version check plus a constant-time compare.
//
// The segment is the /v1/sentry project UUID, and its watermark is the project's own
// KeyVersion column, so rotation is a single-row bump with no shared revocation table.

// publicKeyForVersion derives the versioned ingest public key for a DSN segment.
func publicKeyForVersion(secret []byte, project string, version int) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("org:" + strings.TrimSpace(project) + ":v" + strconv.Itoa(version)))
	return strconv.Itoa(version) + ":" + hex.EncodeToString(m.Sum(nil))
}

// verifyKey constant-time compares a presented "<v>:<hmac>" key against the expected
// one for its segment, rejecting versions below the rotation watermark. An empty
// secret or key, a malformed version, or a below-min version never verify
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
