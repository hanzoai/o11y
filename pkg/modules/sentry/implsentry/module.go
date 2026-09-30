package implsentry

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/identn/iamidentn"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// nowUTC is the ONE package time source for lifecycle writes (overridable in tests).
var nowUTC = func() time.Time { return time.Now().UTC() }

type module struct {
	projects sentrytypes.ProjectStore
	events   sentrytypes.EventStore
	issues   errortracking.Module // reused grouped-issue lifecycle (o11y_issues)

	// known memoizes (org, product) pairs whose project exists, so a steady stream of
	// errors from one surface costs no project read.
	known sync.Map
}

// NewModule composes the Sentinel face over the projects store, the error facts of
// the event plane and the reused issue lifecycle. Trace/event/issue reads are all
// org+project scoped over event.fact; the event.span plane is not read here (see
// TraceDetail).
func NewModule(projects sentrytypes.ProjectStore, events sentrytypes.EventStore, issues errortracking.Module) sentry.Module {
	return &module{projects: projects, events: events, issues: issues}
}

// --- ingest ---

// Ingest files a batch of error facts from ONE (org, product) into the org's issues
// and returns the issues the batch created. The facts already sit on the event plane,
// written by the ingest that accepted them, so nothing here copies them; this owns the
// lifecycle only. The product's project is born on first sight with slug = product —
// the name the plane stores (plane.go) — so a surface shows up the moment it fails.
// A fact that names no product still files its issue; it has no project to list.
func (m *module) Ingest(ctx context.Context, org, product string, occs []*errortrackingtypes.Occurrence) ([]*errortrackingtypes.Issue, error) {
	if org == "" {
		return nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "ingest has no org")
	}
	orgID := iamidentn.OrgUUID(org)
	m.ensureProject(ctx, orgID, product)
	return m.issues.Ingest(ctx, orgID, occs)
}

// ensureProject provisions the product's project under the org, idempotently and
// fail-soft: a create race or a store miss must never fail the lifecycle write (the
// org is the authoritative scope; a missing project row only hides the surface from
// the project selector until the next error re-attempts it).
func (m *module) ensureProject(ctx context.Context, orgID valuer.UUID, product string) {
	slug := slugify(product)
	if orgID.IsZero() || slug == "" || reservedSlugs[slug] {
		return
	}
	key := orgID.String() + "/" + slug
	if _, ok := m.known.Load(key); ok {
		return
	}
	ps, err := m.projects.List(ctx, orgID)
	if err != nil {
		return
	}
	for _, p := range ps {
		if p.Slug == slug {
			m.known.Store(key, struct{}{})
			return
		}
	}
	now := nowUTC()
	if err := m.projects.Create(ctx, &sentrytypes.Project{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		Name:          product,
		Slug:          slug,
		Status:        sentrytypes.ProjectActive,
	}); err == nil {
		m.known.Store(key, struct{}{})
	}
}

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

// DeleteProject removes the project, org-scoped. Retained events are untouched by
// design: deleting a project must not double as a history wipe.
func (m *module) DeleteProject(ctx context.Context, orgID, id valuer.UUID) error {
	return m.projects.Delete(ctx, orgID, id)
}

// gettable returns the API view of a project.
func (m *module) gettable(p *sentrytypes.Project) *sentrytypes.GettableProject {
	return &sentrytypes.GettableProject{Project: p}
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
// (org, project): the org-scoped GetIssue resolves the fingerprint, then the events
// read binds BOTH org and project — a project is an isolation unit, so occurrences are
// never read across projects.
func (m *module) IssueEvents(ctx context.Context, orgID, id, projectID valuer.UUID, limit int) ([]*sentrytypes.Event, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	issue, err := m.issues.GetIssue(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return m.events.ListForFingerprint(ctx, orgID, projectID, issue.Issue.Fingerprint, limit)
}

// --- discover / events / logs / traces / stats (events plane) ---

func (m *module) Discover(ctx context.Context, orgID valuer.UUID, req *sentrytypes.DiscoverRequest) (*sentrytypes.DiscoverResult, error) {
	projectID, err := m.requireProject(ctx, orgID, req.Project)
	if err != nil {
		return nil, err
	}
	return m.events.Discover(ctx, orgID, projectID, req, resolveWindow(req.Period, nowUTC()))
}

func (m *module) GetEvent(ctx context.Context, orgID, projectID valuer.UUID, eventID string) (*sentrytypes.Event, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	return m.events.GetEvent(ctx, orgID, projectID, eventID)
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

// TraceDetail returns the (org, project)-scoped error events referencing a trace —
// the tenant-safe "errors in this trace" view. The load-bearing scope is on the READ
// itself: the events query binds org AND product, so a caller only ever sees their own
// project's events for a trace, never another tenant's.
//
// This returns the errors on the trace, not the full span waterfall. The reason it
// used to be impossible is gone — event.span carries org as its first column, so spans
// ARE tenant-scopable now — but joining the waterfall in is its own cut: the two planes
// have to agree on which identifier space org is in (event.span is keyed by org slug,
// this face by the org UUID) before a join can be trusted.
func (m *module) TraceDetail(ctx context.Context, orgID, projectID valuer.UUID, traceID string) (any, error) {
	if _, err := m.projects.Get(ctx, orgID, projectID); err != nil {
		return nil, err
	}
	events, err := m.events.ListForTrace(ctx, orgID, projectID, traceID, 0)
	if err != nil {
		return nil, err
	}
	return map[string]any{"traceId": traceID, "events": events}, nil
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

// reservedSlugs are the static /v1/o11y/sentinel resource words a project slug may not
// take, so a slug can never be confused with a route.
var reservedSlugs = map[string]bool{
	"projects": true, "issues": true, "discover": true, "events": true,
	"logs": true, "traces": true, "stats": true,
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
