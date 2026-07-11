package implsentry

import (
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWindow() sentrytypes.Window {
	return sentrytypes.Window{From: time.Unix(1000, 0), To: time.Unix(2000, 0)}
}

// TestScopeIsTenantFirst pins the single most important invariant: every read's WHERE
// prefix binds org_id AND project_id FIRST, then the window — so no read can omit the
// tenant boundary, and the tenant values are always bound params (never interpolated).
func TestScopeIsTenantFirst(t *testing.T) {
	where, args := scope("org-A", "proj-1", testWindow())
	assert.Equal(t, "org_id = ? AND project_id = ? AND timestamp >= ? AND timestamp <= ?", where)
	require.Len(t, args, 4)
	assert.Equal(t, "org-A", args[0])
	assert.Equal(t, "proj-1", args[1])
}

func TestBuildDiscover_ScopedAndBound(t *testing.T) {
	req := &sentrytypes.DiscoverRequest{
		GroupBy:      []string{"level"},
		Aggregations: []string{"count", "users"},
		Filters:      []sentrytypes.DiscoverFilter{{Field: "environment", Op: "eq", Value: "prod"}},
		OrderBy:      "count",
		OrderDir:     "desc",
	}
	sql, args, cols, err := buildDiscover("o11y_sentry", "o11y_sentry_events", "org-A", "proj-1", req, testWindow())
	require.NoError(t, err)

	// org + project are the FIRST two bound args, always.
	assert.Equal(t, "org-A", args[0])
	assert.Equal(t, "proj-1", args[1])
	assert.Contains(t, sql, "org_id = ? AND project_id = ?")
	// The filter value is bound, never inlined.
	assert.Contains(t, sql, "environment = ?")
	assert.Contains(t, args, "prod")
	// Fixed aggregation expressions, aliased by key.
	assert.Contains(t, sql, "count() AS count")
	assert.Contains(t, sql, "count(DISTINCT user_id) AS users")
	assert.Contains(t, sql, "GROUP BY level")
	assert.Contains(t, sql, "ORDER BY count DESC")
	assert.Equal(t, []discoverCol{{"level", kindString}, {"count", kindUint}, {"users", kindUint}}, cols)
}

// TestBuildDiscover_RejectsInjection proves no client string reaches the SQL as an
// identifier: any field/aggregation/orderBy outside the allowlist is an error, never
// interpolated.
func TestBuildDiscover_RejectsInjection(t *testing.T) {
	injection := "value) FROM o11y_sentry.o11y_sentry_events; DROP TABLE o11y_sentry_events --"

	cases := []struct {
		name string
		req  *sentrytypes.DiscoverRequest
	}{
		{"groupBy", &sentrytypes.DiscoverRequest{GroupBy: []string{injection}}},
		{"aggregation", &sentrytypes.DiscoverRequest{Aggregations: []string{injection}}},
		{"filter field", &sentrytypes.DiscoverRequest{Filters: []sentrytypes.DiscoverFilter{{Field: injection, Value: "x"}}}},
		{"filter op", &sentrytypes.DiscoverRequest{Filters: []sentrytypes.DiscoverFilter{{Field: "level", Op: "; DROP", Value: "x"}}}},
		{"orderBy", &sentrytypes.DiscoverRequest{GroupBy: []string{"level"}, OrderBy: injection}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, args, _, err := buildDiscover("db", "t", "org", "proj", c.req, testWindow())
			require.Error(t, err, "malicious %s must be rejected, not built", c.name)
			// The injection never becomes SQL: the builder returns an error and NO
			// statement/args. (The error message may name the rejected field for the
			// operator — that string is never executed.)
			assert.Empty(t, sql)
			assert.Empty(t, args)
		})
	}
}

// TestBuildDiscover_MaliciousFilterValueIsBound confirms a value that LOOKS like SQL is
// carried as a bound arg, not spliced into the statement.
func TestBuildDiscover_MaliciousFilterValueIsBound(t *testing.T) {
	evil := "'; DROP TABLE o11y_sentry_events; --"
	req := &sentrytypes.DiscoverRequest{
		GroupBy: []string{"level"},
		Filters: []sentrytypes.DiscoverFilter{{Field: "release", Op: "eq", Value: evil}},
	}
	sql, args, _, err := buildDiscover("db", "t", "org", "proj", req, testWindow())
	require.NoError(t, err)
	assert.NotContains(t, sql, "DROP TABLE", "value must not appear in the SQL text")
	assert.Contains(t, args, evil, "value must be a bound arg")
}

