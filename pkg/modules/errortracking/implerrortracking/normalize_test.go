package implerrortracking

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustEvent(t *testing.T, j string) *errortrackingtypes.SentryEvent {
	t.Helper()
	var e errortrackingtypes.SentryEvent
	require.NoError(t, json.Unmarshal([]byte(j), &e))
	return &e
}

func TestNormalize_UnixTimestamp(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","timestamp":1700000000.5,"exception":{"values":[{"type":"E","value":"v"}]}}`))
	assert.Equal(t, int64(1700000000), occ.Timestamp.Unix())
}

func TestNormalize_ISOTimestamp(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","timestamp":"2023-11-14T22:13:20Z","exception":{"values":[{"type":"E"}]}}`))
	assert.Equal(t, 2023, occ.Timestamp.Year())
	assert.Equal(t, time.November, occ.Timestamp.Month())
}

func TestNormalize_MissingTimestampDefaultsNow(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","exception":{"values":[{"type":"E"}]}}`))
	assert.WithinDuration(t, time.Now().UTC(), occ.Timestamp, 5*time.Second)
}

func TestNormalize_MessageString(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","message":"plain failure"}`))
	assert.Equal(t, "Message", occ.Type)
	assert.Equal(t, "plain failure", occ.Value)
}

func TestNormalize_MessageObject(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","message":{"message":"raw %s","formatted":"raw boom"}}`))
	assert.Equal(t, "raw boom", occ.Value, "formatted preferred over template")
}

func TestNormalize_TagsMap(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","message":"m","tags":{"env":"prod","code":500}}`))
	assert.Equal(t, "prod", occ.Tags["env"])
	assert.Equal(t, "500", occ.Tags["code"], "numeric tag coerced to string")
}

func TestNormalize_TagsArray(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","message":"m","tags":[["env","staging"],["region","sfo"]]}`))
	assert.Equal(t, "staging", occ.Tags["env"])
	assert.Equal(t, "sfo", occ.Tags["region"])
}

func TestNormalize_PrimaryExceptionIsLast(t *testing.T) {
	// Chained: cause first, thrown last. Sentry treats the last value as primary.
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","exception":{"values":[
		{"type":"IOError","value":"disk"},
		{"type":"ServiceError","value":"upstream failed","stacktrace":{"frames":[{"function":"call","module":"svc","in_app":true}]}}
	]}}`))
	assert.Equal(t, "ServiceError", occ.Type)
	assert.Equal(t, "upstream failed", occ.Value)
	require.Len(t, occ.Frames, 1)
	assert.Equal(t, "call", occ.Frames[0].Function)
}

func TestNormalize_CulpritFromTransaction(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","transaction":"GET /users","exception":{"values":[{"type":"E"}]}}`))
	assert.Equal(t, "GET /users", occ.Culprit)
}

func TestNormalize_CulpritFromCrashFrame(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","exception":{"values":[{"type":"E","stacktrace":{"frames":[
		{"function":"outer","module":"a","in_app":true},
		{"function":"boom","filename":"/srv/app/w.py","in_app":true}
	]}}]}}`))
	assert.Equal(t, "boom in w.py", occ.Culprit)
}

func TestNormalize_LevelDefaultsError(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","exception":{"values":[{"type":"E"}]}}`))
	assert.Equal(t, "error", occ.Level)
}

func TestNormalize_TraceContext(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","exception":{"values":[{"type":"E"}]},"contexts":{"trace":{"trace_id":"abc123","span_id":"def456"}}}`))
	assert.Equal(t, "abc123", occ.TraceID)
	assert.Equal(t, "def456", occ.SpanID)
}

// An empty/garbage event must not panic and must still yield a fingerprint.
func TestNormalize_EmptyEventSafe(t *testing.T) {
	occ := normalizeEvent(&errortrackingtypes.SentryEvent{})
	require.NotNil(t, occ)
	assert.NotEmpty(t, occ.Fingerprint)
	assert.Equal(t, "error", occ.Level)
}

func TestNormalize_StampsFingerprint(t *testing.T) {
	occ := normalizeEvent(mustEvent(t, `{"event_id":"a","exception":{"values":[{"type":"E","value":"v"}]}}`))
	assert.Len(t, occ.Fingerprint, 64)
}
