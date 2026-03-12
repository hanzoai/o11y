package sqlmigration

import (
	"context"
	"database/sql"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/roletypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

type addManagedRoles struct {
	sqlstore  sqlstore.SQLStore
	sqlschema sqlschema.SQLSchema
}

func NewAddManagedRolesFactory(sqlstore sqlstore.SQLStore, sqlschema sqlschema.SQLSchema) factory.ProviderFactory[SQLMigration, Config] {
	return factory.NewProviderFactory(factory.MustNewName("add_managed_roles"), func(ctx context.Context, ps factory.ProviderSettings, c Config) (SQLMigration, error) {
		return newAddManagedRoles(ctx, ps, c, sqlstore, sqlschema)
	})
}

func newAddManagedRoles(_ context.Context, _ factory.ProviderSettings, _ Config, sqlStore sqlstore.SQLStore, sqlSchema sqlschema.SQLSchema) (SQLMigration, error) {
	return &addManagedRoles{sqlstore: sqlStore, sqlschema: sqlSchema}, nil
}

func (migration *addManagedRoles) Register(migrations *migrate.Migrations) error {
	if err := migrations.Register(migration.Up, migration.Down); err != nil {
		return err
	}
	return nil
}

func (migration *addManagedRoles) Up(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	var orgIDs []string
	err = tx.NewSelect().
		Table("organizations").
		Column("id").
		Scan(ctx, &orgIDs)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	managedRoles := []*roletypes.StorableRole{}
	for _, orgIDStr := range orgIDs {
		orgID, err := valuer.NewUUID(orgIDStr)
		if err != nil {
			return err
		}

		// o11y admin
		o11yAdminRole := roletypes.NewRole(roletypes.HanzoO11yAdminRoleName, roletypes.HanzoO11yAdminRoleDescription, roletypes.RoleTypeManaged, orgID)
		managedRoles = append(managedRoles, roletypes.NewStorableRoleFromRole(o11yAdminRole))

		// o11y editor
		o11yEditorRole := roletypes.NewRole(roletypes.HanzoO11yEditorRoleName, roletypes.HanzoO11yEditorRoleDescription, roletypes.RoleTypeManaged, orgID)
		managedRoles = append(managedRoles, roletypes.NewStorableRoleFromRole(o11yEditorRole))

		// o11y viewer
		o11yViewerRole := roletypes.NewRole(roletypes.HanzoO11yViewerRoleName, roletypes.HanzoO11yViewerRoleDescription, roletypes.RoleTypeManaged, orgID)
		managedRoles = append(managedRoles, roletypes.NewStorableRoleFromRole(o11yViewerRole))

		// o11y anonymous
		o11yAnonymousRole := roletypes.NewRole(roletypes.HanzoO11yAnonymousRoleName, roletypes.HanzoO11yAnonymousRoleDescription, roletypes.RoleTypeManaged, orgID)
		managedRoles = append(managedRoles, roletypes.NewStorableRoleFromRole(o11yAnonymousRole))
	}

	if len(managedRoles) > 0 {
		_, err = tx.NewInsert().
			Model(&managedRoles).
			On("CONFLICT (org_id, name) DO UPDATE").
			Set("description = EXCLUDED.description, type = EXCLUDED.type, updated_at = EXCLUDED.updated_at").
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (migration *addManagedRoles) Down(_ context.Context, _ *bun.DB) error {
	return nil
}
