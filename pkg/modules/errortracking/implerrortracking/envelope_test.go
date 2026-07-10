package implerrortracking

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvelope_NewlineDelimitedEvent(t *testing.T) {
	body := []byte(`{"event_id":"9ec79c33","dsn":"https://k@h/1"}
{"type":"event"}
{"event_id":"9ec79c33","exception":{"values":[{"type":"ZeroDivisionError","value":"division by zero"}]}}
`)
	events, err := parseEnvelope(body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.NotNil(t, events[0].Exception)
	assert.Equal(t, "ZeroDivisionError", events[0].Exception.Values[0].Type)
}

func TestParseEnvelope_LengthDelimitedEvent(t *testing.T) {
	payload := `{"event_id":"abc","exception":{"values":[{"type":"E","value":"boom"}]}}`
	body := []byte(fmt.Sprintf("{\"event_id\":\"abc\"}\n{\"type\":\"event\",\"length\":%d}\n%s\n", len(payload), payload))
	events, err := parseEnvelope(body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "boom", events[0].Exception.Values[0].Value)
}

func TestParseEnvelope_SkipsNonEventItems(t *testing.T) {
	body := []byte(`{"event_id":"x"}
{"type":"session"}
{"sid":"s1","status":"ok"}
{"type":"event"}
{"event_id":"x","exception":{"values":[{"type":"E"}]}}
{"type":"transaction"}
{"spans":[]}
`)
	events, err := parseEnvelope(body)
	require.NoError(t, err)
	require.Len(t, events, 1, "only the event item is extracted")
	assert.Equal(t, "E", events[0].Exception.Values[0].Type)
}

func TestParseEnvelope_MultipleEvents(t *testing.T) {
	body := []byte(`{"event_id":"x"}
{"type":"event"}
{"exception":{"values":[{"type":"A"}]}}
{"type":"event"}
{"exception":{"values":[{"type":"B"}]}}
`)
	events, err := parseEnvelope(body)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "A", events[0].Exception.Values[0].Type)
	assert.Equal(t, "B", events[1].Exception.Values[0].Type)
}

func TestParseEnvelope_EmptyFails(t *testing.T) {
	_, err := parseEnvelope(nil)
	require.Error(t, err)
}

func TestParseStoreBody(t *testing.T) {
	events, err := parseStoreBody([]byte(`{"event_id":"z","exception":{"values":[{"type":"RuntimeError","value":"nope"}]}}`))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "RuntimeError", events[0].Exception.Values[0].Type)
}

func TestParseStoreBody_EmptyFails(t *testing.T) {
	_, err := parseStoreBody([]byte("  "))
	require.Error(t, err)
}

func TestDecodeBody_Identity(t *testing.T) {
	got, err := decodeBody([]byte("hello"), "")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestDecodeBody_Gzip(t *testing.T) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write([]byte("compressed payload"))
	require.NoError(t, w.Close())

	got, err := decodeBody(buf.Bytes(), "gzip")
	require.NoError(t, err)
	assert.Equal(t, "compressed payload", string(got))
}

func TestDecodeBody_Deflate(t *testing.T) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte("zlib payload"))
	require.NoError(t, w.Close())

	got, err := decodeBody(buf.Bytes(), "deflate")
	require.NoError(t, err)
	assert.Equal(t, "zlib payload", string(got))
}

// End-to-end through decode+parse: a gzipped envelope decodes then parses.
func TestDecodeThenParse_GzippedEnvelope(t *testing.T) {
	raw := `{"event_id":"x"}
{"type":"event"}
{"exception":{"values":[{"type":"OutOfMemory"}]}}
`
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write([]byte(raw))
	require.NoError(t, w.Close())

	decoded, err := decodeBody(buf.Bytes(), "gzip")
	require.NoError(t, err)
	events, err := parseEnvelope(decoded)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "OutOfMemory", events[0].Exception.Values[0].Type)
}
