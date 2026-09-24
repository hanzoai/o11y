// Package migrations is the warehouse schema o11y owns, as the ordered statements
// that built it: NNNN_name.sql, applied in version order, each at most once.
//
// The files are the schema's one statement. The process that owns the schema at
// runtime applies them when it starts — hanzoai/cloud's o11y app, through
// cloud/datastore.Migrate, under a lock that lets one replica run them and a
// ledger that records what ran — so a migration merged here is live on the next
// deploy, with no one to remember to run it.
package migrations

import "embed"

// FS is every migration, by file name.
//
//go:embed *.sql
var FS embed.FS

// Baseline is the last migration that was applied by hand, before anything
// recorded what had run. 0001 renamed the SigNoz databases, 0002 moved the old
// trace tables aside and created event.fact, 0003 promoted the org-first sample
// tables: each is the cutover of one live warehouse, guarded to refuse any other
// state, and none can run a second time. A ledger that starts empty records them
// as applied without running them; every migration after Baseline runs.
const Baseline = 3
