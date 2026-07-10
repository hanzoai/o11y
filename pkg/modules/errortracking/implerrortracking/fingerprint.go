package implerrortracking

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

// The grouping algorithm is a from-scratch, deterministic reimplementation of the
// public Sentry grouping MODEL (exception type + normalized crash frame, with a
// message fallback), not a port of any upstream code. It runs at ingest so the
// Issues list is a plain org-scoped SELECT rather than an aggregation over an
// org-less exception table.

// defaultFingerprintToken is the Sentry sentinel that expands to the computed
// default fingerprint inside a client-supplied fingerprint array.
const defaultFingerprintToken = "{{ default }}"

var (
	reUUID       = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	reHexAddr    = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
	reLongHex    = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)
	reNumber     = regexp.MustCompile(`\b\d[\d.,_]*\b`)
	reQuoted     = regexp.MustCompile(`'[^']*'|"[^"]*"`)
	reWhitespace = regexp.MustCompile(`\s+`)
	reFuncNoise  = regexp.MustCompile(`0x[0-9a-fA-F]+`)
)

// computeFingerprint returns the stable 64-hex-char group key for an occurrence.
// A client-supplied fingerprint is honored (the Sentry contract), with the
// "{{ default }}" token expanded to the computed default parts.
func computeFingerprint(occ *errortrackingtypes.Occurrence, custom []string) string {
	def := defaultFingerprintParts(occ)

	var parts []string
	if len(custom) > 0 {
		for _, c := range custom {
			if strings.TrimSpace(c) == defaultFingerprintToken {
				parts = append(parts, def...)
				continue
			}
			parts = append(parts, c)
		}
	} else {
		parts = def
	}

	return hashParts(parts)
}

// defaultFingerprintParts builds the canonical grouping components: exception
// type + the normalized crash frame, falling back to a normalized message and
// then the transaction, so an occurrence with no useful signal still groups
// stably instead of collapsing every error into one bucket.
func defaultFingerprintParts(occ *errortrackingtypes.Occurrence) []string {
	parts := make([]string, 0, 2)
	if occ.Type != "" {
		parts = append(parts, "type:"+occ.Type)
	}

	if frame := pickCrashFrame(occ.Frames); frame != nil {
		if sig := normalizeFrame(frame); sig != "" {
			parts = append(parts, "frame:"+sig)
			return parts
		}
	}

	if occ.Value != "" {
		parts = append(parts, "value:"+normalizeMessage(occ.Value))
		return parts
	}
	if occ.Transaction != "" {
		parts = append(parts, "txn:"+occ.Transaction)
	}
	if len(parts) == 0 {
		parts = append(parts, "level:"+occ.Level)
	}
	return parts
}

// pickCrashFrame returns the frame the error occurred in: the innermost (last)
// in-app frame if any, else the innermost frame. Sentry orders frames caller→callee,
// so the crash site is the last element.
func pickCrashFrame(frames []errortrackingtypes.Frame) *errortrackingtypes.Frame {
	if len(frames) == 0 {
		return nil
	}
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].InApp {
			return &frames[i]
		}
	}
	return &frames[len(frames)-1]
}

// normalizeFrame renders a frame to a host-independent signature: normalized
// function name at its module (or normalized filename). Line/column numbers and
// absolute paths are dropped so the same logical crash site groups across
// deploys, releases and machines.
func normalizeFrame(f *errortrackingtypes.Frame) string {
	fn := normalizeFunction(f.Function)

	loc := f.Module
	if loc == "" {
		loc = normalizeFilename(f.Filename)
	}
	if loc == "" {
		loc = normalizeFilename(f.AbsPath)
	}

	switch {
	case fn != "" && loc != "":
		return fn + "@" + loc
	case fn != "":
		return fn
	default:
		return loc
	}
}

func normalizeFunction(fn string) string {
	fn = strings.TrimSpace(fn)
	if fn == "" {
		return ""
	}
	// Drop runtime address noise inside anonymous/closure names.
	fn = reFuncNoise.ReplaceAllString(fn, "")
	return strings.TrimSpace(fn)
}

// normalizeFilename keeps the basename and its immediate parent (enough to be
// unique in practice) and masks content-hash / versioned segments so bundled
// asset names like app.9f3a2b1c.js group across builds.
func normalizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.IndexAny(name, "?#"); i >= 0 {
		name = name[:i]
	}
	segs := strings.Split(strings.Trim(name, "/"), "/")
	// Keep the last two path segments.
	if len(segs) > 2 {
		segs = segs[len(segs)-2:]
	}
	for i, s := range segs {
		s = reLongHex.ReplaceAllString(s, "*")
		s = reNumber.ReplaceAllString(s, "*")
		segs[i] = s
	}
	return strings.Join(segs, "/")
}

// normalizeMessage collapses variadic detail (ids, numbers, hex, quoted literals)
// so "user 123 missing" and "user 456 missing" land in one issue.
func normalizeMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	msg = reUUID.ReplaceAllString(msg, "<uuid>")
	msg = reHexAddr.ReplaceAllString(msg, "<hex>")
	msg = reQuoted.ReplaceAllString(msg, "<str>")
	msg = reLongHex.ReplaceAllString(msg, "<hex>")
	msg = reNumber.ReplaceAllString(msg, "<num>")
	msg = reWhitespace.ReplaceAllString(msg, " ")
	return strings.TrimSpace(msg)
}

func hashParts(parts []string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}
