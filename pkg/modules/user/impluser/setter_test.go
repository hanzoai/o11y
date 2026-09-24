package impluser

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/analytics/analyticstest"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlitesqlstore"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grantRecorder is the authz the setter moves grants through. Only Grant and
// Revoke are reached by RekeyUser; any other call panics on the nil interface.
type grantRecorder struct {
	authz.AuthZ
	granted map[string][]string
	revoked map[string][]string
}

func (g *grantRecorder) Grant(_ context.Context, _ valuer.UUID, names []string, subject string) error {
	g.granted[subject] = append(g.granted[subject], names...)
	return nil
}

func (g *grantRecorder) Revoke(_ context.Context, _ valuer.UUID, names []string, subject string) error {
	g.revoked[subject] = append(g.revoked[subject], names...)
	return nil
}

// The schema the migrations leave these tables in: users unique on (email,
// org_id) among the live rows (076), and user_role (071) and user_preference
// (055) pointing at users.id with no ON UPDATE action — so a plain UPDATE of the
// key is refused while either child still points at it.
const rekeySchema = `
CREATE TABLE users (
	id TEXT PRIMARY KEY, display_name TEXT NOT NULL, email TEXT NOT NULL, org_id TEXT NOT NULL,
	is_root BOOLEAN NOT NULL DEFAULT false, status TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL);
CREATE UNIQUE INDEX uq_users_email_org_id ON users (email, org_id) WHERE status != 'deleted';
CREATE TABLE role (
	id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL, type TEXT NOT NULL, org_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL);
CREATE TABLE user_role (
	id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users (id), role_id TEXT NOT NULL REFERENCES role (id),
	created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL);
CREATE UNIQUE INDEX uq_user_role_user_id_role_id ON user_role (user_id, role_id);
CREATE TABLE user_preference (
	id TEXT PRIMARY KEY, name TEXT NOT NULL, value TEXT NOT NULL, user_id TEXT NOT NULL REFERENCES users (id));
`

type rekeyFixture struct {
	sql     sqlstore.SQLStore
	setter  *setter
	grants  *grantRecorder
	orgID   valuer.UUID
	email   valuer.Email
	byName  valuer.UUID
	subject valuer.UUID
}

func newRekeyFixture(t *testing.T) *rekeyFixture {
	t.Helper()
	ctx := context.Background()

	sql, err := sqlitesqlstore.New(ctx, factorytest.NewSettings(), sqlstore.Config{
		Provider:   "sqlite",
		Connection: sqlstore.ConnectionConfig{MaxOpenConns: 1},
		Sqlite: sqlstore.SqliteConfig{
			Path:            filepath.Join(t.TempDir(), "o11y.db"),
			Mode:            "wal",
			BusyTimeout:     5 * time.Second,
			TransactionMode: "deferred",
		},
	})
	require.NoError(t, err)
	_, err = sql.SQLDB().ExecContext(ctx, rekeySchema)
	require.NoError(t, err)

	f := &rekeyFixture{
		sql:     sql,
		grants:  &grantRecorder{granted: map[string][]string{}, revoked: map[string][]string{}},
		orgID:   valuer.GenerateUUID(),
		email:   valuer.MustNewEmail("z@hanzo.ai"),
		byName:  valuer.MustNewUUID("7b05517d-c5bc-553a-9250-b4831ebd8266"),
		subject: valuer.MustNewUUID("2d4d67ab-30f1-474e-b81f-f60461852259"),
	}
	f.setter = NewSetter(NewStore(sql, factorytest.NewSettings()), factorytest.NewSettings(), f.grants, analyticstest.New(), NewUserRoleStore(sql, factorytest.NewSettings())).(*setter)

	now := time.Now()
	u, err := types.NewUserWithID(f.byName, "z", f.email, f.orgID, types.UserStatusActive)
	require.NoError(t, err)
	_, err = sql.BunDB().NewInsert().Model(u).Exec(ctx)
	require.NoError(t, err)

	role := &authtypes.Role{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		Name:          authtypes.O11yAdminRoleName,
		Type:          valuer.NewString("managed"),
		OrgID:         f.orgID,
	}
	_, err = sql.BunDB().NewInsert().Model(role).Exec(ctx)
	require.NoError(t, err)
	_, err = sql.BunDB().NewInsert().Model(&authtypes.UserRole{
		ID: valuer.GenerateUUID(), UserID: f.byName, RoleID: role.ID, CreatedAt: now, UpdatedAt: now,
	}).Exec(ctx)
	require.NoError(t, err)
	_, err = sql.SQLDB().ExecContext(ctx, `INSERT INTO user_preference (id, name, value, user_id) VALUES (?, 'theme', 'dark', ?)`, valuer.GenerateUUID().StringValue(), f.byName.StringValue())
	require.NoError(t, err)

	return f
}

