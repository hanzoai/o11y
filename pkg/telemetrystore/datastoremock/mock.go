// Package datastoremock exposes the datastore mock connection surface behind a
// brand-neutral interface, so o11y test doubles depend on a local Datastore
// contract rather than the mock driver's exported type name.
package datastoremock

import dsmock "github.com/hanzo-ds/mock"

// Conn is the subset of the datastore mock driver used to set query
// expectations in tests. The mock connection satisfies it structurally.
type Conn interface {
	ExpectQuery(sql string) *dsmock.ExpectedQuery
	ExpectQueryRow(sql string) *dsmock.ExpectedQueryRow
	ExpectSelect(sql string) *dsmock.ExpectedSelect
	ExpectExec(sql string) *dsmock.ExpectedExec
	ExpectPrepareBatch(sql string) *dsmock.ExpectedPrepareBatch
	ExpectationsWereMet() error
	MatchExpectationsInOrder(bool)
}
