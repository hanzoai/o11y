// Package datastoresql binds the SQL dialect spoken by the telemetry datastore.
package datastoresql

import "github.com/hanzoai/sqlbuilder"

// Flavor is the SQL dialect every statement builder renders through. It is the
// one place first-party code names the upstream dialect selector.
const Flavor = sqlbuilder.Datastore
