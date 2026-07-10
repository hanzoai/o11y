package errortrackingtypes

import (
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

var (
	ErrCodeErrorTrackingInvalidInput = errors.MustNewCode("errortracking_invalid_input")
	ErrCodeErrorTrackingNotFound     = errors.MustNewCode("errortracking_not_found")
	ErrCodeErrorTrackingUnauthorized = errors.MustNewCode("errortracking_unauthorized")
	ErrCodeErrorTrackingDisabled     = errors.MustNewCode("errortracking_disabled")
	ErrCodeErrorTrackingConflict     = errors.MustNewCode("errortracking_conflict")
)

// IssueStatus is the lifecycle state of an issue (Sentry-class).
type IssueStatus string

const (
	StatusUnresolved IssueStatus = "unresolved"
	StatusResolved   IssueStatus = "resolved"
	StatusIgnored    IssueStatus = "ignored"
)

// Valid reports whether s is one of the three known lifecycle states.
func (s IssueStatus) Valid() bool {
	switch s {
	case StatusUnresolved, StatusResolved, StatusIgnored:
		return true
	}
	return false
}

// Default severity level for an issue when the SDK sends none.
const DefaultLevel = "error"

// Issue is the grouped error — a fingerprint bucket. It is the ONE net-new table
// backing error tracking. Occurrences live in the telemetry store (o11y_traces /
// o11y_logs); only the lifecycle state that CANNOT be derived from telemetry —
// status, assignee, first/last-seen, running count, regression — lives here.
// Grouping is done at INGEST (the shim computes the fingerprint), so the Issues
// list is a plain org-scoped SELECT, never an unscoped scan over an org-less
// exception table.
//
// Tenancy: OrgID is the mandatory boundary. It is the o11y org UUID — identical
// to the claims.OrgID the read path derives from the gateway-asserted X-Org-Id,
// and to the UUID the ingest path derives from the DSN project via the SAME
// UUIDv5 mapping — so a row written by ingest is found by exactly one tenant's
// read. Every store query filters `org_id = ?`; there is no code path that reads
// issues across orgs.
type Issue struct {
	bun.BaseModel `bun:"table:o11y_issues,alias:o11y_issues" json:"-"`

	types.Identifiable
	types.TimeAuditable

	OrgID       valuer.UUID `bun:"org_id,type:text,notnull" json:"-"`
	Fingerprint string      `bun:"fingerprint,type:text,notnull" json:"fingerprint"`
	Type        string      `bun:"type,type:text,notnull" json:"type"`
	Value       string      `bun:"value,type:text" json:"value"`
	Culprit     string      `bun:"culprit,type:text" json:"culprit"`
	Level       string      `bun:"level,type:text,notnull,default:'error'" json:"level"`
	Platform    string      `bun:"platform,type:text" json:"platform,omitempty"`
	Status      IssueStatus `bun:"status,type:text,notnull,default:'unresolved'" json:"status"`
	Assignee    string      `bun:"assignee,type:text" json:"assignee,omitempty"`
	FirstSeen   time.Time   `bun:"first_seen,notnull" json:"firstSeen"`
	LastSeen    time.Time   `bun:"last_seen,notnull" json:"lastSeen"`
	Count       int64       `bun:"count,notnull,default:0" json:"count"`
	ResolvedAt  *time.Time  `bun:"resolved_at" json:"resolvedAt,omitempty"`
	Regressed   bool        `bun:"regressed,notnull,default:false" json:"regressed"`
	Environment string      `bun:"environment,type:text" json:"environment,omitempty"`
	Release     string      `bun:"release,type:text" json:"release,omitempty"`
	ServiceName string      `bun:"service_name,type:text" json:"serviceName,omitempty"`

	// Version is the optimistic-concurrency guard for lifecycle updates: bumped only
	// by UpdateIssue, never by ingest, so an operator's resolve/ignore cannot clobber
	// a concurrent operator's write (last-writer-wins) — a stale version is a conflict.
	Version int64 `bun:"version,type:bigint,notnull,default:0" json:"-"`

	// SampleEvent is the latest normalized Occurrence, stored as JSON so the issue
	// detail is fully viewable straight from SQL — no dependency on the (fast-follow)
	// occurrence-in-logs read path. Never serialized on the list; parsed into
	// GettableIssue.LatestEvent on detail.
	SampleEvent string `bun:"sample_event,type:text" json:"-"`
}

// IssuesQuery is the filter for GET /v1/o11y/errortracking/issues. It carries NO
// org field on purpose — the tenant is passed as a separate, server-validated
// argument to the store (mirroring llmobs ScoresQuery), so no client query param
// can widen the scope.
type IssuesQuery struct {
	Status      string `query:"status" json:"status"`
	Level       string `query:"level" json:"level"`
	Environment string `query:"environment" json:"environment"`
	ServiceName string `query:"serviceName" json:"serviceName"`
	Query       string `query:"query" json:"query"`
	Sort        string `query:"sort" json:"sort"`
	Offset      int    `query:"offset" json:"offset"`
	Limit       int    `query:"limit" json:"limit"`
}

type GettableIssues struct {
	Items  []*Issue `json:"items" required:"true"`
	Total  int      `json:"total" required:"true"`
	Offset int      `json:"offset" required:"true"`
	Limit  int      `json:"limit" required:"true"`
}

// GettableIssue is the issue detail: the lifecycle row plus its latest occurrence
// (parsed from SampleEvent) so the drill-down renders without the occurrence store.
type GettableIssue struct {
	Issue       *Issue      `json:"issue" required:"true"`
	LatestEvent *Occurrence `json:"latestEvent,omitempty"`
}

// UpdateIssue is the PATCH body to change lifecycle state (resolve / ignore /
// reopen / assign). Nil fields are left unchanged.
type UpdateIssue struct {
	Status   *string `json:"status,omitempty"`
	Assignee *string `json:"assignee,omitempty"`
}

// Validate enforces the minimal invariants of a lifecycle update.
func (u *UpdateIssue) Validate() error {
	if u == nil {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeErrorTrackingInvalidInput, "update payload is null")
	}
	if u.Status != nil && !IssueStatus(*u.Status).Valid() {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeErrorTrackingInvalidInput, "invalid status %q (want unresolved|resolved|ignored)", *u.Status)
	}
	return nil
}
