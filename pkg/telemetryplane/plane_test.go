package telemetryplane_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/telemetryaudit"
	"github.com/hanzoai/o11y/pkg/telemetrylogs"
	"github.com/hanzoai/o11y/pkg/telemetrymetadata"
	"github.com/hanzoai/o11y/pkg/telemetrymeter"
	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/hanzoai/o11y/pkg/telemetryplane"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"
)

// TestPlaneIsOneName pins the value. Everything below only checks that nothing
// else spells it; this checks it is right.
func TestPlaneIsOneName(t *testing.T) {
	assert.Equal(t, "event", telemetryplane.DBName)
}

// TestEverySignalResolvesToThePlane is the assertion HIP-0132 needed and did not
// have. The three signal packages each declared their own DBName; two of the
// three were wrong for six weeks and nothing failed.
func TestEverySignalResolvesToThePlane(t *testing.T) {
	for name, got := range map[string]string{
		"telemetrylogs":    telemetrylogs.DBName,
		"telemetrytraces":  telemetrytraces.DBName,
		"telemetrymetrics": telemetrymetrics.DBName,
	} {
		assert.Equalf(t, telemetryplane.DBName, got,
			"%s.DBName must BE the plane, not a copy of its current value", name)
	}
}

// TestSurvivorsAreAliasedNotRespelled — the four databases HIP-0132 did not
// unify still exist and still have writers, so their names are real. What is not
// allowed is spelling one twice: that is exactly how o11y_metadata ended up in
// three files and o11y_analytics in two.
func TestSurvivorsAreAliasedNotRespelled(t *testing.T) {
	assert.Equal(t, telemetryplane.MetadataDBName, telemetrymetadata.DBName)
	assert.Equal(t, telemetryplane.MeterDBName, telemetrymeter.DBName)
	assert.Equal(t, telemetryplane.AuditDBName, telemetryaudit.DBName)

	// And none of the survivors is the plane — an alias that collapsed onto
	// `event` would silently read a database nothing writes.
	for _, survivor := range []string{
		telemetryplane.MetadataDBName,
		telemetryplane.MeterDBName,
		telemetryplane.AuditDBName,
		telemetryplane.AnalyticsDBName,
	} {
		assert.NotEqual(t, telemetryplane.DBName, survivor)
	}
}

// dropped matches a database HIP-0132 DELETED, in either form a Go string
// literal can carry it: bare (`"o11y_logs"`, how five constants held it) or as a
// SQL qualifier (`"… FROM o11y_traces.distributed_o11y_index_v3"`, how nineteen
// templates held it). Both forms shipped; both must be unreachable.
//
// It enumerates three names rather than matching `o11y_*` because
// `o11y_calls_total` and `o11y_latency_bucket` are Prometheus metrics this fork
// emits about ITSELF, not databases.
var dropped = regexp.MustCompile(`o11y_(logs|traces|metrics)\b`)

// survivor matches the four databases HIP-0132 did NOT unify. They are real, so
// naming one is not by itself a defect — spelling it in a second place is.
var survivor = regexp.MustCompile(`^o11y_(metadata|meter|audit|analytics)$`)

// TestNoDroppedDatabaseIsNamedAnywhere is the point of the package.
//
// THE DEFECT THIS CATCHES IS THE ONE THAT HAPPENED. HIP-0132 dropped o11y_logs,
// o11y_traces and o11y_metrics. A commit repointed THREE files at `event` and
// left thirty spelling the dropped names, and the tree stayed GREEN — because a
// string literal naming a database that no longer exists compiles perfectly, and
// because the golden tests asserted the same dead SQL the readers emitted. Two
// copies of a wrong answer agree with each other. The failure surfaced only in
// production: 316 `Code: 81 UNKNOWN_DATABASE` in thirty minutes, five views
// showing an error boundary or a false empty.
//
// That is why this walks TEST files too. A golden that encodes the broken SQL is
// not a test, it is a second vote for the bug.
//
// It parses rather than greps, so a dropped name inside a COMMENT — of which
// there are many, deliberately, recording what moved where — is not a finding.
// Only a string literal in code is.
func TestNoDroppedDatabaseIsNamedAnywhere(t *testing.T) {
	root := repoRoot(t)

	type finding struct{ file, lit string }
	var findings []finding

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "frontend", "vendor", "attic":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		// This package is the one address. Its own test is where the names are
		// enumerated, so both are exempt.
		if strings.HasPrefix(rel, filepath.Join("pkg", "telemetryplane")) {
			return nil
		}
		// The datastore cutover migrations are a RECORD of the rename. They have
		// to say the old names; that is what a migration is.
		if strings.HasPrefix(rel, filepath.Join("deploy", "datastore")) {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0) // 0 => comments dropped
		if parseErr != nil {
			return nil // not our business to police unparseable files
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, uqErr := strconv.Unquote(lit.Value)
			if uqErr != nil {
				return true
			}
			if m := dropped.FindString(v); m != "" {
				findings = append(findings, finding{
					file: rel + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line),
					lit:  m,
				})
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)

	sort.Slice(findings, func(i, j int) bool { return findings[i].file < findings[j].file })
	if len(findings) > 0 {
		var b strings.Builder
		b.WriteString("a database HIP-0132 DROPPED is still named in code:\n")
		for _, f := range findings {
			b.WriteString("  " + f.file + "  " + strconv.Quote(f.lit) + "\n")
		}
		b.WriteString("\nEvery read through one of these answers Code: 81 UNKNOWN_DATABASE.\n")
		b.WriteString("Use telemetryplane.DBName, and the table constants in\n")
		b.WriteString("pkg/telemetry{logs,traces,metrics}/tables.go for the tables.\n")
		t.Fatal(b.String())
	}
}

// repoRoot walks up from this test file to the directory holding go.mod. The
// walk above has to start from the repository, not from this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "walked to / without finding go.mod")
		dir = parent
	}
}

// TestSurvivorsAreSpelledOnce covers the other half. o11y_metadata, o11y_meter,
// o11y_audit and o11y_analytics still exist and still have writers, so naming one
// is correct — naming one TWICE is the defect, and it had already happened:
// o11y_metadata was spelled in three files and o11y_analytics in two, which is
// how a rename lands in one of them and not the others.
//
// Golden SQL fixtures are exempt: a fixture asserting
// `FROM o11y_audit.distributed_logs` is asserting the SQL a live reader emits,
// and it goes red on its own the moment that reader moves. What is checked here
// is the BARE literal — a constant declaration — which is the form that drifts
// silently.
func TestSurvivorsAreSpelledOnce(t *testing.T) {
	root := repoRoot(t)

	type finding struct{ file, lit string }
	var findings []finding

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "frontend", "vendor", "attic":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if strings.HasPrefix(rel, filepath.Join("pkg", "telemetryplane")) {
			return nil
		}
		if strings.HasPrefix(rel, filepath.Join("deploy", "datastore")) {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, uqErr := strconv.Unquote(lit.Value)
			if uqErr != nil {
				return true
			}
			if survivor.MatchString(v) {
				findings = append(findings, finding{
					file: rel + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line),
					lit:  v,
				})
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)

	sort.Slice(findings, func(i, j int) bool { return findings[i].file < findings[j].file })
	if len(findings) > 0 {
		var b strings.Builder
		b.WriteString("a database name is spelled outside pkg/telemetryplane:\n")
		for _, f := range findings {
			b.WriteString("  " + f.file + "  " + strconv.Quote(f.lit) + "\n")
		}
		b.WriteString("\nUse the telemetryplane.<X>DBName alias. One value, one address.\n")
		t.Fatal(b.String())
	}
}
