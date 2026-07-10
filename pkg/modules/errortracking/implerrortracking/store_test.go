package implerrortracking

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlitesqlstore"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore builds a real in-memory-ish sqlite store with the o11y_issues table
// and its (org_id, fingerprint) unique index — the same shape the migration ships,
// so ON CONFLICT upserts behave exactly as in production.
func newTestStore(t *testing.T) sqlstore.SQLStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlitesqlstore.New(context.Background(), factorytest.NewSettings(), sqlstore.Config{
		Provider: "sqlite",
		Connection: sqlstore.ConnectionConfig{
			MaxOpenConns:    1,
			MaxConnLifetime: 0,
		},
		Sqlite: sqlstore.SqliteConfig{
			Path:            dbPath,
			Mode:            "wal",
			BusyTimeout:     5 * time.Second,
			TransactionMode: "deferred",
		},
	})
	require.NoError(t, err)

	_, err = store.BunDB().NewCreateTable().
		Model((*errortrackingtypes.Issue)(nil)).
		IfNotExists().
		Exec(context.Background())
	require.NoError(t, err)

	_, err = store.BunDB().Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_o11y_issues_org_fingerprint ON o11y_issues (org_id, fingerprint)`)
	require.NoError(t, err)
	return store
}

func newTestModule(t *testing.T) (errortracking.Module, valuer.UUID, valuer.UUID) {
	t.Helper()
	m := NewModule(NewStore(newTestStore(t)), NewNoopSink())
	return m, valuer.GenerateUUID(), valuer.GenerateUUID()
}

func occ(fp, typ, val string, ts time.Time) *errortrackingtypes.Occurrence {
	return &errortrackingtypes.Occurrence{
		Fingerprint: fp,
		Type:        typ,
		Value:       val,
		Level:       "error",
		Timestamp:   ts,
		EventID:     "evt-" + val,
	}
}

// TestStore_TwoOrgIsolation is the load-bearing tenancy proof: two orgs report the
// SAME fingerprint; each org sees ONLY its own issue, and neither can read the
// other's issue by id. A cross-tenant leak here is a security bug, so this test is
// the org-scope gate.
func TestStore_TwoOrgIsolation(t *testing.T) {
	ctx := context.Background()
	mod, orgA, orgB := newTestModule(t)
	now := time.Now().UTC()

	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp-shared", "TypeError", "from-A", now)))
	require.NoError(t, mod.Ingest(ctx, orgB, occ("fp-shared", "TypeError", "from-B", now)))

	aList, aTotal, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, aTotal)
	require.Len(t, aList, 1)
	assert.Equal(t, "from-A", aList[0].Value, "org A must see only its own occurrence value")
	assert.Equal(t, orgA, aList[0].OrgID)

	bList, bTotal, err := mod.ListIssues(ctx, orgB, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, bTotal)
	require.Len(t, bList, 1)
	assert.Equal(t, "from-B", bList[0].Value, "org B must see only its own occurrence value")
	assert.Equal(t, orgB, bList[0].OrgID)

	// Cross-org read by id must fail closed (not found), never return the row.
	_, err = mod.GetIssue(ctx, orgB, aList[0].ID)
	require.Error(t, err, "org B must NOT be able to read org A's issue by id")

	// And org B cannot mutate org A's issue.
	reopen := "resolved"
	_, err = mod.UpdateIssue(ctx, orgB, aList[0].ID, &errortrackingtypes.UpdateIssue{Status: &reopen})
	require.Error(t, err, "org B must NOT be able to update org A's issue")

	// Org A's issue is untouched by B's attempts.
	got, err := mod.GetIssue(ctx, orgA, aList[0].ID)
	require.NoError(t, err)
	assert.Equal(t, errortrackingtypes.StatusUnresolved, got.Issue.Status)
}

// TestStore_UpsertGroupsByFingerprint proves the fingerprint bucket: repeated
// occurrences of the same (org, fingerprint) collapse into one issue with a
// running count and advancing last-seen, while first-seen is preserved.
func TestStore_UpsertGroupsByFingerprint(t *testing.T) {
	ctx := context.Background()
	mod, orgA, _ := newTestModule(t)

	first := time.Now().UTC().Add(-time.Hour)
	later := time.Now().UTC()

	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp1", "ValueError", "boom", first)))
	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp1", "ValueError", "boom", later)))
	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp1", "ValueError", "boom", later)))

	list, total, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, total, "same fingerprint must not create new issues")
	require.Len(t, list, 1)
	assert.Equal(t, int64(3), list[0].Count, "count must track occurrences")
	assert.WithinDuration(t, first, list[0].FirstSeen, time.Second, "first-seen preserved")
	assert.WithinDuration(t, later, list[0].LastSeen, time.Second, "last-seen advanced")

	// A different fingerprint is a different issue.
	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp2", "KeyError", "missing", later)))
	_, total2, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 2, total2)
}

// TestStore_RegressionReopensResolved proves regression detection: a resolved
// issue that recurs flips back to unresolved and is flagged regressed; an ignored
// issue stays muted.
func TestStore_RegressionReopensResolved(t *testing.T) {
	ctx := context.Background()
	mod, orgA, _ := newTestModule(t)
	now := time.Now().UTC()

	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp-reg", "TypeError", "x", now)))
	list, _, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	id := list[0].ID

	resolved := string(errortrackingtypes.StatusResolved)
	updated, err := mod.UpdateIssue(ctx, orgA, id, &errortrackingtypes.UpdateIssue{Status: &resolved})
	require.NoError(t, err)
	require.Equal(t, errortrackingtypes.StatusResolved, updated.Status)
	require.NotNil(t, updated.ResolvedAt)

	// Recurrence after resolution => regression.
	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp-reg", "TypeError", "x", now.Add(time.Minute))))
	got, err := mod.GetIssue(ctx, orgA, id)
	require.NoError(t, err)
	assert.Equal(t, errortrackingtypes.StatusUnresolved, got.Issue.Status, "resolved issue must reopen on recurrence")
	assert.True(t, got.Issue.Regressed, "reopened issue must be flagged as a regression")
	assert.Equal(t, int64(2), got.Issue.Count)

	// Ignored issues stay muted on recurrence.
	ignored := string(errortrackingtypes.StatusIgnored)
	_, err = mod.UpdateIssue(ctx, orgA, id, &errortrackingtypes.UpdateIssue{Status: &ignored})
	require.NoError(t, err)
	require.NoError(t, mod.Ingest(ctx, orgA, occ("fp-reg", "TypeError", "x", now.Add(2*time.Minute))))
	got, err = mod.GetIssue(ctx, orgA, id)
	require.NoError(t, err)
	assert.Equal(t, errortrackingtypes.StatusIgnored, got.Issue.Status, "ignored issue must stay muted")
}

// TestModule_GetIssueParsesLatestEvent proves the detail view is fully served from
// SQL (the stored sample), independent of any occurrence-store read path.
func TestModule_GetIssueParsesLatestEvent(t *testing.T) {
	ctx := context.Background()
	mod, orgA, _ := newTestModule(t)
	now := time.Now().UTC()

	o := occ("fp-detail", "RuntimeError", "kaput", now)
	o.Culprit = "handler in server.go"
	require.NoError(t, mod.Ingest(ctx, orgA, o))

	list, _, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	detail, err := mod.GetIssue(ctx, orgA, list[0].ID)
	require.NoError(t, err)
	require.NotNil(t, detail.LatestEvent, "detail must carry the latest occurrence sample")
	assert.Equal(t, "kaput", detail.LatestEvent.Value)
	assert.Equal(t, "handler in server.go", detail.LatestEvent.Culprit)
}
