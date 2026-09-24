package rules

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstoretest"
	"github.com/hanzoai/o11y/pkg/types/ruletypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/require"
)

// A rule's alerts are addressed to the rule's own org, whichever org the
// organizations table happens to list first.
func TestSendAlertsAddressesTheRulesOrg(t *testing.T) {
	orgID := valuer.GenerateUUID()
	now := time.Now()

	// The organizations table lists another org first.
	store := sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherRegexp)
	store.Mock().ExpectQuery(`SELECT .* FROM .*organizations`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(valuer.GenerateUUID().StringValue()))

	r := &BaseRule{
		id:       "rule-1",
		orgID:    orgID,
		sqlstore: store,
		logger:   instrumentationtest.New().Logger(),
		Active: map[uint64]*ruletypes.Alert{
			1: {State: ruletypes.StateFiring, FiredAt: now.Add(-time.Minute)},
		},
	}

	var got string
	var sent int
	r.SendAlerts(context.Background(), now, time.Minute, time.Minute, func(_ context.Context, org string, alerts ...*ruletypes.Alert) {
		got, sent = org, len(alerts)
	})

	require.Equal(t, 1, sent)
	require.Equal(t, orgID.StringValue(), got)
}
