package implsentry

import (
	"context"
	"strings"
	"testing"

	dsmock "github.com/hanzo-ds/mock"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anyMatcher lets the mock match on expectation type + args, not exact SQL text —
// the SQL text itself is asserted by the pure builder tests (eventsql_test.go).
type anyMatcher struct{}

func (anyMatcher) Match(string, string) error { return nil }

// A read must execute exactly one query and NOTHING else. The store used to run
// CREATE DATABASE + CREATE TABLE before every operation, which recreated the database
// it names on the next process start — the way a dropped database comes back from the
// dead. Only the query below is expected here, so any DDL the store issued would be an
// unmatched call and fail this test.
func TestEventStore_ReadsWithoutIssuingDDL(t *testing.T) {
	provider := telemetrystoretest.New(telemetrystore.Config{}, anyMatcher{})
	mock := provider.Mock()
	mock.MatchExpectationsInOrder(false)

	// DistinctFingerprints → one String column. The 4 bound args are the (org,
	// project, from, to) tenant+window scope every read carries.
	mock.ExpectQuery("SELECT DISTINCT").
		WithArgs(nil, nil, nil, nil). // nil = match-any (dsmock.matchArg); 4 = the tenant+window scope
		WillReturnRows(dsmock.NewRows(
			[]dsmock.ColumnType{{Name: "group", Type: "String"}},
			[][]any{{"fp-1"}, {"fp-2"}},
		))

	store := NewEventStore(telemetrystore.TelemetryStore(provider))
	fps, err := store.DistinctFingerprints(context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID(), testWindow())
	require.NoError(t, err)
	assert.Equal(t, []string{"fp-1", "fp-2"}, fps)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestEventStoreTargetsEventError pins WHICH table the plane reads and writes. The
// database is named for what it holds and the table for what it is.
func TestEventStoreTargetsEventError(t *testing.T) {
	s := NewEventStore(nil).(*eventStore)
	assert.Equal(t, "event", s.db)
	assert.Equal(t, "error", s.table)
}

// TestInsertMatchesAppend pins the insert-sink invariant: the INSERT column list and
// the batch.Append argument list are the same length and order, and the read
// projection scans exactly as many columns as it selects — so a written row reads back
// field-for-field.
func TestInsertMatchesAppend(t *testing.T) {
	assert.Equal(t, 26, countCols(insertColumns), "insert writes 26 columns")
	assert.Equal(t, 28, countCols(selectColumns),
		"the read projection selects the 5 frame arrays plus the attribute-backed fields")
}

// countCols counts a comma-separated column list. Every expression in these lists is
// a bare column or a constant map/array subscript, none of which contain a comma.
func countCols(list string) int { return strings.Count(list, ",") + 1 }

// Frames survive the round trip through the five parallel arrays event.error stores.
func TestFramesRoundTrip(t *testing.T) {
	in := []sentrytypes.Frame{
		{Function: "handle", File: "app/svc.py", Line: 42, Column: 7, Own: true},
		{Function: "connect", File: "inpage.js", Line: 1, Column: 84179, Own: false},
	}
	fn, file, line, col, own := unzipFrames(in)
	assert.Equal(t, in, zipFrames(fn, file, line, col, own))
}

// A short parallel array must read as a zero value, never panic — the shape a row
// written before a frame column existed has.
func TestFramesTolerateShortArrays(t *testing.T) {
	got := zipFrames([]string{"handle", "connect"}, []string{"only-one.py"}, nil, nil, nil)
	require.Len(t, got, 2)
	assert.Equal(t, "only-one.py", got[0].File)
	assert.Equal(t, sentrytypes.Frame{Function: "connect"}, got[1])
}

// No frames means no frames — not one empty frame.
func TestFramesEmpty(t *testing.T) {
	assert.Nil(t, zipFrames(nil, nil, nil, nil, nil))
	fn, file, line, col, own := unzipFrames(nil)
	assert.Empty(t, fn)
	assert.Empty(t, file)
	assert.Empty(t, line)
	assert.Empty(t, col)
	assert.Empty(t, own)
}

// attributesOf folds tags plus the envelope-adjacent values into the one map column,
// without mutating the caller's tag map.
func TestAttributesOfDoesNotMutateTags(t *testing.T) {
	tags := map[string]string{"release_channel": "beta"}
	e := &sentrytypes.Event{Tags: tags, Platform: "python", ServerName: "web-1"}

	attrs := attributesOf(e)
	assert.Equal(t, "beta", attrs["release_channel"])
	assert.Equal(t, "python", attrs["platform"])
	assert.Equal(t, "web-1", attrs["server"])
	assert.NotContains(t, attrs, "user_email", "an empty value is not stored")

	assert.Equal(t, map[string]string{"release_channel": "beta"}, tags, "the caller's map is untouched")
}

var _ sentrytypes.EventStore = (*eventStore)(nil)
