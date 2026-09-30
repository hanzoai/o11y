package implsentry

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/identn/iamidentn"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEvents is an in-memory EventStore standing in for the error facts of the event
// plane, keyed by (org, project), so a test can seed what the ingest would have
// written and assert that a read is only ever asked for the caller's own tenant.
type fakeEvents struct {
	inserts   map[[2]string][]*sentrytypes.Event
	lastOrg   string
	lastProj  string
	discovers int
}

func newFakeEvents() *fakeEvents {
	return &fakeEvents{inserts: map[[2]string][]*sentrytypes.Event{}}
}

func (f *fakeEvents) key(o, p valuer.UUID) [2]string { return [2]string{o.String(), p.String()} }

// seed puts facts on the plane the way the ingest that accepted them would have.
func (f *fakeEvents) seed(o, p valuer.UUID, occs ...*errortrackingtypes.Occurrence) {
	for _, occ := range occs {
		f.inserts[f.key(o, p)] = append(f.inserts[f.key(o, p)], &sentrytypes.Event{
			OrgID: o.String(), ProjectID: p.String(), EventID: occ.EventID,
			Fingerprint: occ.Fingerprint, TraceID: occ.TraceID, Timestamp: occ.Timestamp,
		})
	}
}
func (f *fakeEvents) Discover(_ context.Context, o, p valuer.UUID, _ *sentrytypes.DiscoverRequest, _ sentrytypes.Window) (*sentrytypes.DiscoverResult, error) {
	f.lastOrg, f.lastProj, f.discovers = o.String(), p.String(), f.discovers+1
	return &sentrytypes.DiscoverResult{}, nil
}
func (f *fakeEvents) GetEvent(_ context.Context, o, p valuer.UUID, id string) (*sentrytypes.Event, error) {
	// Tenant boundary: only THIS (org, project)'s events — never another org's or
	// another project's within the org.
	for _, e := range f.inserts[f.key(o, p)] {
		if e.EventID == id {
			return e, nil
		}
	}
	return nil, nil
}
func (f *fakeEvents) ListForFingerprint(_ context.Context, o, p valuer.UUID, fp string, _ int) ([]*sentrytypes.Event, error) {
	var out []*sentrytypes.Event
	for _, e := range f.inserts[f.key(o, p)] {
		if e.Fingerprint == fp {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeEvents) ListForTrace(_ context.Context, o, p valuer.UUID, traceID string, _ int) ([]*sentrytypes.Event, error) {
	f.lastOrg, f.lastProj = o.String(), p.String()
	var out []*sentrytypes.Event
	for _, e := range f.inserts[f.key(o, p)] {
		if e.TraceID == traceID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeEvents) DistinctFingerprints(_ context.Context, o, p valuer.UUID, _ sentrytypes.Window) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, e := range f.inserts[f.key(o, p)] {
		if !seen[e.Fingerprint] {
			seen[e.Fingerprint] = true
			out = append(out, e.Fingerprint)
		}
	}
	return out, nil
}
func (f *fakeEvents) ListLogs(_ context.Context, o, p valuer.UUID, _ string, _ sentrytypes.Window, _ int) ([]*sentrytypes.Event, error) {
	f.lastOrg, f.lastProj = o.String(), p.String()
	return f.inserts[f.key(o, p)], nil
}
func (f *fakeEvents) ListTraces(_ context.Context, o, p valuer.UUID, _ sentrytypes.Window, _ int) ([]*sentrytypes.TraceSummary, error) {
	f.lastOrg, f.lastProj = o.String(), p.String()
	return nil, nil
}
func (f *fakeEvents) Stats(_ context.Context, o, p valuer.UUID, _ string, _ sentrytypes.Window) ([]sentrytypes.StatsPoint, error) {
	f.lastOrg, f.lastProj = o.String(), p.String()
	return nil, nil
}

type harness struct {
	mod      sentry.Module
	events   *fakeEvents
	projects sentrytypes.ProjectStore
}

func newModuleHarness(t *testing.T) *harness {
	t.Helper()
	store := newModuleSQLStore(t)
	projects := NewProjectStore(store)
	issues := errortracking.Module(implerrortracking.NewModule(implerrortracking.NewStore(store)))
	events := newFakeEvents()
	mod := NewModule(projects, events, issues)
	return &harness{mod: mod, events: events, projects: projects}
}

// newModuleSQLStore is a sqlite store with BOTH the projects table and the
// errortracking o11y_issues lifecycle table (+ its unique index).
func newModuleSQLStore(t *testing.T) sqlstore.SQLStore {
	t.Helper()
	store := newTestSQLStore(t) // creates o11y_sentry_projects (+ unique index)
	_, err := store.BunDB().NewCreateTable().Model((*errortrackingtypes.Issue)(nil)).IfNotExists().Exec(context.Background())
	require.NoError(t, err)
	_, err = store.BunDB().Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_o11y_issues_org_fingerprint ON o11y_issues (org_id, fingerprint)`)
	require.NoError(t, err)
	return store
}

func mustProject(t *testing.T, h *harness, org valuer.UUID, name string) *sentrytypes.GettableProject {
	t.Helper()
	p, err := h.mod.CreateProject(context.Background(), org, &sentrytypes.PostableProject{Name: name})
	require.NoError(t, err)
	return p
}

func occ(fp, eventID string) *errortrackingtypes.Occurrence {
	return occTrace(fp, eventID, "")
}

func occTrace(fp, eventID, traceID string) *errortrackingtypes.Occurrence {
	return &errortrackingtypes.Occurrence{
		EventID: eventID, Fingerprint: fp, Type: "Error", Value: "boom",
		Level: "error", Timestamp: time.Now().UTC(), TraceID: traceID,
	}
}

// ingest does what the event.error consumer does: the facts are already on the
// plane (seeded here), and Ingest files them into the org's issues.
func ingest(t *testing.T, h *harness, org string, project *sentrytypes.GettableProject, occs ...*errortrackingtypes.Occurrence) []*errortrackingtypes.Issue {
	t.Helper()
	h.events.seed(iamidentn.OrgUUID(org), project.Project.ID, occs...)
	created, err := h.mod.Ingest(context.Background(), org, project.Slug, occs)
	require.NoError(t, err)
	return created
}

// TestIngest_FilesIssuesAndReportsTheNewOnes: Ingest groups a batch into the org's
// issues, reports an issue as created exactly once, and provisions the product's
// project under the slug the plane stores — so a surface appears on its first error.
func TestIngest_FilesIssuesAndReportsTheNewOnes(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)

	created, err := h.mod.Ingest(ctx, "acme", "docs", []*errortrackingtypes.Occurrence{occ("fp-1", "e1"), occ("fp-1", "e2")})
	require.NoError(t, err)
	require.Len(t, created, 1, "two facts of one failure are one new issue")
	assert.Equal(t, "fp-1", created[0].Fingerprint)

	org := iamidentn.OrgUUID("acme")
	issues, err := h.mod.ListIssues(ctx, org, nil, &errortrackingtypes.IssuesQuery{}, testWindow())
	require.NoError(t, err)
	require.Len(t, issues.Items, 1)
	assert.Equal(t, int64(2), issues.Items[0].Count)

	projects, err := h.mod.ListProjects(ctx, org)
	require.NoError(t, err)
	require.Len(t, projects.Items, 1)
	assert.Equal(t, "docs", projects.Items[0].Slug, "the project is the product, by the plane's name")

	again, err := h.mod.Ingest(ctx, "acme", "docs", []*errortrackingtypes.Occurrence{occ("fp-1", "e3")})
	require.NoError(t, err)
	assert.Empty(t, again, "a known failure is not announced twice")

	_, err = h.mod.Ingest(ctx, "", "docs", nil)
	require.Error(t, err, "an ingest that names no org is refused")

	created, err = h.mod.Ingest(ctx, "acme", "", []*errortrackingtypes.Occurrence{occ("fp-2", "e4")})
	require.NoError(t, err)
	assert.Len(t, created, 1, "a fact with no product still files its issue")
	projects, err = h.mod.ListProjects(ctx, org)
	require.NoError(t, err)
	assert.Len(t, projects.Items, 1, "and lists no project for it")
}

// TestReads_ForeignProjectDenied is the mandatory read isolation: a project id that
// belongs to another org is rejected (never silently scoped to the caller), so no
// cross-tenant read is possible via a client-supplied project.
func TestReads_ForeignProjectDenied(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	orgA, orgB := valuer.GenerateUUID(), valuer.GenerateUUID()
	projA := mustProject(t, h, orgA, "a").Project.ID
	_ = mustProject(t, h, orgB, "b")

	// org B asks to Discover org A's project -> denied (project not found in B's org).
	_, err := h.mod.Discover(ctx, orgB, &sentrytypes.DiscoverRequest{Project: projA.String()})
	require.Error(t, err)

	// Same for logs / traces / stats / trace-detail — every project-scoped read.
	_, err = h.mod.ListLogs(ctx, orgB, projA, "", "24h", 10)
	require.Error(t, err)
	_, err = h.mod.ListTraces(ctx, orgB, projA, "24h", 10)
	require.Error(t, err)
	_, err = h.mod.Stats(ctx, orgB, projA, "events", "24h")
	require.Error(t, err)
	_, err = h.mod.TraceDetail(ctx, orgB, projA, "trace-1")
	require.Error(t, err)

	// The fake events store was NEVER asked for org A's data on B's behalf.
	assert.NotEqual(t, orgA.String(), h.events.lastOrg)
}

// TestListIssues_ProjectFilterViaEventsPlane proves the org-grouped issue list is
// correctly projected to a single project through the events-plane fingerprints, and
// that a project with no captured errors yields zero issues (never the whole org).
func TestListIssues_ProjectFilterViaEventsPlane(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	org := iamidentn.OrgUUID("acme")
	webP, apiP := mustProject(t, h, org, "web"), mustProject(t, h, org, "api")
	web := webP.Project.ID

	ingest(t, h, "acme", webP, occ("fp-web", "e1"))
	ingest(t, h, "acme", apiP, occ("fp-api", "e2"))

	// Whole-org list sees BOTH issues.
	all, err := h.mod.ListIssues(ctx, org, nil, &errortrackingtypes.IssuesQuery{}, testWindow())
	require.NoError(t, err)
	assert.Len(t, all.Items, 2)

	// Project-scoped list sees only that project's issue.
	webOnly, err := h.mod.ListIssues(ctx, org, &web, &errortrackingtypes.IssuesQuery{}, testWindow())
	require.NoError(t, err)
	require.Len(t, webOnly.Items, 1)
	assert.Equal(t, "fp-web", webOnly.Items[0].Fingerprint)

	// A foreign project on the issue list is denied.
	otherOrg := valuer.GenerateUUID()
	foreign := mustProject(t, h, otherOrg, "x").Project.ID
	_, err = h.mod.ListIssues(ctx, org, &foreign, &errortrackingtypes.IssuesQuery{}, testWindow())
	require.Error(t, err)
}

// TestTraceDetail_CrossTenantTraceIsolation is the exact scenario Red flagged: org B
// injects an event carrying org A's trace_id, then reads that trace. Because the
// load-bearing scope is on the events READ (org AND project bound), TraceDetail returns
// ONLY org B's own project events for the trace — ZERO of org A's data — and the
// event.span plane is never read here — this returns the errors on the trace.
func TestTraceDetail_CrossTenantTraceIsolation(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	orgA, orgB := iamidentn.OrgUUID("acme"), iamidentn.OrgUUID("globex")
	projA, projB := mustProject(t, h, orgA, "a"), mustProject(t, h, orgB, "b")
	pA, pB := projA.Project.ID, projB.Project.ID

	const victimTrace = "VICTIM-TRACE-DEADBEEF"
	// org A's real event on the victim trace.
	ingest(t, h, "acme", projA, occTrace("fp-a", "a-secret", victimTrace))
	// org B forges an event CLAIMING the same trace id in ITS OWN project.
	ingest(t, h, "globex", projB, occTrace("fp-b", "b-own", victimTrace))

	// org B reads the trace in its own project: sees ONLY its own event, never org A's.
	detail, err := h.mod.TraceDetail(ctx, orgB, pB, victimTrace)
	require.NoError(t, err)
	events := detail.(map[string]any)["events"].([]*sentrytypes.Event)
	require.Len(t, events, 1)
	assert.Equal(t, "b-own", events[0].EventID)
	for _, e := range events {
		assert.NotEqual(t, "a-secret", e.EventID, "org A's event must NEVER surface for org B")
		assert.Equal(t, orgB.String(), e.OrgID, "only org B rows may be returned")
	}

	// org B cannot even target org A's project (foreign project → denied).
	_, err = h.mod.TraceDetail(ctx, orgB, pA, victimTrace)
	require.Error(t, err)
}

// TestGetEvent_ProjectScoped: a within-tenant cross-PROJECT read is denied — a project
// is the isolation unit, so an event in project X is not readable via project Y.
func TestGetEvent_ProjectScoped(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	org := iamidentn.OrgUUID("acme")
	webP := mustProject(t, h, org, "web")
	web, api := webP.Project.ID, mustProject(t, h, org, "api").Project.ID
	ingest(t, h, "acme", webP, occ("fp", "evt-web"))

	// Correct project → found.
	got, err := h.mod.GetEvent(ctx, org, web, "evt-web")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "evt-web", got.EventID)

	// Wrong project (same org) → not found (not a leak).
	got, err = h.mod.GetEvent(ctx, org, api, "evt-web")
	require.NoError(t, err)
	assert.Nil(t, got, "event from another project must not be readable via a different project")

	// Foreign project (another org) → denied.
	other := valuer.GenerateUUID()
	foreign := mustProject(t, h, other, "x").Project.ID
	_, err = h.mod.GetEvent(ctx, org, foreign, "evt-web")
	require.Error(t, err)
}

// TestIssueEvents_ProjectScoped: issue occurrences are read only for the named project.
func TestIssueEvents_ProjectScoped(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	org := iamidentn.OrgUUID("acme")
	webP, apiP := mustProject(t, h, org, "web"), mustProject(t, h, org, "api")
	web := webP.Project.ID
	ingest(t, h, "acme", webP, occ("fp-shared", "e-web"))
	ingest(t, h, "acme", apiP, occ("fp-shared", "e-api"))

	// One org-scoped issue exists for fp-shared; find it.
	issues, err := h.mod.ListIssues(ctx, org, nil, &errortrackingtypes.IssuesQuery{}, testWindow())
	require.NoError(t, err)
	require.Len(t, issues.Items, 1)
	issueID := issues.Items[0].ID

	// Occurrences scoped to web → only the web event.
	webEvents, err := h.mod.IssueEvents(ctx, org, issueID, web, 0)
	require.NoError(t, err)
	require.Len(t, webEvents, 1)
	assert.Equal(t, "e-web", webEvents[0].EventID)

	// Foreign project → denied.
	other := valuer.GenerateUUID()
	foreign := mustProject(t, h, other, "x").Project.ID
	_, err = h.mod.IssueEvents(ctx, org, issueID, foreign, 0)
	require.Error(t, err)
}
