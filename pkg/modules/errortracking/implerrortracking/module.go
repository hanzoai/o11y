package implerrortracking

import (
	"context"
	"encoding/json"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type module struct {
	store errortrackingtypes.Store
	sink  OccurrenceSink
}

// NewModule wires the issue store and the (default no-op) occurrence sink into the
// error-tracking module.
func NewModule(store errortrackingtypes.Store, sink OccurrenceSink) errortracking.Module {
	if sink == nil {
		sink = NoopSink{}
	}
	return &module{store: store, sink: sink}
}

// Ingest groups one occurrence into its issue for the given org and persists the
// occurrence to the reused telemetry store. The issue upsert is authoritative; the
// sink is fail-soft (it owns its own error handling) so telemetry-store trouble can
// never drop error capture.
func (m *module) Ingest(ctx context.Context, orgID valuer.UUID, occ *errortrackingtypes.Occurrence) error {
	if occ == nil || occ.Fingerprint == "" {
		return errors.Newf(errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "occurrence has no fingerprint")
	}
	if orgID.IsZero() {
		return errors.Newf(errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "occurrence has no org")
	}

	if err := m.store.UpsertIssue(ctx, issueFromOccurrence(orgID, occ)); err != nil {
		return err
	}
	_ = m.sink.Write(ctx, orgID, occ)
	return nil
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

// UpdateIssue applies a lifecycle transition (resolve/ignore/reopen/assign),
// scoped to the caller's org, deriving resolved-at and clearing the regression
// flag consistently with the new status.
func (m *module) UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	issue, err := m.store.GetIssue(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

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

	if err := m.store.UpdateIssue(ctx, issue); err != nil {
		return nil, err
	}
	return issue, nil
}

// issueFromOccurrence builds the fresh-insert issue row. On an upsert conflict the
// store keeps first-seen/status/id and only advances count/last-seen/sample, so
// the fields here that describe "first sight" are used solely on first insert.
func issueFromOccurrence(orgID valuer.UUID, occ *errortrackingtypes.Occurrence) *errortrackingtypes.Issue {
	now := nowUTC()
	ts := occ.Timestamp
	if ts.IsZero() {
		ts = now
	}
	sample, _ := json.Marshal(occ)

	return &errortrackingtypes.Issue{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		Fingerprint:   occ.Fingerprint,
		Type:          firstNonEmpty(occ.Type, "Error"),
		Value:         occ.Value,
		Culprit:       occ.Culprit,
		Level:         firstNonEmpty(occ.Level, errortrackingtypes.DefaultLevel),
		Platform:      occ.Platform,
		Status:        errortrackingtypes.StatusUnresolved,
		FirstSeen:     ts,
		LastSeen:      ts,
		Count:         1,
		Environment:   occ.Environment,
		Release:       occ.Release,
		ServiceName:   occ.ServiceName,
		SampleEvent:   string(sample),
	}
}
