package implsentry

import (
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
)

// This file is the PURE query layer of the events plane: every function is a
// bytes-and-args builder with no connection, so the security-critical invariants —
// (1) org_id AND project_id are always the leading, BOUND predicates, (2) the time
// window is always bound, (3) every column/aggregation/interval a client can name is
// resolved through a fixed ALLOWLIST and never interpolated — are exhaustively
// unit-testable in isolation. The IO layer (eventstore.go) only executes what these
// return.

// eventColumns is the allowlist mapping an API field name to its physical column.
// A field NOT in this map is rejected — a client field name never reaches the SQL as
// an identifier. All values are constants defined here, never request data.
var eventColumns = map[string]colKind{
	"timestamp":    {"timestamp", kindTime},
	"level":        {"level", kindString},
	"type":         {"type", kindString},
	"value":        {"value", kindString},
	"message":      {"message", kindString},
	"culprit":      {"culprit", kindString},
	"fingerprint":  {"fingerprint", kindString},
	"platform":     {"platform", kindString},
	"environment":  {"environment", kindString},
	"release":      {"release", kindString},
	"service_name": {"service_name", kindString},
	"transaction":  {"transaction", kindString},
	"trace_id":     {"trace_id", kindString},
	"span_id":      {"span_id", kindString},
	"server_name":  {"server_name", kindString},
	"user_id":      {"user_id", kindString},
	"user_email":   {"user_email", kindString},
}

// eventAggs is the aggregation allowlist. Each value is a FIXED expression with no
// client input, so an aggregation can never carry injected SQL. Keys are the API
// names; the alias is the key itself.
var eventAggs = map[string]colKind{
	"count":        {"count()", kindUint},
	"users":        {"count(DISTINCT user_id)", kindUint},
	"traces":       {"count(DISTINCT trace_id)", kindUint},
	"fingerprints": {"count(DISTINCT fingerprint)", kindUint},
	"first_seen":   {"min(timestamp)", kindTime},
	"last_seen":    {"max(timestamp)", kindTime},
}

// periods maps a relative window token to its duration. Unknown => defaultPeriod.
var periods = map[string]time.Duration{
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"14d": 14 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
	"90d": 90 * 24 * time.Hour,
}

const defaultPeriod = 24 * time.Hour

type kind int

const (
	kindString kind = iota
	kindTime
	kindUint
)

type colKind struct {
	expr string // physical column or fixed aggregation expression
	kind kind
}

// discoverCol names an output column of a Discover result and the kind used to
// allocate its scan target.
type discoverCol struct {
	Name string
	Kind kind
}

const (
	maxDiscoverGroupBy = 6
	maxDiscoverLimit   = 1000
	defaultDiscoverLim = 100
	maxReadLimit       = 1000
	defaultReadLimit   = 100
)

// resolveWindow turns a relative period token into an absolute [from, now] window.
// now is injected so the window is deterministic in tests.
func resolveWindow(period string, now time.Time) sentrytypes.Window {
	d, ok := periods[strings.TrimSpace(period)]
	if !ok {
		d = defaultPeriod
	}
	return sentrytypes.Window{From: now.Add(-d), To: now}
}

// scope is the mandatory (org, project) + window prefix shared by every read: two
// bound tenant predicates FIRST, then the bound time bounds. It returns the WHERE
// fragment and its args in order, so no read can omit the tenant boundary.
func scope(orgID, projectID string, w sentrytypes.Window) (string, []any) {
	return "org_id = ? AND project_id = ? AND timestamp >= ? AND timestamp <= ?",
		[]any{orgID, projectID, w.From, w.To}
}

