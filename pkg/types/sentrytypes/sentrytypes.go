// Package sentrytypes holds the value types and store seams for Hanzo Sentry — the
// Sentry-parity error/log/trace product face served under /v1/sentry. It COMPOSES
// the shared observability substrate rather than reforking it:
//
//   - Projects are the DSN-bearing unit under an IAM org (relational lifecycle).
//   - Raw error EVENTS are rows of event.error, the ONE error table on the shared
//     datastore (queried by Discover / events / stats / logs / traces).
//   - Grouped ISSUE lifecycle stays in o11y_issues (errortracking, reused verbatim).
//
// event.error carries the same 15-column envelope as event.event / event.log /
// event.span, so a Sentry field is the envelope's field wherever one exists: the org
// is the tenant, the project is the envelope's product, the event id is the id, and
// the error group is the group. Cross-signal correlation is then a UNION ALL rather
// than a translation layer.
//
// Every read is org-scoped from the validated IAM principal; the client never names
// its own tenant. A project param is always validated against the caller's org
// before it scopes a query, so a foreign project id returns zero rows, not a leak.
package sentrytypes

import (
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

var (
	ErrCodeSentryInvalidInput = errors.MustNewCode("sentry_invalid_input")
	ErrCodeSentryNotFound     = errors.MustNewCode("sentry_not_found")
	ErrCodeSentryUnauthorized = errors.MustNewCode("sentry_unauthorized")
	ErrCodeSentryDisabled     = errors.MustNewCode("sentry_disabled")
	ErrCodeSentryConflict     = errors.MustNewCode("sentry_conflict")
)

// ProjectStatus is a project's lifecycle state. A revoked/archived project fails
// ingest closed (its DSN stops verifying) without deleting its history.
type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectDisabled ProjectStatus = "disabled"
)

// Project is a DSN-bearing unit under an IAM org. It is a thin relational row: the
// DSN itself is NOT stored — it is derived on demand from the platform ingest secret
// (KMS) + the project id + KeyVersion, so rotating a project is a single-row bump
// with no secret at rest. Tenancy: OrgID is the mandatory boundary; every store
// query filters org_id.
type Project struct {
	bun.BaseModel `bun:"table:o11y_sentry_projects,alias:o11y_sentry_projects" json:"-"`

	types.Identifiable
	types.TimeAuditable

	OrgID valuer.UUID `bun:"org_id,type:text,notnull" json:"-"`

	Name     string        `bun:"name,type:text,notnull" json:"name"`
	Slug     string        `bun:"slug,type:text,notnull" json:"slug"`
	Platform string        `bun:"platform,type:text" json:"platform,omitempty"`
	Status   ProjectStatus `bun:"status,type:text,notnull,default:'active'" json:"status"`

	// KeyVersion is the per-project DSN rotation watermark. A DSN key is
	// "<version>:<hmac>"; a key whose version is below KeyVersion no longer verifies.
	// Rotation bumps this by one — isolated to THIS project, no global secret roll and
	// no shared revocation table.
	KeyVersion int `bun:"key_version,type:bigint,notnull,default:1" json:"-"`
}

// GettableProject is the API view of a project including its freshly-derived DSN.
type GettableProject struct {
	*Project
	DSN string `json:"dsn"`
}

// PostableProject creates a project. Only Name (and optional Slug/Platform) are
// client-supplied; org/id/dsn/key are server-assigned.
type PostableProject struct {
	Name     string `json:"name"`
	Slug     string `json:"slug,omitempty"`
	Platform string `json:"platform,omitempty"`
}

type GettableProjects struct {
	Items []*GettableProject `json:"items" required:"true"`
	Total int                `json:"total" required:"true"`
}

// Frame is one stack frame of an error. Frames are stored as parallel arrays on
// event.error (frames.function / .file / .line / .column / .own), so "which function
// throws most often" is a GROUP BY rather than a JSON scan.
type Frame struct {
	Function string `json:"function,omitempty"`
	File     string `json:"file,omitempty"`
	Line     uint32 `json:"line,omitempty"`
	Column   uint32 `json:"column,omitempty"`
	// Own marks a frame in the reporting application's own code, as opposed to a
	// dependency or an injected browser extension.
	Own bool `json:"own"`
}

// Event is one error occurrence — a row of event.error. It carries exactly the fields
// Discover / events / stats / logs / traces need, org+project scoped and time-bucketed.
//
// Two fields the pre-event.error shape carried are deliberately gone:
//
//   - value duplicated message on every stored row, so message is the one field.
//   - sample was an opaque JSON blob of the whole occurrence, which made stack frames
//     unqueryable. Frames replaces it with queryable columns.
type Event struct {
	OrgID       string            `json:"orgId"`
	ProjectID   string            `json:"projectId"`
	EventID     string            `json:"eventId"`
	Timestamp   time.Time         `json:"timestamp"`
	ReceivedAt  time.Time         `json:"receivedAt"`
	Level       string            `json:"level"`
	Type        string            `json:"type"`
	Message     string            `json:"message"`
	Culprit     string            `json:"culprit"`
	Fingerprint string            `json:"fingerprint"`
	Handled     bool              `json:"handled"`
	Platform    string            `json:"platform,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Release     string            `json:"release,omitempty"`
	ServiceName string            `json:"serviceName,omitempty"`
	Transaction string            `json:"transaction,omitempty"`
	TraceID     string            `json:"traceId,omitempty"`
	SpanID      string            `json:"spanId,omitempty"`
	ServerName  string            `json:"serverName,omitempty"`
	UserID      string            `json:"userId,omitempty"`
	UserEmail   string            `json:"userEmail,omitempty"`
	UserIP      string            `json:"userIp,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Frames      []Frame           `json:"frames,omitempty"`
}

// Window is a resolved absolute time range [From, To]. Every columnar read is bounded
// by a window so a query can never scan the whole retention.
type Window struct {
	From time.Time
	To   time.Time
}

// DiscoverFilter is one equality/like predicate. Field is resolved against the
// column allowlist (or a validated tags[key]) — never interpolated; Value is always
// a bound parameter.
type DiscoverFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"` // eq | neq | like
	Value string `json:"value"`
}

// DiscoverRequest is a columnar aggregation over the events plane. Project is
// mandatory and validated against the caller's org before it scopes the scan.
type DiscoverRequest struct {
	Project      string           `json:"project"`
	Filters      []DiscoverFilter `json:"filters,omitempty"`
	Aggregations []string         `json:"aggregations,omitempty"` // allowlist keys; empty => count
	GroupBy      []string         `json:"groupBy,omitempty"`      // allowlist column keys
	Period       string           `json:"period,omitempty"`       // relative window, e.g. 1h|24h|7d|14d|30d
	OrderBy      string           `json:"orderBy,omitempty"`      // a groupBy key or an aggregation key
	OrderDir     string           `json:"orderDir,omitempty"`     // asc | desc
	Limit        int              `json:"limit,omitempty"`
}

// DiscoverResult is a tabular result: named columns and value rows, in column order.
type DiscoverResult struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// StatsPoint is one bucket of an event-rate timeseries.
type StatsPoint struct {
	Time  time.Time `json:"time"`
	Value uint64    `json:"value"`
}

// TraceSummary is an error-correlated trace: the trace id plus the count and span of
// captured error events that referenced it, for the project. Message is the latest
// error message on the trace, so a trace list reads without a second query.
type TraceSummary struct {
	TraceID   string    `json:"traceId"`
	Count     uint64    `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	Message   string    `json:"message,omitempty"`
}
