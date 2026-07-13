package community

import (
	"context"
	"log/slog"
	"testing"
)

// The hanzoai/cloud embed and the standalone binary both resolve config through
// NewConfig. The flat operator knob O11Y_DATASTORE_DSN MUST win and land on the
// telemetry store DSN — this is the ONE var the cloud CR sets to point the
// in-process runtime at the shared Datastore (Hanzo Datastore). If this alias
// regresses, the embed silently talks to localhost and serves no telemetry.
func TestNewConfigAppliesDatastoreDSNAlias(t *testing.T) {
	const dsn = "tcp://datastore.hanzo.svc:9000?username=u&password=p"
	t.Setenv("O11Y_DATASTORE_DSN", dsn)
	t.Setenv("O11Y_TELEMETRYSTORE_DATASTORE_CLUSTER", "insights")

	config, err := NewConfig(context.Background(), slog.Default(), nil)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}

	if got := config.TelemetryStore.Datastore.DSN; got != dsn {
		t.Fatalf("telemetrystore DSN = %q, want the O11Y_DATASTORE_DSN value %q", got, dsn)
	}
	if got := config.TelemetryStore.Datastore.Cluster; got != "insights" {
		t.Fatalf("telemetrystore cluster = %q, want %q", got, "insights")
	}
}