// buildDiscover assembles the columnar aggregation. Returns the SQL, its bound args,
// and the output column descriptors (for typed scanning). Every groupBy/orderBy field
// resolves through eventColumns and every aggregation through eventAggs; anything else
// is an invalid-input error, never interpolated.
func buildDiscover(db, table, orgID, projectID string, req *sentrytypes.DiscoverRequest, w sentrytypes.Window) (string, []any, []discoverCol, error) {
	if len(req.GroupBy) > maxDiscoverGroupBy {
		return "", nil, nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "too many groupBy fields (max %d)", maxDiscoverGroupBy)
	}

	var selects []string
	var cols []discoverCol
	byAlias := map[string]bool{}

	for _, g := range req.GroupBy {
		ck, ok := eventColumns[g]
		if !ok {
			return "", nil, nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "unknown groupBy field %q", g)
		}
		selects = append(selects, ck.expr+" AS "+g)
		cols = append(cols, discoverCol{Name: g, Kind: ck.kind})
		byAlias[g] = true
	}

	aggs := req.Aggregations
	if len(aggs) == 0 {
		aggs = []string{"count"}
	}
	for _, a := range aggs {
		ck, ok := eventAggs[a]
		if !ok {
			return "", nil, nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "unknown aggregation %q", a)
		}
		selects = append(selects, ck.expr+" AS "+a)
		cols = append(cols, discoverCol{Name: a, Kind: ck.kind})
		byAlias[a] = true
	}

	where, args := scope(orgID, projectID, w)
	filterSQL, filterArgs, err := buildFilters(req.Filters)
	if err != nil {
		return "", nil, nil, err
	}
	where += filterSQL
	args = append(args, filterArgs...)

	var sb strings.Builder
	fmt.Fprintf(&sb, "SELECT %s FROM %s.%s WHERE %s", strings.Join(selects, ", "), db, table, where)
	if len(req.GroupBy) > 0 {
		sb.WriteString(" GROUP BY " + strings.Join(req.GroupBy, ", "))
	}

	if req.OrderBy != "" {
		if !byAlias[req.OrderBy] {
			return "", nil, nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "orderBy %q must be a selected field or aggregation", req.OrderBy)
		}
		dir := "DESC"
		if strings.EqualFold(req.OrderDir, "asc") {
			dir = "ASC"
		}
		sb.WriteString(" ORDER BY " + req.OrderBy + " " + dir)
	}

	sb.WriteString(" LIMIT ?")
	args = append(args, clampLimit(req.Limit, defaultDiscoverLim, maxDiscoverLimit))

	return sb.String(), args, cols, nil
}

// buildFilters turns the equality/like predicates into bound clauses. The field is
// resolved through the allowlist; the value is ALWAYS a bound parameter. Op is a
// fixed set. Returns a leading " AND ..." fragment (possibly empty) and its args.
func buildFilters(filters []sentrytypes.DiscoverFilter) (string, []any, error) {
	var sb strings.Builder
	var args []any
	for _, f := range filters {
		ck, ok := eventColumns[f.Field]
		if !ok {
			return "", nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "unknown filter field %q", f.Field)
		}
		switch strings.ToLower(f.Op) {
		case "", "eq":
			sb.WriteString(" AND " + ck.expr + " = ?")
			args = append(args, f.Value)
		case "neq":
			sb.WriteString(" AND " + ck.expr + " != ?")
			args = append(args, f.Value)
		case "like":
			sb.WriteString(" AND " + ck.expr + " LIKE ?")
			args = append(args, "%"+f.Value+"%")
		default:
			return "", nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "unknown filter op %q (want eq|neq|like)", f.Op)
		}
	}
	return sb.String(), args, nil
}

// selectColumns is the fixed projection used by row reads (event detail / issue
// occurrences / logs). Order MUST match scanEvent in eventstore.go.
const selectColumns = "org_id, project_id, event_id, timestamp, received_at, level, type, value, message, culprit, fingerprint, platform, environment, release, service_name, transaction, trace_id, span_id, server_name, user_id, user_email, user_ip, tags, sample"

func buildGetEvent(db, table, orgID, projectID, eventID string) (string, []any) {
	// Tenant boundary FIRST (org, then project — a project is the DSN-bearing
	// isolation unit), then event id; a foreign org/project/event returns zero rows.
	return fmt.Sprintf("SELECT %s FROM %s.%s WHERE org_id = ? AND project_id = ? AND event_id = ? ORDER BY timestamp DESC LIMIT 1", selectColumns, db, table),
		[]any{orgID, projectID, eventID}
}

func buildListForFingerprint(db, table, orgID, projectID, fingerprint string, limit int) (string, []any) {
	return fmt.Sprintf("SELECT %s FROM %s.%s WHERE org_id = ? AND project_id = ? AND fingerprint = ? ORDER BY timestamp DESC LIMIT ?", selectColumns, db, table),
		[]any{orgID, projectID, fingerprint, clampLimit(limit, defaultReadLimit, maxReadLimit)}
}

