package datastorereader

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dsmock "github.com/hanzo-ds/mock"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/stretchr/testify/require"
)

// GetRecentLogs reads event.log's own columns for the tenant on ctx, binding
// the tenant, the window and the limit.
func TestGetRecentLogsReadsTheTenantsRows(t *testing.T) {
	store := telemetrystoretest.New(telemetrystore.Config{}, sqlmock.QueryMatcherEqual)
	r := &DatastoreReader{db: store.Datastore(), logsDB: "event", logsTableV2: "log", logger: instrumentationtest.New().Logger()}

	at := time.Unix(1790290369, 0).UTC()
	store.Mock().ExpectQuery(recentLogsQuery("event.log")).
		WithArgs("acme", "1790290000000000000", "1790291000000000000", 5).
		WillReturnRows(dsmock.NewRows([]dsmock.ColumnType{
			{Name: "timestamp", Type: "DateTime"},
			{Name: "id", Type: "String"},
			{Name: "body", Type: "String"},
			{Name: "service", Type: "String"},
		}, [][]any{{at, "row-1", "request", "cloud"}}))

	ctx := authtypes.NewContextWithTenant(context.Background(), "acme")
	rows, err := r.GetRecentLogs(ctx, 1790290000000000000, 1790291000000000000, 5)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, at, rows[0].Timestamp)
	require.NoError(t, store.Mock().ExpectationsWereMet())
}

// The statement names the envelope columns, never the dropped ones, and
// scopes by org.
func TestRecentLogsQueryUsesTheEnvelopeColumns(t *testing.T) {
	q := recentLogsQuery("event.log")
	require.Regexp(t, regexp.MustCompile(`^SELECT time AS timestamp, .* FROM event\.log WHERE org = \? AND time >= \? AND time <= \? ORDER BY time DESC LIMIT \?$`), q)
	for _, dropped := range []string{"resources_string", "resource.`", "timestamp >="} {
		require.NotContains(t, q, dropped)
	}
}

// A ctx with no tenant is refused before any read.
func TestGetRecentLogsRefusesWithoutATenant(t *testing.T) {
	r := &DatastoreReader{logsDB: "event", logsTableV2: "log"}
	_, err := r.GetRecentLogs(context.Background(), 0, 0, 10)
	require.True(t, errors.Ast(err, errors.TypeForbidden), "got %v", err)
}
