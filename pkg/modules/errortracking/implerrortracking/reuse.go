package implerrortracking

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// This file is the DELIBERATE, exported reuse surface of the Sentry-wire ingest
// engine. The Hanzo Sentry product face (pkg/modules/sentry) COMPOSES these
// primitives verbatim — ONE ingest engine, two product faces: errortracking's
// org-keyed DSN path and sentry's project-keyed DSN path. Every symbol here is a
// thin, behavior-preserving alias over the unexported original errortracking's own
// handler already uses, so there is exactly one implementation of decode, parse,
// normalize/scrub, DSN-key derivation and rate limiting. No logic is duplicated and
// the reviewed ingest path stays byte-unchanged.

// DecodeBody decompresses a raw ingest body per Content-Encoding, bounded against a
// decompression bomb (the identical MaxDecodedBytes cap the errortracking path uses).
func DecodeBody(body []byte, encoding string) ([]byte, error) { return decodeBody(body, encoding) }

// ParseEnvelope extracts events from a modern Sentry envelope, capped at
// MaxEventsPerEnvelope so one request cannot fan out unbounded.
func ParseEnvelope(decoded []byte) ([]*errortrackingtypes.SentryEvent, error) {
	return parseEnvelope(decoded)
}

// ParseStoreBody extracts the single event of a legacy /store/ body.
func ParseStoreBody(decoded []byte) ([]*errortrackingtypes.SentryEvent, error) {
	return parseStoreBody(decoded)
}

// NormalizeEvent turns a decoded Sentry event into the canonical, fingerprinted
// Occurrence. Total (never errors on a malformed client payload) and fail-secure:
// secrets are always redacted and end-user PII is scrubbed unless capturePII is set.
func NormalizeEvent(e *errortrackingtypes.SentryEvent, capturePII bool) *errortrackingtypes.Occurrence {
	return normalizeEvent(e, ingestOpts{capturePII: capturePII})
}

// PublicKeyForVersion derives the versioned DSN public key for a DSN path segment.
// The Sentry face passes the PROJECT UUID as the segment (errortracking passes the
// ORG); the HMAC-over-platform-secret derivation is identical.
func PublicKeyForVersion(secret []byte, segment string, version int) string {
	return publicKeyForVersion(secret, segment, version)
}

// VerifyKey constant-time verifies a presented "<v>:<hmac>" DSN key for a segment,
// rejecting versions below minVersion (the rotation watermark). Fail-closed on an
// empty secret/key or a malformed/too-low version.
func VerifyKey(secret []byte, segment, presented string, minVersion int) bool {
	return verifyKey(secret, segment, presented, minVersion)
}

// SentryKeyFromRequest extracts the presented DSN public key from the Sentry auth
// surface (X-Sentry-Auth header, then ?sentry_key). The envelope-body DSN is never
// trusted as a credential channel.
func SentryKeyFromRequest(r *http.Request) string { return sentryKeyFromRequest(r) }

// RateLimiter is the per-key token-bucket ingest limiter. The Sentry face keys it on
// the project UUID; errortracking keys it on the org UUID.
type RateLimiter = rateLimiter

// NewRateLimiter builds a token-bucket limiter (steady-state events/sec, burst) with
// the SAME policy the errortracking ingest path applies.
func NewRateLimiter(rate, burst float64) *RateLimiter { return newRateLimiter(rate, burst) }

// Allow reports whether the key (org or project UUID) is within its rate budget and
// consumes a token when it is. Buckets are created lazily and only for a key that
// already passed DSN verification.
func (l *rateLimiter) Allow(key valuer.UUID) bool { return l.allow(key) }

// Shared ingest-policy constants, so the Sentry face applies identical budgets and
// payload/event bounds to its own ingest pipeline.
const (
	IngestRatePerSec     = ingestRatePerSec
	IngestBurst          = ingestBurst
	MaxDecodedBytes      = maxDecodedBytes
	MaxEventsPerEnvelope = maxEventsPerEnvelope
	MaxCompressedBody    = maxCompressedBody
)
