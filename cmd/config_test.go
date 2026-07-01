package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHanzoO11yConfig_NoConfigFiles(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	config, err := NewHanzoO11yConfig(context.Background(), logger, nil, o11y.DeprecatedFlags{})
	require.NoError(t, err)
	assert.NotZero(t, config)
}

func TestNewHanzoO11yConfig_SingleConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
cache:
  provider: "redis"
`), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.DiscardHandler)
	config, err := NewHanzoO11yConfig(context.Background(), logger, []string{configPath}, o11y.DeprecatedFlags{})
	require.NoError(t, err)
	assert.Equal(t, "redis", config.Cache.Provider)
}

func TestNewHanzoO11yConfig_MultipleConfigFiles_LaterOverridesEarlier(t *testing.T) {
	dir := t.TempDir()

	basePath := filepath.Join(dir, "base.yaml")
	err := os.WriteFile(basePath, []byte(`
cache:
  provider: "memory"
sqlstore:
  provider: "sqlite"
`), 0644)
	require.NoError(t, err)

	overridePath := filepath.Join(dir, "override.yaml")
	err = os.WriteFile(overridePath, []byte(`
cache:
  provider: "redis"
`), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.DiscardHandler)
	config, err := NewHanzoO11yConfig(context.Background(), logger, []string{basePath, overridePath}, o11y.DeprecatedFlags{})
	require.NoError(t, err)
	// Later file overrides earlier
	assert.Equal(t, "redis", config.Cache.Provider)
	// Value from base file that wasn't overridden persists
	assert.Equal(t, "sqlite", config.SQLStore.Provider)
}

func TestNewHanzoO11yConfig_EnvOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
cache:
  provider: "fromfile"
`), 0644)
	require.NoError(t, err)

	t.Setenv("O11Y_CACHE_PROVIDER", "fromenv")

	logger := slog.New(slog.DiscardHandler)
	config, err := NewHanzoO11yConfig(context.Background(), logger, []string{configPath}, o11y.DeprecatedFlags{})
	require.NoError(t, err)
	// Env should override file
	assert.Equal(t, "fromenv", config.Cache.Provider)
}

func TestNewHanzoO11yConfig_NonexistentFile(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	_, err := NewHanzoO11yConfig(context.Background(), logger, []string{"/nonexistent/config.yaml"}, o11y.DeprecatedFlags{})
	assert.Error(t, err)
}
