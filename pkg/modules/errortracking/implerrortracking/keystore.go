package implerrortracking

import (
	"context"
	"sync"
	"time"

	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

// RevocationStore resolves an org's minimum-acceptable DSN key version, enabling
// PER-ORG key rotation without a global secret roll: bump ONE org's min-version and
// only that org's older-version DSNs stop verifying. Default 0 = no revocation.
type RevocationStore interface {
	MinVersion(ctx context.Context, orgID valuer.UUID) int
}

// NoopRevocations never revokes (every org's min-version is 0). Used where key
// rotation state isn't wired (tests, standalone).
type NoopRevocations struct{}

func (NoopRevocations) MinVersion(context.Context, valuer.UUID) int { return 0 }

const revocationCacheTTL = 30 * time.Second

// ingestRevocation is one org's rotation watermark.
type ingestRevocation struct {
	bun.BaseModel `bun:"table:o11y_ingest_revocations,alias:o11y_ingest_revocations"`

	OrgID      valuer.UUID `bun:"org_id,pk,type:text"`
	MinVersion int64       `bun:"min_version,notnull,default:0"`
	UpdatedAt  time.Time   `bun:"updated_at,notnull"`
}

// sqlRevocations is the table-backed store with a wholesale in-memory cache (the
// table is small — one row per rotated org). A read miss or TTL lapse reloads the
// whole table; on a load error the last-good cache is kept (fail-open to the last
// known state, never crash the hot ingest path).
type sqlRevocations struct {
	sqlstore sqlstore.SQLStore
	ttl      time.Duration

	mu       sync.RWMutex
	cache    map[string]int
	loadedAt time.Time
}

func NewSQLRevocations(sqlstore sqlstore.SQLStore) *sqlRevocations {
	return &sqlRevocations{sqlstore: sqlstore, ttl: revocationCacheTTL, cache: map[string]int{}}
}

func (r *sqlRevocations) MinVersion(ctx context.Context, orgID valuer.UUID) int {
	r.mu.RLock()
	fresh := !r.loadedAt.IsZero() && time.Since(r.loadedAt) < r.ttl
	v := r.cache[orgID.String()]
	r.mu.RUnlock()
	if fresh {
		return v
	}
	r.reload(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache[orgID.String()]
}

func (r *sqlRevocations) reload(ctx context.Context) {
	var rows []ingestRevocation
	if err := r.sqlstore.BunDBCtx(ctx).NewSelect().Model(&rows).Scan(ctx); err != nil {
		return // keep stale cache on error
	}
	m := make(map[string]int, len(rows))
	for _, row := range rows {
		m[row.OrgID.String()] = int(row.MinVersion)
	}
	r.mu.Lock()
	r.cache = m
	r.loadedAt = time.Now()
	r.mu.Unlock()
}

// Rotate raises an org's minimum key version, revoking every DSN issued below it.
// The operator then mints a fresh DSN at the new version.
func (r *sqlRevocations) Rotate(ctx context.Context, orgID valuer.UUID, minVersion int) error {
	rev := &ingestRevocation{OrgID: orgID, MinVersion: int64(minVersion), UpdatedAt: time.Now().UTC()}
	_, err := r.sqlstore.BunDBCtx(ctx).
		NewInsert().
		Model(rev).
		On("CONFLICT (org_id) DO UPDATE").
		Set("min_version = EXCLUDED.min_version").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	r.mu.Lock()
	r.loadedAt = time.Time{} // force reload on next read
	r.mu.Unlock()
	return err
}
