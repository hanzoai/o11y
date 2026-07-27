package implerrortracking

import (
	"context"
	"fmt"
	"strings"
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
	written, err := mod.Ingest(ctx, orgA, occs)
	require.NoError(t, err)
	assert.Equal(t, 1, written, "5000 identical events must become ONE upsert, not 5000")

	list, total, err := mod.ListIssues(ctx, orgA, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, int64(5000), list[0].Count, "count still reflects every event")
}

// parseEnvelope refuses to extract more than the per-request cap, so one request
// cannot fan out into unbounded upserts.
func TestParseEnvelope_CapsEventCount(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"event_id":"x"}` + "\n")
	for i := 0; i < maxEventsPerEnvelope+500; i++ {
		b.WriteString(`{"type":"event"}` + "\n")
		b.WriteString(`{"exception":{"values":[{"type":"E"}]}}` + "\n")
	}
	events, err := parseEnvelope([]byte(b.String()))
	require.NoError(t, err)
	assert.Len(t, events, maxEventsPerEnvelope, "event extraction is capped")
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

	written, err := s.UpsertIssues(ctx, org, batch, 5)
	require.NoError(t, err)
	assert.Equal(t, 5, written, "only ceiling-many NEW fingerprints are admitted")

	_, total, err := s.ListIssues(ctx, org, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Re-ingesting the SAME batch: existing 5 bump (no new rows past the cap).
	for _, iss := range batch {
		iss.ID = valuer.GenerateUUID()
	}
	_, err = s.UpsertIssues(ctx, org, batch, 5)
	require.NoError(t, err)
	_, total2, err := s.ListIssues(ctx, org, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total2, "ceiling still holds; existing issues just bumped")
}

// --- MEDIUM-1: a hostile envelope item length never panics ---

func TestParseEnvelope_HugeLengthNoPanic(t *testing.T) {
	body := []byte(`{"event_id":"x"}` + "\n" +
		`{"type":"event","length":9223372036854775807}` + "\n" +
		`{"exception":{"values":[{"type":"E"}]}}` + "\n")
	assert.NotPanics(t, func() {
		_, _ = parseEnvelope(body)
	}, "a MaxInt64 length must not overflow the slice bound")
}

func TestParseEnvelope_NegativeLengthNoPanic(t *testing.T) {
	body := []byte(`{"event_id":"x"}` + "\n" +
		`{"type":"event","length":-1}` + "\n" +
		`{"exception":{"values":[{"type":"E"}]}}` + "\n")
	assert.NotPanics(t, func() {
		events, _ := parseEnvelope(body)
		// Falls back to newline-delimited framing, so the event is still read.
		require.Len(t, events, 1)
	})
}

// --- HIGH-1: per-org rate limiter ---

func TestRateLimiter_AllowsBurstThenLimits(t *testing.T) {
	l := newRateLimiter(0.0001, 3) // ~no refill within the test window
	org := valuer.GenerateUUID()
	assert.True(t, l.allow(org))
	assert.True(t, l.allow(org))
	assert.True(t, l.allow(org))
	assert.False(t, l.allow(org), "burst exhausted → limited")

	// A different org has its own bucket.
	assert.True(t, l.allow(valuer.GenerateUUID()))
}

// --- MEDIUM-2: secret redaction (always) + PII scrub (default) ---

func TestSanitize_AlwaysRedactsSecrets(t *testing.T) {
	cases := []string{
		"key sk-abcdef0123456789ABCDEF leaked",
		"aws AKIAIOSFODNN7EXAMPLE creds",
		"Authorization: Bearer abcdef123456ghijkl",
		"postgres://user:s3cr3tpw@db:5432/app",
		"card 4111 1111 1111 1111 charged",
		"hanzo hk-0123456789abcdef0123 token",
	}
	for _, c := range cases {
		// Even with PII capture ON, secrets are still removed.
		got := sanitize(c, true)
		assert.Contains(t, got, redactedMark, "secret must be redacted: %q -> %q", c, got)
	}
}

func TestScrubPII_MasksEmailAndIP_ByDefault(t *testing.T) {
	scrubbed := sanitize("user a@b.com from 10.0.0.5 failed", false)
	assert.NotContains(t, scrubbed, "a@b.com")
	assert.NotContains(t, scrubbed, "10.0.0.5")
	assert.Contains(t, scrubbed, emailMark)
	assert.Contains(t, scrubbed, ipMark)

	// With capture ON, PII is retained (but secrets still go).
	kept := sanitize("user a@b.com from 10.0.0.5 failed", true)
	assert.Contains(t, kept, "a@b.com")
	assert.Contains(t, kept, "10.0.0.5")
}

// The normalizer scrubs by default (fail-secure) — the stored value and the sample
// carry no secret/PII.
func TestNormalize_ScrubsByDefault(t *testing.T) {
	e := mustEvent(t, `{"event_id":"a","exception":{"values":[{"type":"AuthError","value":"token sk-DEADBEEFdeadbeef012345 for a@b.com"}]}}`)
	occ := normalizeEvent(e) // default opts → scrub
	assert.NotContains(t, occ.Value, "sk-DEADBEEFdeadbeef012345")
	assert.NotContains(t, occ.Value, "a@b.com")
}

// --- MEDIUM-3: versioned, per-org-revocable DSN keys ---

func TestVerifyKey_VersionedAndRevocation(t *testing.T) {
	secret := []byte("kms")
	v1 := publicKeyForVersion(secret, "acme", 1)
	v2 := publicKeyForVersion(secret, "acme", 2)
	require.True(t, strings.HasPrefix(v1, "1:"))
	require.True(t, strings.HasPrefix(v2, "2:"))

	// Below the org's min-version is rejected; at/above verifies.
	assert.True(t, verifyKey(secret, "acme", v1, 0), "v1 valid when nothing revoked")
	assert.False(t, verifyKey(secret, "acme", v1, 2), "v1 revoked once min-version is 2")
	assert.True(t, verifyKey(secret, "acme", v2, 2), "v2 still valid at min-version 2")
	// A malformed version prefix fails closed.
	assert.False(t, verifyKey(secret, "acme", "notanumber:"+v1, 0))
	assert.False(t, verifyKey(secret, "acme", "0:"+strings.TrimPrefix(v1, "1:"), 0))
}

func TestSQLRevocations_RotateIsolatesOneOrg(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	r := NewSQLRevocations(store)
	orgA := valuer.GenerateUUID()
	orgB := valuer.GenerateUUID()

	assert.Equal(t, 0, r.MinVersion(ctx, orgA), "default min-version is 0")

	require.NoError(t, r.Rotate(ctx, orgA, 2))
	assert.Equal(t, 2, r.MinVersion(ctx, orgA), "rotated org sees its new watermark")
	assert.Equal(t, 0, r.MinVersion(ctx, orgB), "other orgs are untouched (isolated rotation)")
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
