package implsentry

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/identn/iamidentn"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// IngestKeyResolver maps a presented publishable key (pk-…) to the org slug it
// attributes to. It is the embedding host's job — the host dials IAM (org keys)
// and/or the projects app (per-project keys), and o11y must not import cloud — so
// the host injects it at mount via SetIngestKeyResolver. Returns ok=false for a
// key that names no org.
type IngestKeyResolver func(ctx context.Context, key string) (orgSlug string, ok bool)

var (
	ingestKeyMu       sync.RWMutex
	ingestKeyResolver IngestKeyResolver
)

// SetIngestKeyResolver installs the host's pk- → org resolver. Mirrors o11y's
// SetHandler seam exactly: set once at mount, read per request. Nil (the default)
// leaves keyed ingest fail-closed.
func SetIngestKeyResolver(fn IngestKeyResolver) {
	ingestKeyMu.Lock()
	ingestKeyResolver = fn
	ingestKeyMu.Unlock()
}

func getIngestKeyResolver() IngestKeyResolver {
	ingestKeyMu.RLock()
	defer ingestKeyMu.RUnlock()
	return ingestKeyResolver
}

// nowUTC is the ONE package time source for lifecycle writes (overridable in tests).
var nowUTC = func() time.Time { return time.Now().UTC() }

// Config wires the sentry module's non-store dependencies.
type Config struct {
	// IngestSecret is the KMS-sourced platform DSN-signing secret (the SAME secret the
	// errortracking ingest path verifies against). Empty => ingest fails closed.
	IngestSecret []byte
	// Host is the DSN endpoint origin (e.g. "api.hanzo.ai"); the minted DSN points at
	// https://<key>@<host>/v1/event/<project>.
	Host string
	// CapturePII retains end-user PII (email/ip) when true; default false = scrub.
	CapturePII bool
}

type module struct {
	projects sentrytypes.ProjectStore
	events   sentrytypes.EventStore
	issues   errortracking.Module // reused grouped-issue lifecycle (o11y_issues)
	limiter  *implerrortracking.RateLimiter
	cfg      Config
}

