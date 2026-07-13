package telemetrystoretest

import (
	"context"

	"github.com/DATA-DOG/go-sqlmock"
	dsmock "github.com/hanzo-ds/mock"
	"github.com/hanzoai/o11y/pkg/telemetrystore/datastoremock"
	"github.com/hanzo-ds/go"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/datastoretelemetrystore"
	"github.com/hanzoai/o11y/pkg/types/telemetrystoretypes"
)

var _ telemetrystore.TelemetryStore = (*Provider)(nil)

// Provider represents a mock telemetry store provider for testing.
type Provider struct {
	datastoreDB datastoremock.Conn
}

// New creates a new mock telemetry store provider.
func New(_ telemetrystore.Config, matcher sqlmock.QueryMatcher) *Provider {
	datastoreDB, err := dsmock.NewDatastoreWithQueryMatcher(&datastore.Options{}, matcher)
	if err != nil {
		panic(err)
	}

	return &Provider{
		datastoreDB: datastoreDB,
	}
}

// Datastore returns the mock Datastore connection.
func (p *Provider) Datastore() datastore.Conn {
	return p.datastoreDB.(datastore.Conn)
}

// Cluster returns the cluster name.
func (p *Provider) Cluster() string {
	return "cluster"
}

// Estimate runs EXPLAIN ESTIMATE against the mock connection.
func (p *Provider) Estimate(ctx context.Context, stmt string, args ...any) ([]telemetrystoretypes.EstimateEntry, error) {
	return datastoretelemetrystore.RunExplainEstimate(ctx, p.datastoreDB.(datastore.Conn), stmt, args...)
}

// Plan runs EXPLAIN PLAN against the mock connection.
func (p *Provider) Plan(ctx context.Context, stmt string, args ...any) error {
	return datastoretelemetrystore.RunExplainPlan(ctx, p.datastoreDB.(datastore.Conn), stmt, args...)
}

// Indexes runs EXPLAIN indexes against the mock connection.
func (p *Provider) Indexes(ctx context.Context, stmt string, args ...any) (telemetrystoretypes.Granules, bool, error) {
	return datastoretelemetrystore.RunExplainIndexes(ctx, p.datastoreDB.(datastore.Conn), stmt, args...)
}

// Mock returns the underlying Datastore mock instance for setting expectations.
func (p *Provider) Mock() datastoremock.Conn {
	return p.datastoreDB
}
