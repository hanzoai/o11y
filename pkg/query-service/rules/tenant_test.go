package rules

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstoretest"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/require"
)

// orgNameStore is a sqlstore that answers the rule's one org-name lookup with
// name for orgID.
func orgNameStore(t *testing.T, orgID valuer.UUID, name string) sqlstore.SQLStore {
	t.Helper()
	store := sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherRegexp)
	store.Mock().ExpectQuery(`SELECT "name" FROM "organizations" WHERE \(id = '` + orgID.StringValue() + `'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow(name))
	return store
}

// A rule that reads logs or traces runs with its org's name as the tenant; a
// metrics-only rule needs no lookup.
func TestRuleReadsCarryTheOrgsTenant(t *testing.T) {
	orgID := valuer.GenerateUUID()
	logs := &qbtypes.QueryRangeRequest{CompositeQuery: qbtypes.CompositeQuery{Queries: []qbtypes.QueryEnvelope{{
		Type: qbtypes.QueryTypeBuilder,
		Spec: qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]{Signal: telemetrytypes.SignalLogs},
	}}}}
	metrics := &qbtypes.QueryRangeRequest{CompositeQuery: qbtypes.CompositeQuery{Queries: []qbtypes.QueryEnvelope{{
		Type: qbtypes.QueryTypeBuilder,
		Spec: qbtypes.QueryBuilderQuery[qbtypes.MetricAggregation]{Signal: telemetrytypes.SignalMetrics},
	}}}}

	r := &BaseRule{id: "rule-1", orgID: orgID, sqlstore: orgNameStore(t, orgID, "acme")}
	ctx, err := r.withTenant(context.Background(), logs)
	require.NoError(t, err)
	tenant, err := authtypes.TenantFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, "acme", tenant)

	bare := &BaseRule{id: "rule-2", orgID: orgID}
	ctx, err = bare.withTenant(context.Background(), metrics)
	require.NoError(t, err)
	_, err = authtypes.TenantFromContext(ctx)
	require.Error(t, err)
}
