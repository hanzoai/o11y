package implerrortracking

import (
	"regexp"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

// Error payloads routinely embed secrets (API keys, bearer/JWT tokens, DB
// connection strings, private keys, PANs) and end-user PII (emails, IPs). We treat
// storing those verbatim as a leak-at-rest. Two layers, mirroring the llmobs
// capture-messages precedent (default-secure):
//
//   - Secret patterns are ALWAYS redacted — there is no mode in which we persist an
//     sk-… key or a password in a DSN. Non-negotiable.
//   - PII (email/IP) is scrubbed UNLESS the operator opts in via
//     O11Y_ERRORTRACKING_CAPTURE_PII (default false → scrub). Fail-secure.
//
// Redaction runs before the value enters the fingerprint, so two errors that differ
// only by an embedded secret/email still group together.

const (
	redactedMark = "[redacted]"
	emailMark    = "[email]"
	ipMark       = "[ip]"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN[ A-Z]*PRIVATE KEY-----[\s\S]*?-----END[ A-Z]*PRIVATE KEY-----`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}`), // JWT
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/-]{12,}=*`),                    // bearer token
	regexp.MustCompile(`\b(?:sk|pk|rk)-[A-Za-z0-9]{2,}-?[A-Za-z0-9]{12,}`),           // openai-style
	regexp.MustCompile(`\b(?:sk|pk)_(?:live|test)_[A-Za-z0-9]{16,}`),                 // stripe
	regexp.MustCompile(`\bhk-[A-Za-z0-9]{16,}`),                                      // hanzo key
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),                                       // aws access key id
	regexp.MustCompile(`\bASIA[0-9A-Z]{16}\b`),                                       // aws sts key id
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{20,}`),                                   // google api key
	regexp.MustCompile(`\bgh[posru]_[A-Za-z0-9]{20,}`),                               // github token
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),                             // slack token
	regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:@/]+:[^\s@/]+@`),              // creds in a URL/DSN
	regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`),                                     // PAN-like digit run
}

var (
	reEmail = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	reIPv4  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reIPv6  = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{1,4}:){2,7}[0-9A-Fa-f]{1,4}\b`)
)

// redactSecrets removes known secret shapes. Always applied.
func redactSecrets(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, redactedMark)
	}
	return s
}

// scrubPII masks emails and IPs. Applied unless PII capture is enabled.
func scrubPII(s string) string {
	s = reEmail.ReplaceAllString(s, emailMark)
	s = reIPv6.ReplaceAllString(s, ipMark)
	s = reIPv4.ReplaceAllString(s, ipMark)
	return s
}

// sanitize applies the redaction policy to a free-text field.
func sanitize(s string, capturePII bool) string {
	if s == "" {
		return s
	}
	s = redactSecrets(s)
	if !capturePII {
		s = scrubPII(s)
	}
	return s
}

// sanitizeOccurrence redacts the fields that carry attacker/user-controlled text.
// Frames (code locations) are left intact; value/tags/user are the leak surface.
func sanitizeOccurrence(occ *errortrackingtypes.Occurrence, capturePII bool) {
	occ.Value = sanitize(occ.Value, capturePII)
	for k, v := range occ.Tags {
		occ.Tags[k] = sanitize(v, capturePII)
	}
	if occ.User != nil {
		occ.User.Email = sanitize(occ.User.Email, capturePII)
		occ.User.Username = sanitize(occ.User.Username, capturePII)
		if !capturePII {
			occ.User.IP = ""
		}
		if occ.User.ID == "" && occ.User.Email == "" && occ.User.Username == "" && occ.User.IP == "" {
			occ.User = nil
		}
	}
}