func (f *rekeyFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, f.sql.SQLDB().QueryRowContext(context.Background(), query, args...).Scan(&n))
	return n
}

func subjectOf(orgID, userID valuer.UUID) string {
	return authtypes.MustNewSubject(coretypes.NewResourceUser(), userID.StringValue(), orgID, nil)
}

// The row, its user_role row and every other row keyed to the person move to the
// IAM subject together, and the role grant moves with them.
func TestRekeyUser_MovesTheRowAndEverythingKeyedToIt(t *testing.T) {
	f := newRekeyFixture(t)

	require.NoError(t, f.setter.RekeyUser(context.Background(), f.orgID, f.email, f.subject))

	got, err := f.setter.store.GetByOrgIDAndID(context.Background(), f.orgID, f.subject)
	require.NoError(t, err)
	assert.Equal(t, f.email, got.Email)
	for _, table := range []string{"users", "user_role", "user_preference"} {
		column := "user_id"
		if table == "users" {
			column = "id"
		}
		assert.Equal(t, 0, f.count(t, `SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`, f.byName.StringValue()), table+" still keyed by the old id")
		assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`, f.subject.StringValue()), table+" not keyed by the subject")
	}
	assert.Equal(t, 0, f.count(t, `SELECT COUNT(*) FROM pragma_foreign_key_check`))

	assert.Equal(t, []string{authtypes.O11yAdminRoleName}, f.grants.granted[subjectOf(f.orgID, f.subject)])
	assert.Equal(t, []string{authtypes.O11yAdminRoleName}, f.grants.revoked[subjectOf(f.orgID, f.byName)])
}

// A row already keyed by the subject — a concurrent request won the create — is
// left as it is, and no grant moves.
func TestRekeyUser_RowAlreadyKeyedBySubjectIsLeftAlone(t *testing.T) {
	f := newRekeyFixture(t)

	require.NoError(t, f.setter.RekeyUser(context.Background(), f.orgID, f.email, f.byName))

	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, f.byName.StringValue()))
	assert.Empty(t, f.grants.granted)
	assert.Empty(t, f.grants.revoked)
}

// One transaction: when the move cannot finish, nothing has moved.
func TestRekeyUser_IsOneTransaction(t *testing.T) {
	f := newRekeyFixture(t)
	other, err := types.NewUserWithID(f.subject, "someone else", valuer.MustNewEmail("other@hanzo.ai"), f.orgID, types.UserStatusActive)
	require.NoError(t, err)
	_, err = f.sql.BunDB().NewInsert().Model(other).Exec(context.Background())
	require.NoError(t, err)

	require.Error(t, f.setter.RekeyUser(context.Background(), f.orgID, f.email, f.subject))

	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM users WHERE id = ? AND email = ?`, f.byName.StringValue(), f.email.String()))
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM user_role WHERE user_id = ?`, f.byName.StringValue()))
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM user_preference WHERE user_id = ?`, f.byName.StringValue()))
	assert.Empty(t, f.grants.granted)
}
