package implerrortracking

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"io"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

// maxDecodedBytes caps the decompressed payload so a small gzip bomb cannot
// exhaust memory on this public endpoint. 24 MiB comfortably fits large stack
// traces / batched envelopes while staying bounded.
const maxDecodedBytes = 24 << 20

// decodeBody transparently inflates gzip / zlib / raw-deflate request bodies (the
// Content-Encodings Sentry SDKs use), bounded by maxDecodedBytes. Identity/unknown
// encodings pass through unchanged.
func decodeBody(body []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "identity":
		return body, nil
	case "gzip", "x-gzip":
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer func() { _ = r.Close() }()
		return io.ReadAll(io.LimitReader(r, maxDecodedBytes))
	case "deflate":
		if r, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			defer func() { _ = r.Close() }()
			return io.ReadAll(io.LimitReader(r, maxDecodedBytes))
		}
		// Some clients send headerless raw DEFLATE.
		fr := flate.NewReader(bytes.NewReader(body))
		defer func() { _ = fr.Close() }()
		return io.ReadAll(io.LimitReader(fr, maxDecodedBytes))
	default:
		return body, nil
	}
}

// parseStoreBody decodes a legacy `/store/` payload: a single event JSON.
func parseStoreBody(body []byte) ([]*errortrackingtypes.SentryEvent, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, errors.Newf(errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "empty store payload")
	}
	var ev errortrackingtypes.SentryEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, errors.Wrapf(err, errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "invalid store event json")
	}
	return []*errortrackingtypes.SentryEvent{&ev}, nil
}

// parseEnvelope decodes a Sentry envelope and returns every `event`-type item.
// The envelope is newline-framed: a header line, then repeating (item-header,
// payload) pairs where a payload is either length-delimited (per its header) or
// runs to the next newline. Non-event items (transaction/session/attachment/…)
// are skipped. Malformed tails are tolerated — we return what parsed cleanly.
func parseEnvelope(body []byte) ([]*errortrackingtypes.SentryEvent, error) {
	pos := 0
	readLine := func() ([]byte, bool) {
		if pos >= len(body) {
			return nil, false
		}
		if nl := bytes.IndexByte(body[pos:], '\n'); nl >= 0 {
			line := body[pos : pos+nl]
			pos += nl + 1
			return line, true
		}
		line := body[pos:]
		pos = len(body)
		return line, true
	}

	// Envelope header (event_id / dsn / sent_at) — required to be present but not
	// otherwise consumed here.
	if _, ok := readLine(); !ok {
		return nil, errors.Newf(errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "empty envelope")
	}

	var events []*errortrackingtypes.SentryEvent
	for pos < len(body) {
		hdrLine, ok := readLine()
		if !ok {
			break
		}
		if len(bytes.TrimSpace(hdrLine)) == 0 {
			continue
		}
		var ih errortrackingtypes.EnvelopeItemHeader
		if err := json.Unmarshal(hdrLine, &ih); err != nil {
			break // corrupt framing; stop rather than misread payloads as headers
		}

		var payload []byte
		if ih.Length != nil && *ih.Length >= 0 {
			end := pos + *ih.Length
			if end > len(body) {
				end = len(body)
			}
			payload = body[pos:end]
			pos = end
			if pos < len(body) && body[pos] == '\n' {
				pos++
			}
		} else {
			payload, ok = readLine()
			if !ok {
				break
			}
		}

		if ih.Type == "event" {
			var ev errortrackingtypes.SentryEvent
			if err := json.Unmarshal(payload, &ev); err == nil {
				events = append(events, &ev)
			}
		}
	}
	return events, nil
}
