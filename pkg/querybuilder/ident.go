package querybuilder

import "strings"

// A telemetry field key NAME is caller text. It arrives as JSON on
// /api/v5/query_range, and it is replayed out of stored dashboards and alert
// rules — the rule evaluator builds a QueryRangeRequest and calls the querier
// directly, so a name on that path never passes the request validator at all.
//
// It lands in two syntactic positions, and a bound parameter can occupy
// NEITHER, so quoting is the whole defence:
//
//   - a backtick-quoted IDENTIFIER — the SELECT alias, the GROUP BY term, the
//     ORDER BY term;
//   - a single-quoted string LITERAL — the map/JSON accessors that read an
//     attribute out of a column, e.g. JSONExtractString(labels, '<name>').
//
// Metrics is the position that proves this has to live here rather than in the
// validator: a metric label is free-form, so an unrecognised name falls back to
// the labels column BY DESIGN and there is no key allowlist to fail against.
//
// These two are the only way a name becomes SQL text. Nothing else quoted it —
// sqlbuilder.Escape, the one call that used to sit next to a name, replaces "$"
// with "$$" for the builder's own placeholder syntax and is not SQL escaping.

// QuoteIdent renders a name as a backtick-quoted datastore identifier.
//
// The escape is the datastore's own, and the authority for it is the driver:
// hanzo-ds/go lib/column/column.go carries the same pair as colEscape /
// colUnEscape. Inside a backtick-quoted identifier a backslash escapes the next
// character, so BOTH the backtick and the backslash have to be escaped — the
// backslash because a name ending in one would otherwise consume the closing
// backtick this function adds and leave the identifier open.
//
// It is a single-pass Replacer on purpose. The two patterns do not overlap, so
// one pass is order-independent and each input byte is rewritten exactly once.
// Two sequential ReplaceAll calls would NOT be safe — escaping the backtick
// first and the backslash second would then double the backslashes the first
// call had just introduced — so do not unroll this into ReplaceAll.
//
// A name with none of those characters — every real attribute key, which is a
// dotted identifier — renders byte for byte as before.
func QuoteIdent(name string) string {
	return "`" + identEscaper.Replace(name) + "`"
}

// EscapeLiteral escapes a name for use INSIDE an existing pair of single quotes,
// e.g. fmt.Sprintf("JSONExtractString(labels, '%s')", EscapeLiteral(key.Name)).
// It does not add the quotes, because every call site already writes them as
// part of a larger expression.
//
// Same construction as QuoteIdent, and single-pass for the same reason: the
// quote and the backslash do not overlap, so each byte is rewritten once.
func EscapeLiteral(s string) string {
	return literalEscaper.Replace(s)
}

var (
	identEscaper   = strings.NewReplacer(`\`, `\\`, "`", "\\`")
	literalEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
)
