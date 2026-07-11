package implsentry

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// nowUTC is the ONE package time source for lifecycle writes (overridable in tests).
var nowUTC = func() time.Time { return time.Now().UTC() }

// Config wires the sentry module's non-store dependencies.
type Config struct {
	// IngestSecret is the KMS-sourced platform DSN-signing secret (the SAME secret the
	// errortracking ingest path verifies against). Empty => ingest fails closed.
	IngestSecret []byte
	// Host is the DSN endpoint origin (e.g. "api.hanzo.ai"); the minted DSN points at
	// https://<key>@<host>/v1/sentry/<project>.
	Host string
	// CapturePII retains end-user PII (email/ip) when true; default false = scrub.
	CapturePII bool
}

type module struct {
	projects    sentrytypes.ProjectStore
	events      sentrytypes.EventStore
	issues      errortracking.Module // reused grouped-issue lifecycle (o11y_issues)
	traceDetail tracedetail.Module   // reused o11y_traces waterfall read
	limiter     *implerrortracking.RateLimiter
	cfg         Config
}

// NewModule composes the sentry product face over the reused engine, the projects
// store, the columnar events plane and the reused issue/trace read paths.
func NewModule(projects sentrytypes.ProjectStore, events sentrytypes.EventStore, issues errortracking.Module, traceDetail tracedetail.Module, cfg Config) sentry.Module {
	return &module{
		projects:    projects,
		events:      events,
		issues:      issues,
		traceDetail: traceDetail,
		limiter:     implerrortracking.NewRateLimiter(implerrortracking.IngestRatePerSec, implerrortracking.IngestBurst),
		cfg:         cfg,
	}
}

// --- ingest ---

// Ingest persists a request's occurrences for (org, project): the columnar events
// plane (queryable, high-volume) AND the grouped-issue lifecycle (reused verbatim).
// The events write is fail-soft — a datastore hiccup must not drop the durable issue
// upsert — so its error is logged-and-swallowed here (the issue path is the source of
// truth for the Issues UI).
func (m *module) Ingest(ctx context.Context, orgID, projectID valuer.UUID, occs []*errortrackingtypes.Occurrence) error {
	if orgID.IsZero() || projectID.IsZero() {
		return errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "ingest has no org/project")
	}
	events := make([]*sentrytypes.Event, 0, len(occs))
	for _, occ := range occs {
		if occ == nil || occ.Fingerprint == "" {
			continue
		}
		events = append(events, eventFromOccurrence(orgID, projectID, occ))
	}
	// Events plane (fail-soft) FIRST, then the durable issue lifecycle (authoritative).
	_ = m.events.Insert(ctx, orgID, projectID, events)
	if _, err := m.issues.Ingest(ctx, orgID, occs); err != nil {
		return err
	}
	return nil
}

// ResolveIngest maps a DSN project id to its owning org, verifying the presented key
// against the project's rotation watermark. Fail-closed at every step: unknown or
// disabled project, or a key below the watermark, returns ok=false.
func (m *module) ResolveIngest(ctx context.Context, projectID valuer.UUID, presentedKey string) (valuer.UUID, bool) {
	if len(m.cfg.IngestSecret) == 0 {
		return valuer.UUID{}, false
	}
	orgID, keyVersion, status, found, err := m.projects.Resolve(ctx, projectID)
	if err != nil || !found || status != sentrytypes.ProjectActive {
		return valuer.UUID{}, false
	}
	if !implerrortracking.VerifyKey(m.cfg.IngestSecret, projectID.String(), presentedKey, keyVersion) {
		return valuer.UUID{}, false
	}
	return orgID, true
}

func (m *module) RateAllow(projectID valuer.UUID) bool { return m.limiter.Allow(projectID) }

// --- projects ---

func (m *module) CreateProject(ctx context.Context, orgID valuer.UUID, in *sentrytypes.PostableProject) (*sentrytypes.GettableProject, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "project name is required")
	}
	slug := slugify(in.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	if reservedSlugs[slug] {
		return nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "project slug %q is reserved", slug)
	}
	now := nowUTC()
	p := &sentrytypes.Project{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		Name:          name,
		Slug:          slug,
		Platform:      strings.TrimSpace(in.Platform),
		Status:        sentrytypes.ProjectActive,
		KeyVersion:    1,
	}
	if err := m.projects.Create(ctx, p); err != nil {
		return nil, err
	}
	return m.gettable(p), nil
}

