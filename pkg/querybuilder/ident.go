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
// The escape is the datastore's own: inside a backtick-quoted identifier, a
// backslash escapes the next character, so a backslash must be doubled BEFORE a
// backtick is escaped — otherwise a trailing backslash in the name would eat the
// closing backtick and leave the identifier open. Order matters and is the
// reason this is one function rather than two Replace calls at 40 call sites.
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
// Same ordering rule as QuoteIdent: backslash first, then the quote.
func EscapeLiteral(s string) string {
	return literalEscaper.Replace(s)
}

var (
	identEscaper   = strings.NewReplacer(`\`, `\\`, "`", "\\`")
	literalEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
)