// NewModule composes the sentry product face over the reused engine, the projects
// store, the event.error plane and the reused issue lifecycle. Trace/event/issue reads
// are all org+project scoped over event.error; the event.span plane is not read here
// (see TraceDetail).
func NewModule(projects sentrytypes.ProjectStore, events sentrytypes.EventStore, issues errortracking.Module, cfg Config) sentry.Module {
	return &module{
		projects: projects,
		events:   events,
		issues:   issues,
		limiter:  implerrortracking.NewRateLimiter(implerrortracking.IngestRatePerSec, implerrortracking.IngestBurst),
		cfg:      cfg,
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

// ResolveIngest attributes a keyed ingest to its org. ONE auth path: the presented
// publishable key (pk-…) — the SAME key /v1/event accepts — resolves to an org via
// the host-injected resolver, and the project named in the DSN is born under that
// org on first sight (the zero-onboarding move o11y already makes for a tenant). So
// a surface declares nothing: the org key that already feeds analytics + insights
// feeds errors too. Fail-closed — no resolver, or a key that names no org, returns
// ok=false.
//
// The prior per-project DSN watermark is GONE: it was a second attribution model
// braided beside the org key (its own secret, its own per-project keys, provisioned
// out of band), and it is exactly what left this plane dark. One key, not two.
func (m *module) ResolveIngest(ctx context.Context, projectID valuer.UUID, presentedKey string) (valuer.UUID, bool) {
	resolve := getIngestKeyResolver()
	if resolve == nil {
		return valuer.UUID{}, false
	}
	orgSlug, ok := resolve(ctx, presentedKey)
	if !ok || orgSlug == "" {
		return valuer.UUID{}, false
	}
	orgID := iamidentn.OrgUUID(orgSlug)
	// A project that already exists and has been disabled stops accepting ingest,
	// which is what disabling one is for and what ProjectStatus documents. Only an
	// existing row can say this: a project unseen until now is provisioned active
	// just below, and a store that cannot answer must not silently open the door.
	if _, _, status, found, err := m.projects.Resolve(ctx, projectID); err == nil && found &&
		status != sentrytypes.ProjectActive {
		return valuer.UUID{}, false
	}
	m.ensureProject(ctx, orgID, projectID)
	return orgID, true
}

// ensureProject provisions the DSN's project under the org on first keyed ingest,
// so a surface's errors land under a stable project without anyone creating one —
// the same auto-provision o11y does for a tenant on first session. Idempotent and
// fail-soft: a create race or a store miss must never fail the ingest (the org is
// the authoritative scope; a missing project row only affects Issues-UI grouping,
// which the next occurrence re-attempts).
func (m *module) ensureProject(ctx context.Context, orgID, projectID valuer.UUID) {
	if orgID.IsZero() || projectID.IsZero() {
		return
	}
	if _, _, _, found, _ := m.projects.Resolve(ctx, projectID); found {
		return
	}
	short := projectID.String()
	if len(short) > 8 {
		short = short[:8]
	}
	now := nowUTC()
	_ = m.projects.Create(ctx, &sentrytypes.Project{
		Identifiable:  types.Identifiable{ID: projectID},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		Name:          short,
		Slug:          "p-" + short,
		Status:        sentrytypes.ProjectActive,
		KeyVersion:    1,
	})
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

// DeleteProject removes the project, org-scoped. Once gone, ResolveIngest can no
// longer map its id to a tenant, so its DSN fails closed exactly like an unknown
// one — no separate revocation step. Retained events are untouched by design.
func (m *module) DeleteProject(ctx context.Context, orgID, id valuer.UUID) error {
	return m.projects.Delete(ctx, orgID, id)
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

// eventFromOccurrence maps a normalized Occurrence to an event.error row for (org,
// project). Stack frames become the five parallel frames.* arrays, so the crash site
// is queryable rather than buried in a JSON blob.
func eventFromOccurrence(orgID, projectID valuer.UUID, occ *errortrackingtypes.Occurrence) *sentrytypes.Event {
	e := &sentrytypes.Event{
		OrgID:       orgID.String(),
		ProjectID:   projectID.String(),
		EventID:     occ.EventID,
		Timestamp:   occ.Timestamp,
		Level:       occ.Level,
		Type:        occ.Type,
		Message:     firstNonEmpty(occ.Value, occ.Type),
		Culprit:     occ.Culprit,
		Fingerprint: occ.Fingerprint,
		Handled:     handledFromTags(occ.Tags),
		Platform:    occ.Platform,
		Environment: occ.Environment,
		Release:     occ.Release,
		ServiceName: occ.ServiceName,
		Transaction: occ.Transaction,
		TraceID:     occ.TraceID,
		SpanID:      occ.SpanID,
		ServerName:  occ.ServerName,
		Tags:        occ.Tags,
		Frames:      framesFromOccurrence(occ.Frames),
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

// framesFromOccurrence narrows a normalized stack to the frame fields event.error
// stores. Filename is preferred over the absolute path: it is what a stack trace reads
// as, and it does not leak a build machine's directory layout.
func framesFromOccurrence(in []errortrackingtypes.Frame) []sentrytypes.Frame {
	if len(in) == 0 {
		return nil
	}
	out := make([]sentrytypes.Frame, len(in))
	for i, f := range in {
		out[i] = sentrytypes.Frame{
			Function: f.Function,
			File:     firstNonEmpty(f.Filename, f.AbsPath),
			Line:     clampLine(f.Lineno),
			Column:   clampLine(f.Colno),
			Own:      f.InApp,
		}
	}
	return out
}

// clampLine narrows a wire line/column to the stored unsigned column, mapping a
// negative or absent value to zero ("unknown") rather than wrapping it.
func clampLine(n int) uint32 {
	if n <= 0 {
		return 0
	}
	return uint32(n)
}

// handledFromTags reads the SDK's exception-mechanism flag, which normalization
// carries as a tag. Absent means the report arrived through the normal capture path,
// which is handled; only an explicit "false" marks a crash.
func handledFromTags(tags map[string]string) bool {
	return tags["handled"] != "false"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// reservedSlugs are the static /v1/sentinel resource words a project slug may not take,
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
