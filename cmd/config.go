package cmd

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/pkg/community"
	"github.com/hanzoai/o11y/pkg/o11y"
)

// NewO11yConfig resolves the O11y config from the given YAML files plus the
// process environment. It delegates to community.NewConfig — the ONE config path
// shared by the standalone binary and the hanzoai/cloud embed — so both read
// configuration (and the Hanzo operator-facing aliases like O11Y_DATASTORE_DSN)
// identically.
func NewO11yConfig(ctx context.Context, logger *slog.Logger, configFiles []string) (o11y.Config, error) {
	return community.NewConfig(ctx, logger, configFiles)
}
