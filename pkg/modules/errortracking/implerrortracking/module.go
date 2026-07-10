package implerrortracking

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const (
	// maxIssuesPerOrg caps distinct fingerprints an org may accumulate — backpressure
	// against a fingerprint-explosion DoS. New fingerprints past the cap are dropped
	// at ingest; existing issues keep counting.
	maxIssuesPerOrg = 10000

	// retentionSweepInterval is how often the (optional) TTL sweeper runs.
	retentionSweepInterval = 6 * time.Hour
)

type module struct {
	store     errortrackingtypes.Store
	sink      OccurrenceSink
	retention time.Duration
}

// Option configures the module at construction.
type Option func(*module)

// WithRetention enables a background TTL sweep that purges issues whose last_seen
// predates the given age. Zero (the default) disables the sweeper — so tests that
// construct a module never spawn a goroutine.
func WithRetention(d time.Duration) Option {
	return func(m *module) { m.retention = d }
}

// NewModule wires the issue store and the (default no-op) occurrence sink. When a
// positive retention is configured it starts the TTL sweeper.
func NewModule(store errortrackingtypes.Store, sink OccurrenceSink, opts ...Option) errortracking.Module {
	if sink == nil {
		sink = NoopSink{}
	}
	m := &module{store: store, sink: sink}
	for _, o := range opts {
		o(m)
	}
	if m.retention > 0 {
		go m.retentionLoop(context.Background())
	}
	return m
}

// Ingest groups a whole request's occurrences into their issues. Occurrences are
// FIRST collapsed by fingerprint (a flood of identical events becomes one upsert
// with an incremented count) and then written in ONE transaction under the per-org
// ceiling — so a single request can never fan out into a transaction-per-event
// write storm. The occurrence sink is fail-soft (owns its own errors).
func (m *module) Ingest(ctx context.Context, orgID valuer.UUID, occs []*errortrackingtypes.Occurrence) (int, error) {
	if orgID.IsZero() {
		return 0, errors.Newf(errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "ingest has no org")
	}

	groups := aggregateByFingerprint(occs)
	if len(groups) == 0 {
		return 0, nil
	}

	now := nowUTC()
	issues := make([]*errortrackingtypes.Issue, 0, len(groups))
	for fp, g := range groups {
		issues = append(issues, issueFromGroup(orgID, fp, g, now))
	}

	written, err := m.store.UpsertIssues(ctx, orgID, issues, maxIssuesPerOrg)
	if err != nil {
		return 0, err
	}
	// Reused occurrence store; bounded to one write per distinct fingerprint and
	// fail-soft — the issues are already durable.
	for _, g := range groups {
		_ = m.sink.Write(ctx, orgID, g.sample)
	}
	return written, nil
}

func (m *module) ListIssues(ctx context.Context, orgID valuer.UUID, q *errortrackingtypes.IssuesQuery) ([]*errortrackingtypes.Issue, int, error) {
	return m.store.ListIssues(ctx, orgID, q)
}

func (m *module) GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.GettableIssue, error) {
	issue, err := m.store.GetIssue(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	g := &errortrackingtypes.GettableIssue{Issue: issue}
	if issue.SampleEvent != "" {
		var occ errortrackingtypes.Occurrence
		if err := json.Unmarshal([]byte(issue.SampleEvent), &occ); err == nil {
			g.LatestEvent = &occ
		}
	}
	return g, nil
}

// UpdateIssue applies a lifecycle transition scoped to the caller's org, with an
// optimistic-concurrency guard on the loaded version so a stale write conflicts
// instead of clobbering a concurrent operator's change.
func (m *module) UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	issue, err := m.store.GetIssue(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	expectedVersion := issue.Version

	now := nowUTC()
	if in.Status != nil {
		status := errortrackingtypes.IssueStatus(*in.Status)
		issue.Status = status
		switch status {
		case errortrackingtypes.StatusResolved:
			issue.ResolvedAt = &now
			issue.Regressed = false
		case errortrackingtypes.StatusUnresolved:
			issue.ResolvedAt = nil
			issue.Regressed = false
		case errortrackingtypes.StatusIgnored:
			// muted: leave resolved_at untouched
		}
	}
	if in.Assignee != nil {
		issue.Assignee = *in.Assignee
	}
	issue.UpdatedAt = now

	if err := m.store.UpdateIssue(ctx, issue, expectedVersion); err != nil {
		return nil, err
	}
	issue.Version = expectedVersion + 1
	return issue, nil
}

func (m *module) retentionLoop(ctx context.Context) {
	t := time.NewTicker(retentionSweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = m.store.DeleteStale(ctx, nowUTC().Add(-m.retention))
		}
	}
}

// occurrenceGroup is the per-fingerprint rollup of a batch.
type occurrenceGroup struct {
	count  int64
	first  time.Time
	last   time.Time
	sample *errortrackingtypes.Occurrence
}

// aggregateByFingerprint collapses a batch to one group per fingerprint, tracking
// count and the first/last timestamps, keeping the latest occurrence as the sample.
func aggregateByFingerprint(occs []*errortrackingtypes.Occurrence) map[string]*occurrenceGroup {
	groups := map[string]*occurrenceGroup{}
	for _, occ := range occs {
		if occ == nil || occ.Fingerprint == "" {
			continue
		}
		g := groups[occ.Fingerprint]
		if g == nil {
			g = &occurrenceGroup{first: occ.Timestamp, last: occ.Timestamp, sample: occ}
			groups[occ.Fingerprint] = g
		}
		g.count++
		ts := occ.Timestamp
		if ts.IsZero() {
			continue
		}
		if g.first.IsZero() || ts.Before(g.first) {
			g.first = ts
		}
		if ts.After(g.last) {
			g.last = ts
			g.sample = occ
		}
	}
	return groups
}

func issueFromGroup(orgID valuer.UUID, fingerprint string, g *occurrenceGroup, now time.Time) *errortrackingtypes.Issue {
	first := g.first
	if first.IsZero() {
		first = now
	}
	last := g.last
	if last.IsZero() {
		last = now
	}
	sample, _ := json.Marshal(g.sample)

	return &errortrackingtypes.Issue{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		Fingerprint:   fingerprint,
		Type:          firstNonEmpty(g.sample.Type, "Error"),
		Value:         g.sample.Value,
		Culprit:       g.sample.Culprit,
		Level:         firstNonEmpty(g.sample.Level, errortrackingtypes.DefaultLevel),
		Platform:      g.sample.Platform,
		Status:        errortrackingtypes.StatusUnresolved,
		FirstSeen:     first,
		LastSeen:      last,
		Count:         g.count,
		Environment:   g.sample.Environment,
		Release:       g.sample.Release,
		ServiceName:   g.sample.ServiceName,
		SampleEvent:   string(sample),
	}
}
