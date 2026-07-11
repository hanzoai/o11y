package implsentry

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlitesqlstore"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSQLStore builds a real sqlite store with the o11y_sentry_projects table and
// its (org_id, slug) unique index — the shape the migration ships.
func newTestSQLStore(t *testing.T) sqlstore.SQLStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlitesqlstore.New(context.Background(), factorytest.NewSettings(), sqlstore.Config{
		Provider: "sqlite",
		Connection: sqlstore.ConnectionConfig{
			MaxOpenConns:    1,
			MaxConnLifetime: 0,
		},
		Sqlite: sqlstore.SqliteConfig{
			Path:            dbPath,
			Mode:            "wal",
			BusyTimeout:     5 * time.Second,
			TransactionMode: "deferred",
		},
	})
	require.NoError(t, err)

	_, err = store.BunDB().NewCreateTable().Model((*sentrytypes.Project)(nil)).IfNotExists().Exec(context.Background())
	require.NoError(t, err)
	_, err = store.BunDB().Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_o11y_sentry_projects_org_slug ON o11y_sentry_projects (org_id, slug)`)
	require.NoError(t, err)
	return store
}

func newProject(orgID valuer.UUID, name, slug string) *sentrytypes.Project {
	now := time.Now().UTC()
	return &sentrytypes.Project{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		Name:          name,
		Slug:          slug,
		Status:        sentrytypes.ProjectActive,
		KeyVersion:    1,
	}
}

func TestProjectStore_CRUDAndRotate(t *testing.T) {
	ctx := context.Background()
	store := NewProjectStore(newTestSQLStore(t))
	org := valuer.GenerateUUID()

	p := newProject(org, "Web", "web")
	require.NoError(t, store.Create(ctx, p))

	got, err := store.Get(ctx, org, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "web", got.Slug)
	assert.Equal(t, 1, got.KeyVersion)

	list, err := store.List(ctx, org)
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Rotate bumps the key watermark.
	v, err := store.Rotate(ctx, org, p.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, v)
	got, err = store.Get(ctx, org, p.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.KeyVersion)

	// Resolve (ingest path) returns the owning org + watermark.
	rOrg, ver, status, found, err := store.Resolve(ctx, p.ID)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, org, rOrg)
	assert.Equal(t, 2, ver)
	assert.Equal(t, sentrytypes.ProjectActive, status)
}

// TestProjectStore_TenantIsolation is the mandatory two-org isolation test: org B can
// neither read nor rotate org A's project, and each org lists only its own.
func TestProjectStore_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	store := NewProjectStore(newTestSQLStore(t))
	orgA, orgB := valuer.GenerateUUID(), valuer.GenerateUUID()

	pa := newProject(orgA, "A app", "a-app")
	pb := newProject(orgB, "B app", "b-app")
	require.NoError(t, store.Create(ctx, pa))
	require.NoError(t, store.Create(ctx, pb))

	// org B cannot GET org A's project — foreign id is not found in B's scope.
	_, err := store.Get(ctx, orgB, pa.ID)
	require.Error(t, err)

	// org B cannot ROTATE org A's project.
	_, err = store.Rotate(ctx, orgB, pa.ID)
	require.Error(t, err)
	// ...and org A's key is untouched by B's attempt.
	stillA, err := store.Get(ctx, orgA, pa.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stillA.KeyVersion)

	// Each org lists only its own.
	la, _ := store.List(ctx, orgA)
	lb, _ := store.List(ctx, orgB)
	require.Len(t, la, 1)
	require.Len(t, lb, 1)
	assert.Equal(t, pa.ID, la[0].ID)
	assert.Equal(t, pb.ID, lb[0].ID)
}

func TestProjectStore_ResolveUnknownFailsClosed(t *testing.T) {
	ctx := context.Background()
	store := NewProjectStore(newTestSQLStore(t))
	_, _, _, found, err := store.Resolve(ctx, valuer.GenerateUUID())
	require.NoError(t, err)
	assert.False(t, found, "an unknown project must resolve to found=false, never a tenant")
}
