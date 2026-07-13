package telemetrymetadata

import (
	"fmt"
	"testing"

	"github.com/hanzoai/o11y/pkg/querybuilder"
	"github.com/hanzoai/o11y/pkg/telemetrylogs"
	"github.com/hanzoai/otel-collector/constants"
	"github.com/stretchr/testify/require"
)

func TestBuildListLogsJSONIndexesQuery(t *testing.T) {
	testCases := []struct {
		name         string
		cluster      string
		filters      []string
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name:    "No filters",
			cluster: "test-cluster",
			filters: nil,
			expectedSQL: "SELECT name, type_full, expr, granularity FROM clusterAllReplicas('test-cluster', system.data_skipping_indices) " +
				"WHERE database = ? AND table = ? AND (expr ILIKE ? OR expr ILIKE ?)",
			expectedArgs: []any{
				telemetrylogs.DBName,
				telemetrylogs.LogsV2LocalTableName,
				fmt.Sprintf("%%%s%%", querybuilder.FormatValueForContains(constants.BodyV2ColumnPrefix)),
				fmt.Sprintf("%%%s%%", querybuilder.FormatValueForContains(constants.BodyPromotedColumnPrefix)),
			},
		},
		{
			name:    "With filters",
			cluster: "test-cluster",
			filters: []string{"foo", "bar"},
			expectedSQL: "SELECT name, type_full, expr, granularity FROM clusterAllReplicas('test-cluster', system.data_skipping_indices) " +
				"WHERE database = ? AND table = ? AND (expr ILIKE ? OR expr ILIKE ?) AND (replaceAll(expr, '`', '') ILIKE ? OR replaceAll(expr, '`', '') ILIKE ?)",
			expectedArgs: []any{
				telemetrylogs.DBName,
				telemetrylogs.LogsV2LocalTableName,
				fmt.Sprintf("%%%s%%", querybuilder.FormatValueForContains(constants.BodyV2ColumnPrefix)),
				fmt.Sprintf("%%%s%%", querybuilder.FormatValueForContains(constants.BodyPromotedColumnPrefix)),
				fmt.Sprintf("%%%s%%", querybuilder.FormatValueForContains("foo")),
				fmt.Sprintf("%%%s%%", querybuilder.FormatValueForContains("bar")),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, args := buildListLogsJSONIndexesQuery(tc.cluster, tc.filters...)

			require.Equal(t, tc.expectedSQL, query)
			require.Equal(t, tc.expectedArgs, args)
		})
	}
}