func (m *module) ListProjects(ctx context.Context, orgID valuer.UUID) (*sentrytypes.GettableProjects, error) {
	ps, err := m.projects.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := make([]*sentrytypes.GettableProject, 0, len(ps))
	for _, p := range ps {
		items = append(items, m.gettable(p))
	}
	return &sentrytypes.GettableProjects{Items: items, Total: len(items)}, nil
}

func (m *module) GetProject(ctx context.Context, orgID, id valuer.UUID) (*sentrytypes.GettableProject, error) {
	p, err := m.projects.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return m.gettable(p), nil
}

func (m *module) RotateProjectKey(ctx context.Context, orgID, id valuer.UUID) (*sentrytypes.GettableProject, error) {
	version, err := m.projects.Rotate(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	p, err := m.projects.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	p.KeyVersion = version
	return m.gettable(p), nil
}

// gettable derives the project's current DSN (never stored) and returns the API view.
func (m *module) gettable(p *sentrytypes.Project) *sentrytypes.GettableProject {
	dsn := ""
	if len(m.cfg.IngestSecret) > 0 {
		dsn = mintDSN(m.cfg.IngestSecret, m.cfg.Host, p.ID, p.KeyVersion)
	}
	return &sentrytypes.GettableProject{Project: p, DSN: dsn}
}

// --- issues (reused errortracking lifecycle, org-scoped) ---

// ListIssues returns the org's grouped issues, optionally narrowed to a project. The
// project narrowing is the events-plane projection: an issue (fingerprint) belongs to
// a project iff it has captured events there in the window. The fingerprint set is
// server-derived and passed as a server-only filter, so no client can widen scope.
func (m *module) ListIssues(ctx context.Context, orgID valuer.UUID, projectID *valuer.UUID, q *errortrackingtypes.IssuesQuery, w sentrytypes.Window) (*errortrackingtypes.GettableIssues, error) {
	if projectID != nil {
		// Validate the project belongs to the caller's org (foreign id => not found).
		if _, err := m.projects.Get(ctx, orgID, *projectID); err != nil {
			return nil, err
		}
		fps, err := m.events.DistinctFingerprints(ctx, orgID, *projectID, w)
		if err != nil {
			return nil, err
		}
		if len(fps) == 0 {
			// A project with no captured errors has no issues — do not run an unfiltered
			// (whole-org) list.
			return &errortrackingtypes.GettableIssues{Items: []*errortrackingtypes.Issue{}, Total: 0, Offset: 0, Limit: q.Limit}, nil
		}
		q.Fingerprints = fps
	}
	items, total, err := m.issues.ListIssues(ctx, orgID, q)
	if err != nil {
		return nil, err
	}
	return &errortrackingtypes.GettableIssues{Items: items, Total: total, Offset: q.Offset, Limit: q.Limit}, nil
}

func (m *module) GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.GettableIssue, error) {
	return m.issues.GetIssue(ctx, orgID, id)
}

func (m *module) UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error) {
	return m.issues.UpdateIssue(ctx, orgID, id, in)
}

