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
			[]dsmock.ColumnType{{Name: "issue", Type: "String"}},
			[][]any{{"fp-1"}, {"fp-2"}},
		))

	store := NewEventStore(telemetrystore.TelemetryStore(provider), namedScope("acme", "docs"))
	fps, err := store.DistinctFingerprints(context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID(), testWindow())
	require.NoError(t, err)
	assert.Equal(t, []string{"fp-1", "fp-2"}, fps)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestEventStoreTargetsEventFact pins WHICH table the plane reads: the one fact
// table, whose error slice every read selects by signal.
func TestEventStoreTargetsEventFact(t *testing.T) {
	s := NewEventStore(nil, namedScope("acme", "docs")).(*eventStore)
	assert.Equal(t, "event", s.db)
	assert.Equal(t, "fact", s.table)
}

// TestProjectionMatchesScan pins the read invariant: queryEvents scans exactly as many
// targets as selectColumns selects.
func TestProjectionMatchesScan(t *testing.T) {
	assert.Len(t, selectList, 28,
		"the read projection selects the 5 frame arrays plus the attribute-backed fields")
	assert.Equal(t, strings.Join(selectList, ", "), selectColumns)
}

// Frames read back from the five parallel arrays event.fact stores.
func TestFramesRead(t *testing.T) {
	want := []sentrytypes.Frame{
		{Function: "handle", File: "app/svc.py", Line: 42, Column: 7, Own: true},
		{Function: "connect", File: "inpage.js", Line: 1, Column: 84179, Own: false},
	}
	got := zipFrames([]string{"handle", "connect"}, []string{"app/svc.py", "inpage.js"},
		[]uint32{42, 1}, []uint32{7, 84179}, []bool{true, false})
	assert.Equal(t, want, got)
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
}

var _ sentrytypes.EventStore = (*eventStore)(nil)
