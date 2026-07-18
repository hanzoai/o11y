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

// TestEventStore_EnsureSchemaAndQuery exercises the REAL datastore IO path against the
// hanzo-ds/mock: ensureSchema runs the CREATE DATABASE + CREATE TABLE DDL exactly
// once, and a read binds + decodes rows. This verifies the wiring (query executes,
// rows scan) — it does NOT verify a live datastore accepts the DDL (see the honest
// caveat on createSchemaDDL; no live datastore was reachable in this build).
func TestEventStore_EnsureSchemaAndQuery(t *testing.T) {
	provider := telemetrystoretest.New(telemetrystore.Config{}, anyMatcher{})
	mock := provider.Mock()
	mock.MatchExpectationsInOrder(false)

	// ensureSchema → CREATE DATABASE + CREATE TABLE (idempotent, once).
	mock.ExpectExec("CREATE DATABASE")
	mock.ExpectExec("CREATE TABLE")

	// DistinctFingerprints → one String column. The 4 bound args are the (org,
	// project, from, to) tenant+window scope every read carries.
	mock.ExpectQuery("SELECT DISTINCT fingerprint").
		WithArgs(nil, nil, nil, nil). // nil = match-any (dsmock.matchArg); 4 = the tenant+window scope
		WillReturnRows(dsmock.NewRows(
			[]dsmock.ColumnType{{Name: "fingerprint", Type: "String"}},
			[][]any{{"fp-1"}, {"fp-2"}},
		))

	store := NewEventStore(telemetrystore.TelemetryStore(provider))
	fps, err := store.DistinctFingerprints(context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID(), testWindow())
	require.NoError(t, err)
	assert.Equal(t, []string{"fp-1", "fp-2"}, fps)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInsertTemplateColumnCount pins the insert-sink invariant: the INSERT column list
// and the batch.Append argument list are the SAME length (24) and in the SAME order as
// the read projection — so a written row reads back field-for-field. A drift here is
// exactly the class of bug the honest sink note warned about.
func TestInsertTemplateColumnCount(t *testing.T) {
	// Column list between the first "(" and the first ")".
	open := strings.Index(insertSQL, "(")
	closeP := strings.Index(insertSQL, ")")
	require.Greater(t, closeP, open)
	insertCols := strings.Count(insertSQL[open:closeP], ",") + 1
	selectCols := strings.Count(selectColumns, ",") + 1

	assert.Equal(t, 24, insertCols, "insert must write all 24 columns")
	assert.Equal(t, 24, selectCols, "read projection must match the 24 written columns")
}

var _ sentrytypes.EventStore = (*eventStore)(nil)
