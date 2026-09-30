package implerrortracking

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- HIGH-1: ingest amplification is bounded ---

// A flood of identical events collapses to ONE issue with an incremented count —
// not one upsert (or transaction) per event.
func TestIngest_CollapsesDuplicateFingerprints(t *testing.T) {
	ctx := context.Background()
	mod, orgA, _ := newTestModule(t)

	occs := make([]*errortrackingtypes.Occurrence, 0, 5000)
	for i := 0; i < 5000; i++ {
		occs = append(occs, occ("fp-flood", "TypeError", "boom", time.Now().UTC()))
	}
	created, err := mod.Ingest(ctx, orgA, occs)
	require.NoError(t, err)
	assert.Len(t, created, 1, "5000 identical events must become ONE new issue, not 5000")

	list, total, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, int64(5000), list[0].Count, "count still reflects every event")
}

// The per-org issue ceiling admits only `ceiling` NEW fingerprints; existing ones
// keep bumping past the cap.
func TestStore_CeilingCapsNewFingerprints(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newTestStore(t))
	org := valuer.GenerateUUID()

	mk := func(fp string) *errortrackingtypes.Issue {
		now := time.Now().UTC()
		return &errortrackingtypes.Issue{
			Fingerprint: fp, OrgID: org, Type: "E", Level: "error", Status: errortrackingtypes.StatusUnresolved,
			FirstSeen: now, LastSeen: now, Count: 1,
		}
	}
	batch := make([]*errortrackingtypes.Issue, 0, 10)
	for i := 0; i < 10; i++ {
		iss := mk(fmt.Sprintf("fp-%d", i))
		iss.ID = valuer.GenerateUUID()
		batch = append(batch, iss)
	}

	created, err := s.UpsertIssues(ctx, org, batch, 5)
	require.NoError(t, err)
	assert.Len(t, created, 5, "only ceiling-many NEW fingerprints are admitted")

	_, total, err := s.ListIssues(ctx, org, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Re-ingesting the SAME batch: existing 5 bump (no new rows past the cap).
	for _, iss := range batch {
		iss.ID = valuer.GenerateUUID()
	}
	again, err := s.UpsertIssues(ctx, org, batch, 5)
	require.NoError(t, err)
	assert.Empty(t, again, "a second sight of an existing fingerprint creates nothing")
	_, total2, err := s.ListIssues(ctx, org, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total2, "ceiling still holds; existing issues just bumped")
}

// --- LOW-1: optimistic concurrency on lifecycle update ---

func TestStore_OptimisticUpdateConflict(t *testing.T) {
	ctx := context.Background()
	mod, orgA, _ := newTestModule(t)
	mustIngest(t, mod, ctx, orgA, occ("fp-oc", "E", "x", time.Now().UTC()))
	list, _, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	id := list[0].ID

	// Two operators load the SAME version.
	first, err := mod.GetIssue(ctx, orgA, id)
	require.NoError(t, err)
	second, err := mod.GetIssue(ctx, orgA, id)
	require.NoError(t, err)
	require.Equal(t, first.Issue.Version, second.Issue.Version)
	staleVersion := second.Issue.Version

	// First operator resolves — succeeds, bumping the row's version.
	_, err = mod.UpdateIssue(ctx, orgA, id, &errortrackingtypes.UpdateIssue{Status: strp(string(errortrackingtypes.StatusResolved))})
	require.NoError(t, err)

	// The second operator's write, carrying the STALE version, must conflict.
	stale := second.Issue
	stale.Status = errortrackingtypes.StatusIgnored
	stale.UpdatedAt = time.Now().UTC()
	err = moduleStore(mod).UpdateIssue(ctx, stale, staleVersion)
	require.Error(t, err, "a stale-version write must conflict, not clobber")
}

// --- retention/TTL ---

func TestStore_DeleteStale(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newTestStore(t))
	org := valuer.GenerateUUID()

	old := &errortrackingtypes.Issue{Identifiable: types.Identifiable{ID: valuer.GenerateUUID()}, OrgID: org, Fingerprint: "old", Type: "E", Level: "error", Status: errortrackingtypes.StatusResolved, FirstSeen: time.Now().Add(-100 * 24 * time.Hour), LastSeen: time.Now().Add(-100 * 24 * time.Hour), Count: 1}
	recent := &errortrackingtypes.Issue{Identifiable: types.Identifiable{ID: valuer.GenerateUUID()}, OrgID: org, Fingerprint: "new", Type: "E", Level: "error", Status: errortrackingtypes.StatusUnresolved, FirstSeen: time.Now(), LastSeen: time.Now(), Count: 1}
	_, err := s.UpsertIssues(ctx, org, []*errortrackingtypes.Issue{old, recent}, 100)
	require.NoError(t, err)

	n, err := s.DeleteStale(ctx, time.Now().Add(-90*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "only the stale issue is purged")

	_, total, err := s.ListIssues(ctx, org, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func strp(s string) *string { return &s }

// moduleStore reaches the concrete module's store for the concurrency test.
func moduleStore(m interface{}) errortrackingtypes.Store {
	return m.(*module).store
}
