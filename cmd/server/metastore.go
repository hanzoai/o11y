package main

import (
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
)

func sqlstoreProviderFactories() factory.NamedMap[factory.ProviderFactory[sqlstore.SQLStore, sqlstore.Config]] {
	return o11y.NewSQLStoreProviderFactories()
}

func sqlschemaProviderFactories(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
	return o11y.NewSQLSchemaProviderFactories(sqlstore)
}
