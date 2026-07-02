package telemetrystoretest

import (
	datastore "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	cmock "github.com/hanzoai/clickhouse-go-mock"
)

var _ telemetrystore.TelemetryStore = (*Provider)(nil)

// Provider represents a mock telemetry store provider for testing.
type Provider struct {
	datastoreDB cmock.ClickConnMockCommon
}

// New creates a new mock telemetry store provider.
func New(_ telemetrystore.Config, matcher sqlmock.QueryMatcher) *Provider {
	datastoreDB, err := cmock.NewClickHouseWithQueryMatcher(&datastore.Options{}, matcher)
	if err != nil {
		panic(err)
	}

	return &Provider{
		datastoreDB: datastoreDB,
	}
}

// ClickhouseDB returns the mock Clickhouse connection
func (p *Provider) ClickhouseDB() datastore.Conn {
	return p.datastoreDB.(datastore.Conn)
}

// Cluster returns the cluster name.
func (p *Provider) Cluster() string {
	return "cluster"
}

// Mock returns the underlying Clickhouse mock instance for setting expectations.
func (p *Provider) Mock() cmock.ClickConnMockCommon {
	return p.datastoreDB
}