func TestBuildStats_ScopedAndFieldAllowlist(t *testing.T) {
	sql, args, err := buildStats("db", "t", "org-A", "proj-1", "errors", testWindow())
	require.NoError(t, err)
	assert.Equal(t, "org-A", args[0])
	assert.Equal(t, "proj-1", args[1])
	assert.Contains(t, sql, "org_id = ? AND project_id = ?")
	assert.Contains(t, sql, "level IN ('error','fatal')")

	_, _, err = buildStats("db", "t", "org", "proj", "sneaky') OR 1=1 --", testWindow())
	require.Error(t, err, "unknown stats field must be rejected")
}

func TestRowReads_AreOrgAndProjectScoped(t *testing.T) {
	// Event detail is org+project bound (a project is an isolation unit).
	get, gArgs := buildGetEvent("db", "t", "org-A", "proj-1", "evt-1")
	assert.Contains(t, get, "org_id = ? AND project_id = ? AND event_id = ?")
	assert.Equal(t, "org-A", gArgs[0])
	assert.Equal(t, "proj-1", gArgs[1])

	// Issue occurrences are org+project bound.
	fp, fArgs := buildListForFingerprint("db", "t", "org-A", "proj-1", "fp-1", 10)
	assert.Contains(t, fp, "org_id = ? AND project_id = ? AND fingerprint = ?")
	assert.Equal(t, "org-A", fArgs[0])
	assert.Equal(t, "proj-1", fArgs[1])

	// Trace detail reads the EVENTS plane (org+project+trace bound) — never o11y_traces.
	ft, ftArgs := buildListForTrace("db", "t", "org-A", "proj-1", "trace-xyz", 10)
	assert.Contains(t, ft, "org_id = ? AND project_id = ? AND trace_id = ?")
	assert.Equal(t, "org-A", ftArgs[0])
	assert.Equal(t, "proj-1", ftArgs[1])
	assert.Equal(t, "trace-xyz", ftArgs[2])

	logs, lArgs := buildListLogs("db", "t", "org-A", "proj-1", "boom", testWindow(), 10)
	assert.Contains(t, logs, "org_id = ? AND project_id = ?")
	assert.Equal(t, "org-A", lArgs[0])
	assert.Equal(t, "proj-1", lArgs[1])
	assert.Contains(t, logs, "message LIKE ? OR value LIKE ?")

	tr, tArgs := buildListTraces("db", "t", "org-A", "proj-1", testWindow(), 10)
	assert.Contains(t, tr, "org_id = ? AND project_id = ?")
	assert.Equal(t, "org-A", tArgs[0])

	df, dArgs := buildDistinctFingerprints("db", "t", "org-A", "proj-1", testWindow())
	assert.Contains(t, df, "org_id = ? AND project_id = ?")
	assert.Equal(t, "org-A", dArgs[0])
}

func TestResolveWindow(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	w := resolveWindow("7d", now)
	assert.Equal(t, now, w.To)
	assert.Equal(t, now.Add(-7*24*time.Hour), w.From)

	// Unknown period falls back to the default (24h), never an unbounded scan.
	def := resolveWindow("garbage", now)
	assert.Equal(t, now.Add(-defaultPeriod), def.From)
}

func TestClampLimit(t *testing.T) {
	assert.Equal(t, 100, clampLimit(0, 100, 1000))
	assert.Equal(t, 1000, clampLimit(5000, 100, 1000))
	assert.Equal(t, 42, clampLimit(42, 100, 1000))
}

func TestBuildDiscover_TooManyGroupBy(t *testing.T) {
	req := &sentrytypes.DiscoverRequest{GroupBy: strings.Split("a,b,c,d,e,f,g", ",")}
	_, _, _, err := buildDiscover("db", "t", "org", "proj", req, testWindow())
	require.Error(t, err)
}
