package sqlmigration

import (
	"context"
	"time"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/migrate"
)

type addServiceAccountManagedRoleTransactions struct {
	sqlstore sqlstore.SQLStore
}

func NewAddServiceAccountManagedRoleTransactionsFactory(sqlstore sqlstore.SQLStore) factory.ProviderFactory[SQLMigration, Config] {
	return factory.NewProviderFactory(factory.MustNewName("add_sa_managed_role_txn"), func(ctx context.Context, ps factory.ProviderSettings, c Config) (SQLMigration, error) {
		return &addServiceAccountManagedRoleTransactions{sqlstore: sqlstore}, nil
	})
}

func (migration *addServiceAccountManagedRoleTransactions) Register(migrations *migrate.Migrations) error {
	return migrations.Register(migration.Up, migration.Down)
}

// managedRoleTuple describes a single FGA tuple to insert for a managed role.
type managedRoleTuple struct {
	roleName   string
	objectType string // "metaresources" or "metaresource"
	objectName string // "service-accounts" or "service-account"
	relation   string // "create", "list", "read", "update", "delete"
}

func (migration *addServiceAccountManagedRoleTransactions) Up(ctx context.Context, db *bun.DB) error {
	// All tuples that need to be created for service account FGA managed role permissions.
	tuples := []managedRoleTuple{
		{authtypes.HanzoO11yAdminRoleName, "role", "role", "attach"},
		{authtypes.HanzoO11yAdminRoleName, "serviceaccount", "serviceaccount", "attach"},
		{authtypes.HanzoO11yAdminRoleName, "metaresources", "serviceaccount", "create"},
		{authtypes.HanzoO11yAdminRoleName, "metaresources", "serviceaccount", "list"},
		{authtypes.HanzoO11yAdminRoleName, "serviceaccount", "serviceaccount", "read"},
		{authtypes.HanzoO11yAdminRoleName, "serviceaccount", "serviceaccount", "update"},
		{authtypes.HanzoO11yAdminRoleName, "serviceaccount", "serviceaccount", "delete"},
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var storeID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM store WHERE name = ? LIMIT 1`, "signoz").Scan(&storeID)
	if err != nil {
		return err
	}

	// Fetch all orgs.
	var orgIDs []string
	rows, err := tx.QueryContext(ctx, `SELECT id FROM organizations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			return err
		}
		orgIDs = append(orgIDs, orgID)
	}

	isPG := migration.sqlstore.BunDB().Dialect().Name() == dialect.PG

	for _, orgID := range orgIDs {
		for _, tuple := range tuples {
			entropy := ulid.DefaultEntropy()
			now := time.Now().UTC()
			tupleID := ulid.MustNew(ulid.Timestamp(now), entropy).String()

			objectID := "organization/" + orgID + "/" + tuple.objectName + "/*"
			roleSubject := "organization/" + orgID + "/role/" + tuple.roleName

			if isPG {
				user := "role:" + roleSubject + "#assignee"
				result, err := tx.ExecContext(ctx, `
					INSERT INTO tuple (store, object_type, object_id, relation, _user, user_type, ulid, inserted_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT (store, object_type, object_id, relation, _user) DO NOTHING`,
					storeID, tuple.objectType, objectID, tuple.relation, user, "userset", tupleID, now,
				)
				if err != nil {
					return err
				}
				rowsAffected, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if rowsAffected == 0 {
					continue
				}
				_, err = tx.ExecContext(ctx, `
					INSERT INTO changelog (store, object_type, object_id, relation, _user, operation, ulid, inserted_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT (store, ulid, object_type) DO NOTHING`,
					storeID, tuple.objectType, objectID, tuple.relation, user, "TUPLE_OPERATION_WRITE", tupleID, now,
				)
				if err != nil {
					return err
				}
			} else {
				result, err := tx.ExecContext(ctx, `
					INSERT INTO tuple (store, object_type, object_id, relation, user_object_type, user_object_id, user_relation, user_type, ulid, inserted_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT (store, object_type, object_id, relation, user_object_type, user_object_id, user_relation) DO NOTHING`,
					storeID, tuple.objectType, objectID, tuple.relation, "role", roleSubject, "assignee", "userset", tupleID, now,
				)
				if err != nil {
					return err
				}
				rowsAffected, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if rowsAffected == 0 {
					continue
				}
				_, err = tx.ExecContext(ctx, `
					INSERT INTO changelog (store, object_type, object_id, relation, user_object_type, user_object_id, user_relation, operation, ulid, inserted_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT (store, ulid, object_type) DO NOTHING`,
					storeID, tuple.objectType, objectID, tuple.relation, "role", roleSubject, "assignee", 0, tupleID, now,
				)
				if err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func (migration *addServiceAccountManagedRoleTransactions) Down(context.Context, *bun.DB) error {
	return nil
}
