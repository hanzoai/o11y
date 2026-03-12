package o11y

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hanzoai/o11y/pkg/alertmanager/nfmanager/nfmanagertest"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/modules/organization/implorganization"
	"github.com/hanzoai/o11y/pkg/modules/user/impluser"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlschema/sqlschematest"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstoretest"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/hanzoai/o11y/pkg/tokenizer/tokenizertest"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/stretchr/testify/assert"
)

// This is a test to ensure that provider factories can be created without panicking since
// we are using the factory.MustNewNamedMap function to initialize the provider factories.
// It also helps us catch these errors during testing instead of runtime.
func TestNewProviderFactories(t *testing.T) {
	assert.NotPanics(t, func() {
		NewCacheProviderFactories()
	})

	assert.NotPanics(t, func() {
		NewWebProviderFactories()
	})

	assert.NotPanics(t, func() {
		NewSQLStoreProviderFactories()
	})

	assert.NotPanics(t, func() {
		NewTelemetryStoreProviderFactories()
	})

	assert.NotPanics(t, func() {
		NewSQLMigrationProviderFactories(
			sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual),
			sqlschematest.New(map[string]*sqlschema.Table{}, map[string][]*sqlschema.UniqueConstraint{}, map[string]sqlschema.Index{}),
			telemetrystoretest.New(telemetrystore.Config{Provider: "clickhouse"}, sqlmock.QueryMatcherEqual),
			instrumentationtest.New().ToProviderSettings(),
		)
	})

	assert.NotPanics(t, func() {
		NewPrometheusProviderFactories(telemetrystoretest.New(telemetrystore.Config{Provider: "clickhouse"}, sqlmock.QueryMatcherEqual))
	})

	assert.NotPanics(t, func() {
		orgGetter := implorganization.NewGetter(implorganization.NewStore(sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual)), nil)
		notificationManager := nfmanagertest.NewMock()
		NewAlertmanagerProviderFactories(sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual), orgGetter, notificationManager)
	})

	assert.NotPanics(t, func() {
		queryParser := queryparser.New(instrumentationtest.New().ToProviderSettings())
		NewRulerProviderFactories(sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual), queryParser)
	})

	assert.NotPanics(t, func() {
		NewEmailingProviderFactories()
	})

	assert.NotPanics(t, func() {
		NewSharderProviderFactories()
	})

	assert.NotPanics(t, func() {
		flagger, err := flagger.New(context.Background(), instrumentationtest.New().ToProviderSettings(), flagger.Config{}, flagger.MustNewRegistry())
		if err != nil {
			panic(err)
		}

		userGetter := impluser.NewGetter(impluser.NewStore(sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual), instrumentationtest.New().ToProviderSettings()), flagger)
		orgGetter := implorganization.NewGetter(implorganization.NewStore(sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual)), nil)
		telemetryStore := telemetrystoretest.New(telemetrystore.Config{Provider: "clickhouse"}, sqlmock.QueryMatcherEqual)
		NewStatsReporterProviderFactories(telemetryStore, []statsreporter.StatsCollector{}, orgGetter, userGetter, tokenizertest.NewMockTokenizer(t), version.Build{}, analytics.Config{Enabled: true})
	})

	assert.NotPanics(t, func() {
		NewAPIServerProviderFactories(
			implorganization.NewGetter(implorganization.NewStore(sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherEqual)), nil),
			nil,
			nil,
			Modules{},
			Handlers{},
		)
	})
}
