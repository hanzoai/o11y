package retention

import (
	"context"

	"github.com/hanzoai/o11y/pkg/types/retentiontypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Getter resolves retention data and expressions for read paths.
type Getter interface {
	// GetRetentionPolicySegments returns retention policy segments active over a half-open meter window.
	GetRetentionPolicySegments(ctx context.Context, orgID valuer.UUID, dbName string, tableName string, fallbackDefaultDays int, startMs int64, endMs int64) ([]*retentiontypes.RetentionPolicySegment, error)
}
