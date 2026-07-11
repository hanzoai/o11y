package implsentry

import (
	"context"
	"testing"
	"time"

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

// fakeEvents is an in-memory EventStore: it records every write keyed by (org,
// project) so a test can assert BOTH that ingest reached the events plane AND that a
// read is only ever asked for the caller's own tenant.
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

func (f *fakeEvents) Insert(_ context.Context, o, p valuer.UUID, e []*sentrytypes.Event) error {
	f.inserts[f.key(o, p)] = append(f.inserts[f.key(o, p)], e...)
	return nil
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

const testSecret = "platform-ingest-secret"

type harness struct {
	mod      sentry.Module
	events   *fakeEvents
	projects sentrytypes.ProjectStore
}

func newModuleHarness(t *testing.T) *harness {
	t.Helper()
	store := newModuleSQLStore(t)
	projects := NewProjectStore(store)
	issues := errortracking.Module(implerrortracking.NewModule(implerrortracking.NewStore(store), implerrortracking.NewNoopSink()))
	events := newFakeEvents()
	mod := NewModule(projects, events, issues, Config{IngestSecret: []byte(testSecret), Host: "api.hanzo.ai"})
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

// TestIngest_WritesEventsAndIssues proves the dual write: one Ingest lands the
// occurrence on BOTH the columnar events plane and the grouped-issue lifecycle.
func TestIngest_WritesEventsAndIssues(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	org := valuer.GenerateUUID()
	proj := mustProject(t, h, org, "web")
	pid := proj.Project.ID

	require.NoError(t, h.mod.Ingest(ctx, org, pid, []*errortrackingtypes.Occurrence{occ("fp-1", "e1"), occ("fp-1", "e2")}))

	// Events plane got both occurrences under (org, project).
	assert.Len(t, h.events.inserts[[2]string{org.String(), pid.String()}], 2)

	// Issue lifecycle grouped them into one issue for the org.
	issues, err := h.mod.ListIssues(ctx, org, nil, &errortrackingtypes.IssuesQuery{}, testWindow())
	require.NoError(t, err)
	require.Len(t, issues.Items, 1)
	assert.Equal(t, "fp-1", issues.Items[0].Fingerprint)
	assert.Equal(t, int64(2), issues.Items[0].Count)
}

// TestResolveIngest_FailsClosed is the DSN gate: unknown project, disabled project,
// wrong key and a below-watermark (rotated) key all return ok=false; only a correct,
// current key resolves — to the OWNING org.
func TestResolveIngest_FailsClosed(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	org := valuer.GenerateUUID()
	proj := mustProject(t, h, org, "web")
	pid := proj.Project.ID

	validKey := implerrortracking.PublicKeyForVersion([]byte(testSecret), pid.String(), 1)

	// Valid key + active project -> resolves to the owning org.
	gotOrg, ok := h.mod.ResolveIngest(ctx, pid, validKey)
	require.True(t, ok)
	assert.Equal(t, org, gotOrg)

	// Unknown project -> closed.
	_, ok = h.mod.ResolveIngest(ctx, valuer.GenerateUUID(), validKey)
	assert.False(t, ok)

	// Wrong key -> closed.
	_, ok = h.mod.ResolveIngest(ctx, pid, "1:deadbeef")
	assert.False(t, ok)

	// Empty key -> closed.
	_, ok = h.mod.ResolveIngest(ctx, pid, "")
	assert.False(t, ok)

	// After rotation, the old v1 key is below the watermark -> closed; the new one works.
	_, err := h.projects.Rotate(ctx, org, pid)
	require.NoError(t, err)
	_, ok = h.mod.ResolveIngest(ctx, pid, validKey)
	assert.False(t, ok, "a below-watermark (pre-rotation) key must stop resolving")
	v2 := implerrortracking.PublicKeyForVersion([]byte(testSecret), pid.String(), 2)
	_, ok = h.mod.ResolveIngest(ctx, pid, v2)
	assert.True(t, ok)
}

func TestResolveIngest_DisabledProjectFailsClosed(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	org := valuer.GenerateUUID()
	proj := mustProject(t, h, org, "web")
	pid := proj.Project.ID

	// Disable the project directly in the store.
	disableProject(t, h, org, pid)

	key := implerrortracking.PublicKeyForVersion([]byte(testSecret), pid.String(), 1)
	_, ok := h.mod.ResolveIngest(ctx, pid, key)
	assert.False(t, ok, "a disabled project must not resolve even with a valid key")
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
	org := valuer.GenerateUUID()
	web := mustProject(t, h, org, "web").Project.ID
	api := mustProject(t, h, org, "api").Project.ID

	require.NoError(t, h.mod.Ingest(ctx, org, web, []*errortrackingtypes.Occurrence{occ("fp-web", "e1")}))
	require.NoError(t, h.mod.Ingest(ctx, org, api, []*errortrackingtypes.Occurrence{occ("fp-api", "e2")}))

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
// o11y_traces span plane (which has no org column) is never read.
func TestTraceDetail_CrossTenantTraceIsolation(t *testing.T) {
	ctx := context.Background()
	h := newModuleHarness(t)
	orgA, orgB := valuer.GenerateUUID(), valuer.GenerateUUID()
	pA := mustProject(t, h, orgA, "a").Project.ID
	pB := mustProject(t, h, orgB, "b").Project.ID

	const victimTrace = "VICTIM-TRACE-DEADBEEF"
	// org A's real event on the victim trace.
	require.NoError(t, h.mod.Ingest(ctx, orgA, pA, []*errortrackingtypes.Occurrence{occTrace("fp-a", "a-secret", victimTrace)}))
	// org B forges an event CLAIMING the same trace id in ITS OWN project.
	require.NoError(t, h.mod.Ingest(ctx, orgB, pB, []*errortrackingtypes.Occurrence{occTrace("fp-b", "b-own", victimTrace)}))

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
	org := valuer.GenerateUUID()
	web := mustProject(t, h, org, "web").Project.ID
	api := mustProject(t, h, org, "api").Project.ID
	require.NoError(t, h.mod.Ingest(ctx, org, web, []*errortrackingtypes.Occurrence{occ("fp", "evt-web")}))

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
	org := valuer.GenerateUUID()
	web := mustProject(t, h, org, "web").Project.ID
	api := mustProject(t, h, org, "api").Project.ID
	require.NoError(t, h.mod.Ingest(ctx, org, web, []*errortrackingtypes.Occurrence{occ("fp-shared", "e-web")}))
	require.NoError(t, h.mod.Ingest(ctx, org, api, []*errortrackingtypes.Occurrence{occ("fp-shared", "e-api")}))

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

// disableProject flips a project's status to disabled via a direct store write.
func disableProject(t *testing.T, h *harness, org, id valuer.UUID) {
	t.Helper()
	ps := h.projects.(*projectStore)
	_, err := ps.sqlstore.BunDB().NewUpdate().
		Model((*sentrytypes.Project)(nil)).
		Set("status = ?", sentrytypes.ProjectDisabled).
		Where("org_id = ?", org).Where("id = ?", id).
		Exec(context.Background())
	require.NoError(t, err)
}
