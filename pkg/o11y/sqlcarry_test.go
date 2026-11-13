package o11y

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// A sqlite control plane moves to postgres whole: every shared table's rows, typed for
// the target, counted on both sides, with the target's own seed rows replaced.
func TestCarrySQLiteToPostgres(t *testing.T) {
	ctx := context.Background()
	all := engines()
	lite, _ := all[0].open(t)
	pg, _ := all[1].open(t)
	settings := factorytest.NewSettings()

	require.NoError(t, MigrateSQL(ctx, settings, lite.SQLDB(), false))
	require.NoError(t, MigrateSQL(ctx, settings, pg.SQLDB(), true))

	org := types.NewOrganization("hanzo", "hanzo")
	_, err := lite.BunDB().NewInsert().Model(org).Exec(ctx)
	require.NoError(t, err)
	user, err := types.NewUserWithID(valuer.GenerateUUID(), "z", valuer.MustNewEmail("z@hanzo.ai"), org.ID, types.UserStatusActive)
	require.NoError(t, err)
	_, err = lite.BunDB().NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	_, err = lite.SQLDB().ExecContext(ctx, `INSERT INTO user_preference (id, name, value, user_id) VALUES (?, 'theme', 'dark', ?)`, valuer.GenerateUUID().StringValue(), user.ID.StringValue())
	require.NoError(t, err)

	// a row the target would otherwise keep
	stray := types.NewOrganization("stray", "stray")
	_, err = pg.BunDB().NewInsert().Model(stray).Exec(ctx)
	require.NoError(t, err)

	carried, err := CarrySQLite(ctx, lite.SQLDB(), pg.SQLDB())
	require.NoError(t, err)
	require.EqualValues(t, 1, carried["organizations"])
	require.EqualValues(t, 1, carried["users"])
	require.EqualValues(t, 1, carried["user_preference"])

	var name string
	var created string
	require.NoError(t, pg.SQLDB().QueryRowContext(ctx, `SELECT display_name, created_at::text FROM users WHERE id = $1`, user.ID.StringValue()).Scan(&name, &created))
	require.Equal(t, "z", name)
	require.NotEmpty(t, created)
	var orgs int
	require.NoError(t, pg.SQLDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM organizations WHERE name = 'stray'`).Scan(&orgs))
	require.Zero(t, orgs)

	// the carried store is a working one: a second carry of the same rows is the same store
	again, err := CarrySQLite(ctx, lite.SQLDB(), pg.SQLDB())
	require.NoError(t, err)
	require.Equal(t, carried, again)
}
