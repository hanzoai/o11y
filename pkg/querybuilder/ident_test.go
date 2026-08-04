package querybuilder

import (
	"strings"
	"testing"
)

// The datastore driver is the authority on this escape: hanzo-ds/go
// lib/column/column.go spells the same pair as colEscape, and colUnEscape is its
// inverse. These hold QuoteIdent to it, so the day the driver's dialect changes
// this fails rather than silently emitting an identifier the server reads
// differently than we wrote it.
var (
	driverColEscape   = strings.NewReplacer("`", "\\`", "\\", "\\\\")
	driverColUnEscape = strings.NewReplacer("\\`", "`", "\\\\", "\\")
)

func TestQuoteIdentMatchesTheDriverEscape(t *testing.T) {
	for _, name := range []string{
		"le", "service.name", "http.status_code", "k8s.pod.name",
		"le`", `le\`, "a`b\\c", "`", `\`, "``", `\\`,
		"x`,(select 1)--", `tail\`, "",
	} {
		got := QuoteIdent(name)
		want := "`" + driverColEscape.Replace(name) + "`"
		if got != want {
			t.Errorf("QuoteIdent(%q) = %q, driver would write %q", name, got, want)
		}
		// The driver's own inverse must recover the name exactly, which is what
		// makes the escape lossless rather than merely safe.
		if back := driverColUnEscape.Replace(strings.TrimSuffix(strings.TrimPrefix(got, "`"), "`")); back != name {
			t.Errorf("QuoteIdent(%q) did not round-trip through the driver: got %q", name, back)
		}
	}
}

// An ordinary attribute key must render with no escaping at all — that is why
// the golden SQL in every signal package is unchanged by the quoting.
func TestQuoteIdentLeavesOrdinaryNamesAlone(t *testing.T) {
	for _, name := range []string{"le", "service.name", "http.status_code", "k8s.pod.name"} {
		if got := QuoteIdent(name); got != "`"+name+"`" {
			t.Errorf("QuoteIdent(%q) = %q, want it unescaped", name, got)
		}
	}
}

func TestEscapeLiteralClosesNothing(t *testing.T) {
	for _, s := range []string{"a'", `a\`, "a'),(select 1)--", "'", `\`} {
		got := EscapeLiteral(s)
		// Walk the escaped form the way the server does: a backslash consumes the
		// next byte. No bare quote may survive, or the literal ends early.
		for i := 0; i < len(got); i++ {
			if got[i] == '\\' {
				i++
				continue
			}
			if got[i] == '\'' {
				t.Errorf("EscapeLiteral(%q) = %q leaves a bare quote at %d", s, got, i)
			}
		}
	}
}
