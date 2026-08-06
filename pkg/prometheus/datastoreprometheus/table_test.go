package datastoreprometheus

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PromQL addresses the event plane. It once addressed o11y_metrics, a database
// that does not exist on this deployment, and every dashboard and alert rule
// that reads a metric failed with "Database o11y_metrics does not exist" —
// silently, because a read path that returns an error returns no rows and an
// empty chart looks like a quiet system. These assert the rendered SQL, since
// that is the only artifact the datastore actually sees.

func TestSeriesTableIsTheEventPlane(t *testing.T) {
	hour := time.Hour.Milliseconds()

	for _, tc := range []struct {
		name   string
		window int64
		table  string
	}{
		{"under 6h reads raw series", 1 * hour, telemetrymetrics.SeriesTableName},
		{"under a day reads the 6h rollup", 12 * hour, telemetrymetrics.Series6hTableName},
		{"under a week reads the 1d rollup", 48 * hour, telemetrymetrics.Series1dTableName},
		{"beyond a week reads the 1w rollup", 30 * 24 * hour, telemetrymetrics.Series1wTableName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := 30 * 24 * hour
			_, _, table := getStartAndEndAndTableName(end-tc.window, end)
			assert.Equal(t, tc.table, table)
			assert.False(t, strings.HasPrefix(table, "distributed_"),
				"no Distributed wrappers exist on this deployment")
		})
	}
}

func TestRenderedSQLNamesLiveTables(t *testing.T) {
	// The database is the one event plane, never the retired metrics database.
	assert.Equal(t, "event", databaseName)
	assert.Equal(t, "metric", samplesTableName)

	c := &client{}
	q := &prompb.Query{
		StartTimestampMs: 0,
		EndTimestampMs:   time.Hour.Milliseconds(),
		Matchers: []*prompb.LabelMatcher{
			{Type: prompb.LabelMatcher_EQ, Name: "service", Value: "cloud"},
		},
	}

	seriesSQL, _, err := c.queryToDatastoreQuery(context.Background(), q, "hanzo_service_up", false)
	require.NoError(t, err)
	assert.Contains(t, seriesSQL, "FROM event.series ")
	assert.Contains(t, seriesSQL, "any(labels)", "labels live on the series dimension")
	assert.NotContains(t, seriesSQL, "o11y_metrics")

	samplesSQL, _ := buildSamplesQuery(0, time.Hour.Milliseconds(), "hanzo_service_up", seriesSQL, nil)
	assert.Contains(t, samplesSQL, "FROM event.metric")
	assert.NotContains(t, samplesSQL, "distributed_samples_v4")
}
