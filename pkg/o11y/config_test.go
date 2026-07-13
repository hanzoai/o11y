package o11y

import (
	"context"
	"log/slog"
	"testing"

	"github.com/hanzoai/o11y/pkg/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This is a test to ensure that all fields of config implement the factory.Config interface and are valid with
// their default values.
func TestValidateConfig(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	_, err := NewConfig(context.Background(), logger, configtest.NewResolverConfig())
	assert.NoError(t, err)
}

// O11Y_DATASTORE_DSN is the canonical, flat operator-facing key: it must map into
// the telemetrystore datastore DSN via mergeAndEnsureBackwardCompatibility.
func TestDatastoreDSNCanonicalAlias(t *testing.T) {
	const dsn = "tcp://datastore.hanzo.svc:9000/?database=o11y"
	t.Setenv("O11Y_DATASTORE_DSN", dsn)

	logger := slog.New(slog.DiscardHandler)
	config, err := NewConfig(context.Background(), logger, configtest.NewResolverConfig())
	require.NoError(t, err)
	assert.Equal(t, dsn, config.TelemetryStore.Datastore.DSN)
}