// buildListForTrace returns the (org, project)-scoped error events that reference a
// trace id — the tenant-safe "errors in this trace" detail. The trace id is
// attacker-influenced (it comes from the ingested event body), so it is ANDed AFTER
// the bound org+project predicates: a caller only ever sees their OWN project's events
// for a trace, never another tenant's spans (the o11y_traces span plane has no general
// org column and is intentionally NOT read here — see the report).
func buildListForTrace(db, table, orgID, projectID, traceID string, limit int) (string, []any) {
	return fmt.Sprintf("SELECT %s FROM %s.%s WHERE org_id = ? AND project_id = ? AND trace_id = ? ORDER BY timestamp ASC LIMIT ?", selectColumns, db, table),
		[]any{orgID, projectID, traceID, clampLimit(limit, defaultReadLimit, maxReadLimit)}
}

func buildListLogs(db, table, orgID, projectID, query string, w sentrytypes.Window, limit int) (string, []any) {
	where, args := scope(orgID, projectID, w)
	if strings.TrimSpace(query) != "" {
		where += " AND (message LIKE ? OR value LIKE ?)"
		like := "%" + query + "%"
		args = append(args, like, like)
	}
	args = append(args, clampLimit(limit, defaultReadLimit, maxReadLimit))
	return fmt.Sprintf("SELECT %s FROM %s.%s WHERE %s ORDER BY timestamp DESC LIMIT ?", selectColumns, db, table, where), args
}

func buildDistinctFingerprints(db, table, orgID, projectID string, w sentrytypes.Window) (string, []any) {
	where, args := scope(orgID, projectID, w)
	return fmt.Sprintf("SELECT DISTINCT fingerprint FROM %s.%s WHERE %s AND fingerprint != ''", db, table, where), args
}

func buildListTraces(db, table, orgID, projectID string, w sentrytypes.Window, limit int) (string, []any) {
	where, args := scope(orgID, projectID, w)
	args = append(args, clampLimit(limit, defaultReadLimit, maxReadLimit))
	return fmt.Sprintf(
		"SELECT trace_id, count() AS c, min(timestamp) AS f, max(timestamp) AS l, argMax(sample, timestamp) AS s FROM %s.%s WHERE %s AND trace_id != '' GROUP BY trace_id ORDER BY l DESC LIMIT ?",
		db, table, where), args
}

// statsFields is the allowlist selecting the counted subset for the stats timeseries.
// Each value is a fixed WHERE fragment with no client input.
var statsFields = map[string]string{
	"":         "",
	"events":   "",
	"errors":   " AND level IN ('error','fatal')",
	"warnings": " AND level = 'warning'",
}

// buildStats assembles the bucketed event-count timeseries. The bucket width is
// derived from the window span over a FIXED ladder (never client input), so the
// INTERVAL literal is a value this code controls.
func buildStats(db, table, orgID, projectID, field string, w sentrytypes.Window) (string, []any, error) {
	extra, ok := statsFields[strings.ToLower(strings.TrimSpace(field))]
	if !ok {
		return "", nil, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "unknown stats field %q", field)
	}
	bucket := bucketSeconds(w.To.Sub(w.From))
	where, args := scope(orgID, projectID, w)
	// bucket is a controlled int (bucketSeconds ladder), safe to format as the INTERVAL.
	sql := fmt.Sprintf(
		"SELECT toStartOfInterval(timestamp, INTERVAL %d SECOND) AS bucket, count() AS c FROM %s.%s WHERE %s%s GROUP BY bucket ORDER BY bucket ASC",
		bucket, db, table, where, extra)
	return sql, args, nil
}

// bucketSeconds picks a bucket width from a fixed ladder so a timeseries has a
// reasonable number of points regardless of the window.
func bucketSeconds(span time.Duration) int {
	switch {
	case span <= 2*time.Hour:
		return 60 // 1m
	case span <= 24*time.Hour:
		return 3600 // 1h
	case span <= 7*24*time.Hour:
		return 6 * 3600 // 6h
	default:
		return 24 * 3600 // 1d
	}
}

func clampLimit(limit, def, max int) int {
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}