// IssueEvents returns an issue's recent occurrences from the events plane, scoped to
// the caller's org via the issue's own fingerprint (an org-scoped GetIssue first).
func (m *module) IssueEvents(ctx context.Context, orgID, id valuer.UUID, limit int) ([]*sentrytypes.Event, error) {
	issue, err := m.issues.GetIssue(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return m.events.ListForFingerprint(ctx, orgID, issue.Issue.Fingerprint, limit)
}

// --- discover / events / logs / traces / stats (events plane) ---

func (m *module) Discover(ctx context.Context, orgID valuer.UUID, req *sentrytypes.DiscoverRequest) (*sentrytypes.DiscoverResult, error) {
	projectID, err := m.requireProject(ctx, orgID, req.Project)
	if err != nil {
		return nil, err
	}
	return m.events.Discover(ctx, orgID, projectID, req, resolveWindow(req.Period, nowUTC()))
}

func (m *module) GetEvent(ctx context.Context, orgID valuer.UUID, eventID string) (*sentrytypes.Event, error) {
	return m.events.GetEvent(ctx, orgID, eventID)
}

func (m *module) ListLogs(ctx context.Context, orgID, projectID valuer.UUID, query, period string, limit int) ([]*sentrytypes.Event, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	return m.events.ListLogs(ctx, orgID, projectID, query, resolveWindow(period, nowUTC()), limit)
}

func (m *module) ListTraces(ctx context.Context, orgID, projectID valuer.UUID, period string, limit int) ([]*sentrytypes.TraceSummary, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	return m.events.ListTraces(ctx, orgID, projectID, resolveWindow(period, nowUTC()), limit)
}

// TraceDetail returns the full o11y_traces waterfall for a trace — but ONLY after the
// events plane confirms the trace produced a captured error for (org, project). That
// tenant gate is what makes reusing the org-agnostic tracedetail read safe: a foreign
// trace id fails the gate and never reaches the waterfall query.
func (m *module) TraceDetail(ctx context.Context, orgID, projectID valuer.UUID, traceID string) (any, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	ok, err := m.events.TraceBelongsToProject(ctx, orgID, projectID, traceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Newf(errors.TypeNotFound, sentrytypes.ErrCodeSentryNotFound, "trace %s not found for the project", traceID)
	}
	return m.traceDetail.GetWaterfallV4(ctx, traceID, "", nil)
}

func (m *module) Stats(ctx context.Context, orgID, projectID valuer.UUID, field, period string) ([]sentrytypes.StatsPoint, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	return m.events.Stats(ctx, orgID, projectID, field, resolveWindow(period, nowUTC()))
}

// requireProject parses + org-validates a project param, returning a clear error when
// it is missing or foreign (the tenant boundary for every project-scoped read).
func (m *module) requireProject(ctx context.Context, orgID valuer.UUID, raw string) (valuer.UUID, error) {
	id, err := valuer.NewUUID(strings.TrimSpace(raw))
	if err != nil {
		return valuer.UUID{}, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "a valid project is required")
	}
	if _, err := m.projects.Get(ctx, orgID, id); err != nil {
		return valuer.UUID{}, err
	}
	return id, nil
}

// eventFromOccurrence maps a normalized Occurrence to a columnar events-plane row for
// (org, project). The full occurrence is retained as the sample JSON for event detail.
func eventFromOccurrence(orgID, projectID valuer.UUID, occ *errortrackingtypes.Occurrence) *sentrytypes.Event {
	sample, _ := json.Marshal(occ)
	e := &sentrytypes.Event{
		OrgID:       orgID.String(),
		ProjectID:   projectID.String(),
		EventID:     occ.EventID,
		Timestamp:   occ.Timestamp,
		Level:       occ.Level,
		Type:        occ.Type,
		Value:       occ.Value,
		Message:     firstNonEmpty(occ.Value, occ.Type),
		Culprit:     occ.Culprit,
		Fingerprint: occ.Fingerprint,
		Platform:    occ.Platform,
		Environment: occ.Environment,
		Release:     occ.Release,
		ServiceName: occ.ServiceName,
		Transaction: occ.Transaction,
		TraceID:     occ.TraceID,
		SpanID:      occ.SpanID,
		ServerName:  occ.ServerName,
		Tags:        occ.Tags,
		Sample:      string(sample),
	}
	if occ.Timestamp.IsZero() {
		e.Timestamp = nowUTC()
	}
	if occ.User != nil {
		e.UserID = occ.User.ID
		e.UserEmail = occ.User.Email
		e.UserIP = occ.User.IP
	}
	return e
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// reservedSlugs are the static /v1/sentry resource words a project slug may not take,
// so a slug can never be confused with a route (belt-and-suspenders alongside the
// UUID-constrained ingest route and static-before-wildcard registration).
var reservedSlugs = map[string]bool{
	"projects": true, "issues": true, "discover": true, "events": true,
	"logs": true, "traces": true, "stats": true, "envelope": true, "store": true,
}

// slugify lowercases and reduces a name to a URL-safe slug (a-z0-9 and single dashes).
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
