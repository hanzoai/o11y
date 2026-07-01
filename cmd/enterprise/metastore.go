package main

import (
	"github.com/hanzoai/o11y/ee/sqlschema/postgressqlschema"
	"github.com/hanzoai/o11y/ee/sqlstore/postgressqlstore"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/signoz"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstorehook"
)

func sqlstoreProviderFactories() factory.NamedMap[factory.ProviderFactory[sqlstore.SQLStore, sqlstore.Config]] {
	existingFactories := signoz.NewSQLStoreProviderFactories()
	if err := existingFactories.Add(postgressqlstore.NewFactory(sqlstorehook.NewLoggingFactory(), sqlstorehook.NewInstrumentationFactory())); err != nil {
		panic(err)
	}

	return existingFactories
}

func sqlschemaProviderFactories(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
	existingFactories := signoz.NewSQLSchemaProviderFactories(sqlstore)
	if err := existingFactories.Add(postgressqlschema.NewFactory(sqlstore)); err != nil {
		panic(err)
	}

	return existingFactories
}
