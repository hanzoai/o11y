package telemetrylogs

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/datastoresql"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/flagger/flaggertest"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/querybuilder"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/stretchr/testify/require"
)

// detailContains reports whether any additional detail on err (its message or one
// of its suggestions) contains sub.
func detailContains(err error, sub string) bool {
	for _, e := range errors.AsJSON(err).Errors {
		if strings.Contains(e.Message, sub) {
			return true
		}
		for _, s := range e.Suggestions {
			if strings.Contains(s, sub) {
				return true
			}
		}
	}
	return false
}

// TestFilterExprLogs tests a comprehensive set of query patterns for logs search.
func TestFilterExprLogs(t *testing.T) {
	fl := flaggertest.New(t)
	releaseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	fm := NewFieldMapper(fl)
	cb := NewConditionBuilder(fm, fl)

	// Define a comprehensive set of field keys to support all test cases
	keys := buildCompleteFieldKeyMap(releaseTime)

	opts := querybuilder.FilterExprVisitorOpts{
		Context:          ctx,
		Logger:           instrumentationtest.New().Logger(),
		FieldMapper:      fm,
		ConditionBuilder: cb,
		FieldKeys:        keys,
		FullTextColumn:   DefaultFullTextColumn,
		JsonKeyToKey:     GetBodyJSONKey,
		StartNs:          uint64(releaseTime.Add(-5 * time.Minute).UnixNano()),
		EndNs:            uint64(releaseTime.Add(5 * time.Minute).UnixNano()),
	}

	testCases := []struct {
		category              string
		query                 string
		shouldPass            bool
		expectedQuery         string
		expectedArgs          []any
		expectedErrorContains string
	}{
		// Single word searches
		{
			category:              "Single word",
			query:                 "Download",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"Download"},
			expectedErrorContains: "",
		},
		{
			category:              "Single word invalid regex",
			query:                 "'[LocalLog partition=__cluster_metadata-0,'",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"\\[LocalLog partition=__cluster_metadata-0,"},
			expectedErrorContains: "",
		},
		{
			category:              "Single word",
			query:                 "LAMBDA",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"LAMBDA"},
			expectedErrorContains: "",
		},
		{
			category:              "Single word",
			query:                 "AccessDenied",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"AccessDenied"},
			expectedErrorContains: "",
		},
		{
			category:              "Single word",
			query:                 "42069",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"42069"},
			expectedErrorContains: "",
		},
		{
			category:              "Single word",
			query:                 "pulljob",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"pulljob"},
			expectedErrorContains: "",
		},
		{
			category:              "Single word",
			query:                 "<script>alert('xss')</script>",
			shouldPass:            false,
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got '<'",
		},

		// Single word searches with spaces
		{
			category:              "Single word with spaces",
			query:                 `" 504 "`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{" 504 "},
			expectedErrorContains: "",
		},
		{
			category:              "Single word with spaces",
			query:                 `"Importing "`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"Importing "},
			expectedErrorContains: "",
		},
		{
			category:              "Single word with spaces",
			query:                 `"Job ID"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"Job ID"},
			expectedErrorContains: "",
		},

		{
			category:              "Key with curly brace",
			query:                 `{UserId} = "U101"`,
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['{UserId}'] = ? AND mapContains(attributes, '{UserId}') = ?)",
			expectedArgs:          []any{"U101", true},
			expectedErrorContains: "",
		},
		{
			category:              "Key with @symbol",
			query:                 `user@email = "u@example.com"`,
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['user@email'] = ? AND mapContains(attributes, 'user@email') = ?)",
			expectedArgs:          []any{"u@example.com", true},
			expectedErrorContains: "",
		},
		{
			category:              "Key with @symbol",
			query:                 `#user_name = "anon42069"`,
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['#user_name'] = ? AND mapContains(attributes, '#user_name') = ?)",
			expectedArgs:          []any{"anon42069", true},
			expectedErrorContains: "",
		},
		{
			category:              "Key with @symbol",
			query:                 `gen_ai.completion.0.content = "जब तक इस देश में सिनेमा है"`,
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['gen_ai.completion.0.content'] = ? AND mapContains(attributes, 'gen_ai.completion.0.content') = ?)",
			expectedArgs:          []any{"जब तक इस देश में सिनेमा है", true},
			expectedErrorContains: "",
		},
		// Searches with special characters
		{
			category:              "Special characters",
			query:                 "[tracing]",
			shouldPass:            false,
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got '['",
		},
		{
			category:              "Special characters",
			query:                 "srikanth@o11y.io",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"srikanth@o11y.io"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "cancel_membership",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"cancel_membership"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 `"ERROR: cannot execute update() in a read-only context"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"ERROR: cannot execute update() in a read-only context"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "ERROR: cannot execute update() in a read-only context",
			shouldPass:            false,
			expectedErrorContains: "expecting one of {(, AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got ')'",
		},
		{
			category:              "Special characters",
			query:                 "https://example.com/user/default/0196877a-f01f-785e-a937-5da0a3efbb75",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"https://example.com/user/default/0196877a-f01f-785e-a937-5da0a3efbb75"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "\"STEPS_PER_DAY\"",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"STEPS_PER_DAY"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "#bvn",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"#bvn"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "question?mark",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"question?mark"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "backslash\\\\escape",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"backslash\\\\escape"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "underscore_separator",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"underscore_separator"},
			expectedErrorContains: "",
		},
		{
			category:              "Special characters",
			query:                 "\"Text with [brackets]\"",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"Text with [brackets]"},
			expectedErrorContains: "",
		},

		// Multi word searches
		{
			category:              "Multi word",
			query:                 "Fail to parse",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"Fail", "to", "parse"},
			expectedErrorContains: "",
		},
		{
			category:              "Multi word",
			query:                 "Importing file",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"Importing", "file"},
			expectedErrorContains: "",
		},
		{
			category:              "Multi word",
			query:                 "sync account status",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"sync", "account", "status"},
			expectedErrorContains: "",
		},
		{
			category:              "Multi word",
			query:                 "Download CSV Reports",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"Download", "CSV", "Reports"},
			expectedErrorContains: "",
		},
		{
			category:              "Multi word",
			query:                 "Emitted event to the Kafka topic",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"Emitted", "event", "to", "the", "Kafka", "topic"},
			expectedErrorContains: "",
		},
		{
			category:              "Multi word",
			query:                 "\"user authentication\" failed",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"user authentication", "failed"},
			expectedErrorContains: "",
		},

		// Search for IDs
		{
			category:              "ID search",
			query:                 "250430165501118HIgesxlEb9",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"250430165501118HIgesxlEb9"},
			expectedErrorContains: "",
		},
		{
			category:              "ID search",
			query:                 "d7b9d77aefa95aef19719775c10fda60c28342f23657d1e27304d6c59a3c3004",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"d7b9d77aefa95aef19719775c10fda60c28342f23657d1e27304d6c59a3c3004"},
			expectedErrorContains: "",
		},
		{
			category:              "ID search",
			query:                 "51183870",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"51183870"},
			expectedErrorContains: "",
		},
		{
			category:              "ID search",
			query:                 "79f82635-d014-4f99-adf5-41d31d291ae3",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"79f82635-d014-4f99-adf5-41d31d291ae3"},
			expectedErrorContains: "",
		},

		// Unicode characters in full text
		{
			category:              "Unicode",
			query:                 "café",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"café"},
			expectedErrorContains: "",
		},
		{
			category:              "Unicode",
			query:                 "résumé",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"résumé"},
			expectedErrorContains: "",
		},
		{
			category:              "Unicode",
			query:                 "Россия",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"Россия"},
			expectedErrorContains: "",
		},
		{
			category:              "Unicode",
			query:                 "\"I do not like emojis ❤️\"",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"I do not like emojis ❤️"},
			expectedErrorContains: "",
		},

		// Various number formats
		{
			category:              "Number format",
			query:                 "123",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"123"},
			expectedErrorContains: "",
		},
		{
			category:              "Number format",
			query:                 "3.14159",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"3.14159"},
			expectedErrorContains: "",
		},
		{
			category:              "Number format",
			query:                 "-42",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"-42"},
			expectedErrorContains: "",
		},
		{
			category:              "Number format",
			query:                 "1e6",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"1e6"},
			expectedErrorContains: "",
		},
		{
			category:              "Number format",
			query:                 "+100",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"\\+100"},
			expectedErrorContains: "",
		},
		{
			category:              "Number format",
			query:                 "0xFF",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"0xFF"},
			expectedErrorContains: "",
		},

		// Typical queries that combine FREETEXT with other constructs
		{
			category:              "FREETEXT with conditions",
			query:                 "critical NOT resolved status=open",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND NOT (match(LOWER(body), LOWER(?))) AND (toString(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?))",
			expectedArgs:          []any{"critical", "resolved", "open", true},
			expectedErrorContains: "",
		},
		{
			// this will result in failure from the DB side.
			// user will have to use attribute.status:string > open
			category:              "FREETEXT with conditions",
			query:                 "critical NOT resolved status > open",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND NOT (match(LOWER(body), LOWER(?))) AND (toString(toFloat64OrNull(attributes['status'])) > ? AND mapContains(attributes, 'status') = ?))",
			expectedArgs:          []any{"critical", "resolved", "open", true},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with conditions",
			query:                 "database error type=mysql",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)) AND (attributes['type'] = ? AND mapContains(attributes, 'type') = ?))",
			expectedArgs:          []any{"database", "error", "mysql", true},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with conditions",
			query:                 "\"connection timeout\" duration>30",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND (toFloat64(toFloat64OrNull(attributes['duration'])) > ? AND mapContains(attributes, 'duration') = ?))",
			expectedArgs:          []any{"connection timeout", float64(30), true},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with conditions",
			query:                 "warning level=critical",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND (attributes['level'] = ? AND mapContains(attributes, 'level') = ?))",
			expectedArgs:          []any{"warning", "critical", true},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with conditions",
			query:                 "error service.name=authentication",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND service = ?)",
			expectedArgs:          []any{"error", "authentication"},
			expectedErrorContains: "",
		},

		//fulltext with parenthesized expression
		{
			category:              "FREETEXT with parentheses",
			query:                 "error (status.code=500 OR status.code=503)",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND (((toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?) OR (toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?))))",
			expectedArgs:          []any{"error", float64(500), true, float64(503), true},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with parentheses",
			query:                 "(status.code=500 OR status.code=503) error",
			shouldPass:            true,
			expectedQuery:         "WHERE ((((toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?) OR (toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?))) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{float64(500), true, float64(503), true, "error"},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with parentheses",
			query:                 "error AND (status.code=500 OR status.code=503)",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND (((toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?) OR (toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?))))",
			expectedArgs:          []any{"error", float64(500), true, float64(503), true},
			expectedErrorContains: "",
		},
		{
			category:              "FREETEXT with parentheses",
			query:                 "(status.code=500 OR status.code=503) AND error",
			shouldPass:            true,
			expectedQuery:         "WHERE ((((toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?) OR (toFloat64(toFloat64OrNull(attributes['status.code'])) = ? AND mapContains(attributes, 'status.code') = ?))) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{float64(500), true, float64(503), true, "error"},
			expectedErrorContains: "",
		},

		// Whitespace with Fulltext
		{
			category:              "Whitespace with FREETEXT",
			query:                 "term1    term2",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"term1", "term2"},
			expectedErrorContains: "",
		},

		// Conflicts with the key token, are valid and without additional tokens, they are searched as FREETEXT
		{
			category:              "Key token conflict",
			query:                 "status.code",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"status.code"},
			expectedErrorContains: "",
		},
		{
			category:              "Key token conflict",
			query:                 "array_field",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"array_field"},
			expectedErrorContains: "",
		},
		{
			category:              "Key token conflict",
			query:                 "user_id.value",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"user_id.value"},
			expectedErrorContains: "",
		}, // Could be a key with dot notation or FREETEXT

		// Random set of cases
		{
			category:              "Random cases",
			query:                 "true",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"true"},
			expectedErrorContains: "",
		}, // Could be interpreted as boolean or FREETEXT
		{
			category:              "Random cases",
			query:                 "false",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"false"},
			expectedErrorContains: "",
		}, // Could be interpreted as boolean or FREETEXT
		{
			category:              "Random cases",
			query:                 "null",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"null"},
			expectedErrorContains: "",
		}, // Special value or FREETEXT
		{
			category:              "Random cases",
			query:                 "123abc",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"123abc"},
			expectedErrorContains: "",
		}, // Starts with number but contains letters
		{
			category:              "Random cases",
			query:                 "0x123F",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"0x123F"},
			expectedErrorContains: "",
		}, // Hex number format
		{
			category:              "Random cases",
			query:                 "1.2.3",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"1.2.3"},
			expectedErrorContains: "",
		}, // Version number format
		{
			category:              "Random cases",
			query:                 "a+b-c*d/e",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"a+b-c*d/e"},
			expectedErrorContains: "",
		}, // Mathematical expression as FREETEXT
		{
			category:              "Random cases",
			query:                 "http://example.com/path",
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"http://example.com/path"},
			expectedErrorContains: "",
		}, // URL as FREETEXT

		// Keyword conflicting
		{
			category:              "Keyword conflict",
			query:                 "and",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'and'",
		},
		{
			category:              "Keyword conflict",
			query:                 "or",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'or'",
		},
		{
			category:              "Keyword conflict",
			query:                 "not",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), FREETEXT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got EOF",
		},
		{
			category:              "Keyword conflict",
			query:                 "like",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'like'",
		},
		{
			category:              "Keyword conflict",
			query:                 "between",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'between'",
		},
		{
			category:              "Keyword conflict",
			query:                 "in",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'in'",
		},
		{
			category:              "Keyword conflict",
			query:                 "exists",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'exists'",
		},
		{
			category:              "Keyword conflict",
			query:                 "regexp",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'regexp'",
		},
		{
			category:              "Keyword conflict",
			query:                 "contains",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'contains'",
		},
		{
			category:              "Keyword conflict",
			query:                 "has",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, )} but got EOF",
		},
		{
			category:              "Keyword conflict",
			query:                 "hasany",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, )} but got EOF",
		},
		{
			category:              "Keyword conflict",
			query:                 "hasall",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "expecting one of {(, )} but got EOF",
		},

		// Boundary of key-operator-value AND full text
		{
			category:              "Key-operator-value boundary",
			query:                 `"not!equal"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"not!equal"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "greater>than",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "key `greater` not found",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 `"greater>than"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"greater>than"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "less<than",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "key `less` not found",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 `"less<than"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"less<than"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "single'quote'",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"single", "quote"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "quoted\"text\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND match(LOWER(body), LOWER(?)))",
			expectedArgs:          []any{"quoted", "text"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "array[index]",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "line 1:5",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "function(param)",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(LOWER(body), LOWER(?)) AND (match(LOWER(body), LOWER(?))))",
			expectedArgs:          []any{"function", "param"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 `"function(param)"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"function(param)"},
			expectedErrorContains: "",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 "user=admin",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          []any{},
			expectedErrorContains: "key `user` not found",
		},
		{
			category:              "Key-operator-value boundary",
			query:                 `"user=admin"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"user=admin"},
			expectedErrorContains: "",
		},

		// Basic equality
		{
			category:              "Basic equality",
			query:                 "status=200",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)",
			expectedArgs:          []any{float64(200), true},
			expectedErrorContains: "",
		},
		{
			category:              "Basic equality",
			query:                 "code=400",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['code'])) = ? AND mapContains(attributes, 'code') = ?)",
			expectedArgs:          []any{float64(400), true},
			expectedErrorContains: "",
		},
		{
			category:              "Basic equality",
			query:                 "service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE service = ?",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "Basic equality",
			query:                 "user.email=\"user@example.com\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['user.email'] = ? AND mapContains(attributes, 'user.email') = ?)",
			expectedArgs:          []any{"user@example.com", true},
			expectedErrorContains: "",
		},
		{
			category:              "Basic equality",
			query:                 "isEnabled=true",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['isEnabled'] = 'true' = ? AND mapContains(attributes, 'isEnabled') = ?)",
			expectedArgs:          []any{true, true},
			expectedErrorContains: "",
		},
		{
			category:              "Basic equality",
			query:                 "count=0",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) = ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(0), true},
			expectedErrorContains: "",
		},
		{
			category:              "Basic equality",
			query:                 "is_valid=false",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['is_valid'] = 'true' = ? AND mapContains(attributes, 'is_valid') = ?)",
			expectedArgs:          []any{false, true},
			expectedErrorContains: "",
		},

		// Not equals
		{
			category:              "Not equals",
			query:                 "status!=200",
			shouldPass:            true,
			expectedQuery:         "WHERE toFloat64(toFloat64OrNull(attributes['status'])) <> ?",
			expectedArgs:          []any{float64(200)},
			expectedErrorContains: "",
		},
		{
			category:              "Not equals",
			query:                 "status<>200",
			shouldPass:            true,
			expectedQuery:         "WHERE toFloat64(toFloat64OrNull(attributes['status'])) <> ?",
			expectedArgs:          []any{float64(200)},
			expectedErrorContains: "",
		},
		{
			category:              "Not equals",
			query:                 "code!=400",
			shouldPass:            true,
			expectedQuery:         "WHERE toFloat64(toFloat64OrNull(attributes['code'])) <> ?",
			expectedArgs:          []any{float64(400)},
			expectedErrorContains: "",
		},
		{
			category:              "Not equals",
			query:                 "service.name!=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE service <> ?",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "Not equals",
			query:                 "user.email!=\"user@example.com\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['user.email'] <> ?",
			expectedArgs:          []any{"user@example.com"},
			expectedErrorContains: "",
		},

		// Less than
		{
			category:              "Less than",
			query:                 "count<10",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) < ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(10), true},
			expectedErrorContains: "",
		},
		{
			category:              "Less than",
			query:                 "duration<1000",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?)",
			expectedArgs:          []any{float64(1000), true},
			expectedErrorContains: "",
		},

		// Less than or equal
		{
			category:              "Less than or equal",
			query:                 "count<=10",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) <= ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(10), true},
			expectedErrorContains: "",
		},
		{
			category:              "Less than or equal",
			query:                 "duration<=1000",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['duration'])) <= ? AND mapContains(attributes, 'duration') = ?)",
			expectedArgs:          []any{float64(1000), true},
			expectedErrorContains: "",
		},

		// Greater than
		{
			category:              "Greater than",
			query:                 "count>10",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(10), true},
			expectedErrorContains: "",
		},
		{
			category:              "Greater than",
			query:                 "duration>1000",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['duration'])) > ? AND mapContains(attributes, 'duration') = ?)",
			expectedArgs:          []any{float64(1000), true},
			expectedErrorContains: "",
		},

		// Greater than or equal
		{
			category:              "Greater than or equal",
			query:                 "count>=10",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) >= ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(10), true},
			expectedErrorContains: "",
		},
		{
			category:              "Greater than or equal",
			query:                 "duration>=1000",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['duration'])) >= ? AND mapContains(attributes, 'duration') = ?)",
			expectedArgs:          []any{float64(1000), true},
			expectedErrorContains: "",
		},

		// Basic LIKE
		{
			category:              "LIKE operator",
			query:                 "message LIKE \"%error%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['message'] LIKE ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:          []any{"%error%", true},
			expectedErrorContains: "",
		},
		{
			category:              "LIKE operator",
			query:                 "path LIKE \"/api/%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['path'] LIKE ? AND mapContains(attributes, 'path') = ?)",
			expectedArgs:          []any{"/api/%", true},
			expectedErrorContains: "",
		},
		{
			category:              "LIKE operator",
			query:                 "email LIKE \"%@example.com\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['email'] LIKE ? AND mapContains(attributes, 'email') = ?)",
			expectedArgs:          []any{"%@example.com", true},
			expectedErrorContains: "",
		},
		{
			category:              "LIKE operator",
			query:                 "filename LIKE \"%.pdf\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['filename'] LIKE ? AND mapContains(attributes, 'filename') = ?)",
			expectedArgs:          []any{"%.pdf", true},
			expectedErrorContains: "",
		},

		// Case-insensitive LIKE (ILIKE)
		{
			category:              "ILIKE operator",
			query:                 "message ILIKE \"%error%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:          []any{"%error%", true},
			expectedErrorContains: "",
		},
		{
			category:              "ILIKE operator",
			query:                 "path ILIKE \"/api/%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['path'] ILIKE ? AND mapContains(attributes, 'path') = ?)",
			expectedArgs:          []any{"/api/%", true},
			expectedErrorContains: "",
		},
		{
			category:              "ILIKE operator",
			query:                 "email ILIKE \"%@EXAMPLE.com\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['email'] ILIKE ? AND mapContains(attributes, 'email') = ?)",
			expectedArgs:          []any{"%@EXAMPLE.com", true},
			expectedErrorContains: "",
		},
		{
			category:              "ILIKE operator",
			query:                 "filename ILIKE \"%.PDF\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['filename'] ILIKE ? AND mapContains(attributes, 'filename') = ?)",
			expectedArgs:          []any{"%.PDF", true},
			expectedErrorContains: "",
		},

		// NOT LIKE
		{
			category:              "NOT LIKE operator",
			query:                 "message NOT LIKE \"%error%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['message'] NOT LIKE ?",
			expectedArgs:          []any{"%error%"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT LIKE operator",
			query:                 "path NOT LIKE \"/api/%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['path'] NOT LIKE ?",
			expectedArgs:          []any{"/api/%"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT LIKE operator",
			query:                 "email NOT LIKE \"%@example.com\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['email'] NOT LIKE ?",
			expectedArgs:          []any{"%@example.com"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT LIKE operator",
			query:                 "filename NOT LIKE \"%.pdf\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['filename'] NOT LIKE ?",
			expectedArgs:          []any{"%.pdf"},
			expectedErrorContains: "",
		},

		// NOT ILIKE
		{
			category:              "NOT ILIKE operator",
			query:                 "message NOT ILIKE \"%error%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['message'] NOT ILIKE ?",
			expectedArgs:          []any{"%error%"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT ILIKE operator",
			query:                 "path NOT ILIKE \"/api/%\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['path'] NOT ILIKE ?",
			expectedArgs:          []any{"/api/%"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT ILIKE operator",
			query:                 "email NOT ILIKE \"%@EXAMPLE.com\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['email'] NOT ILIKE ?",
			expectedArgs:          []any{"%@EXAMPLE.com"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT ILIKE operator",
			query:                 "filename NOT ILIKE \"%.PDF\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['filename'] NOT ILIKE ?",
			expectedArgs:          []any{"%.PDF"},
			expectedErrorContains: "",
		},

		// Basic BETWEEN
		{
			category:              "BETWEEN operator",
			query:                 "count BETWEEN 1 AND 10",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) BETWEEN ? AND ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(1), float64(10), true},
			expectedErrorContains: "",
		},
		{
			category:              "BETWEEN operator",
			query:                 "duration BETWEEN 100 AND 1000",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['duration'])) BETWEEN ? AND ? AND mapContains(attributes, 'duration') = ?)",
			expectedArgs:          []any{float64(100), float64(1000), true},
			expectedErrorContains: "",
		},
		{
			category:              "BETWEEN operator",
			query:                 "amount BETWEEN 0.1 AND 9.9",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['amount'])) BETWEEN ? AND ? AND mapContains(attributes, 'amount') = ?)",
			expectedArgs:          []any{0.1, 9.9, true},
			expectedErrorContains: "",
		},

		// NOT BETWEEN
		{
			category:              "NOT BETWEEN operator",
			query:                 "count NOT BETWEEN 1 AND 10",
			shouldPass:            true,
			expectedQuery:         "WHERE toFloat64(toFloat64OrNull(attributes['count'])) NOT BETWEEN ? AND ?",
			expectedArgs:          []any{float64(1), float64(10)},
			expectedErrorContains: "",
		},
		{
			category:              "NOT BETWEEN operator",
			query:                 "duration NOT BETWEEN 100 AND 1000",
			shouldPass:            true,
			expectedQuery:         "WHERE toFloat64(toFloat64OrNull(attributes['duration'])) NOT BETWEEN ? AND ?",
			expectedArgs:          []any{float64(100), float64(1000)},
			expectedErrorContains: "",
		},
		{
			category:              "NOT BETWEEN operator",
			query:                 "amount NOT BETWEEN 0.1 AND 9.9",
			shouldPass:            true,
			expectedQuery:         "WHERE toFloat64(toFloat64OrNull(attributes['amount'])) NOT BETWEEN ? AND ?",
			expectedArgs:          []any{0.1, 9.9},
			expectedErrorContains: "",
		},

		// IN with parentheses
		{
			category:              "IN operator (parentheses)",
			query:                 "status IN (200, 201, 202)",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? OR toFloat64(toFloat64OrNull(attributes['status'])) = ? OR toFloat64(toFloat64OrNull(attributes['status'])) = ?) AND mapContains(attributes, 'status') = ?)",
			expectedArgs:          []any{float64(200), float64(201), float64(202), true},
			expectedErrorContains: "",
		},
		{
			category:              "IN operator (parentheses)",
			query:                 "error.code IN (404, 500, 503)",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['error.code'])) = ? OR toFloat64(toFloat64OrNull(attributes['error.code'])) = ? OR toFloat64(toFloat64OrNull(attributes['error.code'])) = ?) AND mapContains(attributes, 'error.code') = ?)",
			expectedArgs:          []any{float64(404), float64(500), float64(503), true},
			expectedErrorContains: "",
		},
		{
			category:              "IN operator (parentheses)",
			query:                 "service.name IN (\"api\", \"web\", \"auth\")",
			shouldPass:            true,
			expectedQuery:         "WHERE (service = ? OR service = ? OR service = ?)",
			expectedArgs:          []any{"api", "web", "auth"},
			expectedErrorContains: "",
		},
		{
			category:              "IN operator (parentheses)",
			query:                 "environment IN (\"dev\", \"test\", \"staging\", \"prod\")",
			shouldPass:            true,
			expectedQuery:         "WHERE ((attributes['environment'] = ? OR attributes['environment'] = ? OR attributes['environment'] = ? OR attributes['environment'] = ?) AND mapContains(attributes, 'environment') = ?)",
			expectedArgs:          []any{"dev", "test", "staging", "prod", true},
			expectedErrorContains: "",
		},

		// IN with brackets
		{
			category:              "IN operator (brackets)",
			query:                 "status IN [200, 201, 202]",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? OR toFloat64(toFloat64OrNull(attributes['status'])) = ? OR toFloat64(toFloat64OrNull(attributes['status'])) = ?) AND mapContains(attributes, 'status') = ?)",
			expectedArgs:          []any{float64(200), float64(201), float64(202), true},
			expectedErrorContains: "",
		},
		{
			category:              "IN operator (brackets)",
			query:                 "error.code IN [404, 500, 503]",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['error.code'])) = ? OR toFloat64(toFloat64OrNull(attributes['error.code'])) = ? OR toFloat64(toFloat64OrNull(attributes['error.code'])) = ?) AND mapContains(attributes, 'error.code') = ?)",
			expectedArgs:          []any{float64(404), float64(500), float64(503), true},
			expectedErrorContains: "",
		},
		{
			category:              "IN operator (brackets)",
			query:                 "service.name IN [\"api\", \"web\", \"auth\"]",
			shouldPass:            true,
			expectedQuery:         "WHERE (service = ? OR service = ? OR service = ?)",
			expectedArgs:          []any{"api", "web", "auth"},
			expectedErrorContains: "",
		},
		{
			category:              "IN operator (brackets)",
			query:                 "environment IN [\"dev\", \"test\", \"staging\", \"prod\"]",
			shouldPass:            true,
			expectedQuery:         "WHERE ((attributes['environment'] = ? OR attributes['environment'] = ? OR attributes['environment'] = ? OR attributes['environment'] = ?) AND mapContains(attributes, 'environment') = ?)",
			expectedArgs:          []any{"dev", "test", "staging", "prod", true},
			expectedErrorContains: "",
		},

		// NOT IN with parentheses
		{
			category:              "NOT IN operator (parentheses)",
			query:                 "status NOT IN (400, 500)",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['status'])) <> ? AND toFloat64(toFloat64OrNull(attributes['status'])) <> ?)",
			expectedArgs:          []any{float64(400), float64(500)},
			expectedErrorContains: "",
		},
		{
			category:              "NOT IN operator (parentheses)",
			query:                 "error.code NOT IN (401, 403)",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['error.code'])) <> ? AND toFloat64(toFloat64OrNull(attributes['error.code'])) <> ?)",
			expectedArgs:          []any{float64(401), float64(403)},
			expectedErrorContains: "",
		},
		{
			category:              "NOT IN operator (parentheses)",
			query:                 "service.name NOT IN (\"database\", \"cache\")",
			shouldPass:            true,
			expectedQuery:         "WHERE (service <> ? AND service <> ?)",
			expectedArgs:          []any{"database", "cache"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT IN operator (parentheses)",
			query:                 "environment NOT IN (\"prod\")",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['environment'] <> ?)",
			expectedArgs:          []any{"prod"},
			expectedErrorContains: "",
		},

		// NOT IN with brackets
		{
			category:              "NOT IN operator (brackets)",
			query:                 "status NOT IN [400, 500]",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['status'])) <> ? AND toFloat64(toFloat64OrNull(attributes['status'])) <> ?)",
			expectedArgs:          []any{float64(400), float64(500)},
			expectedErrorContains: "",
		},
		{
			category:              "NOT IN operator (brackets)",
			query:                 "error.code NOT IN [401, 403]",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['error.code'])) <> ? AND toFloat64(toFloat64OrNull(attributes['error.code'])) <> ?)",
			expectedArgs:          []any{float64(401), float64(403)},
			expectedErrorContains: "",
		},
		{
			category:              "NOT IN operator (brackets)",
			query:                 "service.name NOT IN [\"database\", \"cache\"]",
			shouldPass:            true,
			expectedQuery:         "WHERE (service <> ? AND service <> ?)",
			expectedArgs:          []any{"database", "cache"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT IN operator (brackets)",
			query:                 "environment NOT IN [\"prod\"]",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['environment'] <> ?)",
			expectedArgs:          []any{"prod"},
			expectedErrorContains: "",
		},

		// Basic EXISTS
		{
			category:              "EXISTS operator",
			query:                 "user.id EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'user.id') = ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "EXISTS operator",
			query:                 "metadata.version EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'metadata.version') = ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "EXISTS operator",
			query:                 "request.headers.authorization EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'request.headers.authorization') = ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "EXISTS operator",
			query:                 "response.body.data EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'response.body.data') = ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "EXISTS operator on resource",
			query:                 "service.name EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE notEmpty(service)",
			expectedErrorContains: "",
		},

		// NOT EXISTS
		{
			category:              "NOT EXISTS operator",
			query:                 "user.id NOT EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'user.id') <> ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "NOT EXISTS operator",
			query:                 "user.id not exists",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'user.id') <> ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "NOT EXISTS operator",
			query:                 "metadata.version NOT EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'metadata.version') <> ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "NOT EXISTS operator",
			query:                 "request.headers.authorization NOT EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'request.headers.authorization') <> ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "NOT EXISTS operator",
			query:                 "response.body.data NOT EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'response.body.data') <> ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "EXISTS operator on resource",
			query:                 "service.name NOT EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE empty(service)",
			expectedErrorContains: "",
		},

		// Basic REGEXP
		{
			category:              "REGEXP operator",
			query:                 "message REGEXP \"^ERROR:\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(attributes['message'], ?) AND mapContains(attributes, 'message') = ?)",
			expectedArgs:          []any{"^ERROR:", true},
			expectedErrorContains: "",
		},
		{
			category:              "REGEXP operator",
			query:                 "email REGEXP \"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}$\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(attributes['email'], ?) AND mapContains(attributes, 'email') = ?)",
			expectedArgs:          []any{"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", true},
			expectedErrorContains: "",
		},
		{
			category:              "REGEXP operator",
			query:                 "version REGEXP \"^v\\\\d+\\\\.\\\\d+\\\\.\\\\d+$\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(attributes['version'], ?) AND mapContains(attributes, 'version') = ?)",
			expectedArgs:          []any{"^v\\d+\\.\\d+\\.\\d+$", true},
			expectedErrorContains: "",
		},
		{
			category:              "REGEXP operator",
			query:                 "path REGEXP \"^/api/v\\\\d+/users/\\\\d+$\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (match(attributes['path'], ?) AND mapContains(attributes, 'path') = ?)",
			expectedArgs:          []any{"^/api/v\\d+/users/\\d+$", true},
			expectedErrorContains: "",
		},
		{
			category:              "REGEXP operator",
			query:                 `"^\[(INFO|WARN|ERROR|DEBUG)\] .+$"`,
			shouldPass:            true,
			expectedQuery:         "WHERE match(LOWER(body), LOWER(?))",
			expectedArgs:          []any{`^\[(INFO|WARN|ERROR|DEBUG)\] .+$`},
			expectedErrorContains: "",
		},

		// NOT REGEXP
		{
			category:              "NOT REGEXP operator",
			query:                 "message NOT REGEXP \"^ERROR:\"",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT match(attributes['message'], ?)",
			expectedArgs:          []any{"^ERROR:"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT REGEXP operator",
			query:                 "email NOT REGEXP \"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}$\"",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT match(attributes['email'], ?)",
			expectedArgs:          []any{"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT REGEXP operator",
			query:                 "version NOT REGEXP \"^v\\\\d+\\\\.\\\\d+\\\\.\\\\d+$\"",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT match(attributes['version'], ?)",
			expectedArgs:          []any{"^v\\d+\\.\\d+\\.\\d+$"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT REGEXP operator",
			query:                 "path NOT REGEXP \"^/api/v\\d+/users/\\d+$\"",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT match(attributes['path'], ?)",
			expectedArgs:          []any{"^/api/v\\d+/users/\\d+$"},
			expectedErrorContains: "",
		},

		// Basic CONTAINS
		{
			category:              "CONTAINS operator",
			query:                 "message CONTAINS \"error\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:          []any{"%error%", true},
			expectedErrorContains: "",
		},
		{
			category:              "number contains body",
			query:                 "body CONTAINS 521509198310",
			shouldPass:            true,
			expectedQuery:         "WHERE body ILIKE ?",
			expectedArgs:          []any{"%521509198310%"},
			expectedErrorContains: "",
		},
		{
			category:              "CONTAINS operator",
			query:                 "level CONTAINS \"critical\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['level'] ILIKE ? AND mapContains(attributes, 'level') = ?)",
			expectedArgs:          []any{"%critical%", true},
			expectedErrorContains: "",
		},
		{
			category:              "CONTAINS operator",
			query:                 "path CONTAINS \"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['path'] ILIKE ? AND mapContains(attributes, 'path') = ?)",
			expectedArgs:          []any{"%api%", true},
			expectedErrorContains: "",
		},

		// NOT CONTAINS
		{
			category:              "NOT CONTAINS operator",
			query:                 "message NOT CONTAINS \"error\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['message'] NOT ILIKE ?",
			expectedArgs:          []any{"%error%"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT CONTAINS operator",
			query:                 "level NOT CONTAINS \"critical\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['level'] NOT ILIKE ?",
			expectedArgs:          []any{"%critical%"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT CONTAINS operator",
			query:                 "path NOT CONTAINS \"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE attributes['path'] NOT ILIKE ?",
			expectedArgs:          []any{"%api%"},
			expectedErrorContains: "",
		},

		// HASTOKEN
		{
			category:              "hasToken",
			query:                 "hasToken(body, \"download\")",
			shouldPass:            true,
			expectedQuery:         "WHERE hasToken(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"download"},
			expectedErrorContains: "",
		},
		{
			category:              "hasTokenNumber",
			query:                 "hasToken(body, 1)",
			shouldPass:            false,
			expectedQuery:         "WHERE hasToken(LOWER(body), LOWER(?))",
			expectedArgs:          []any{"download"},
			expectedErrorContains: "function `hasToken` expects value parameter to be a string",
		},

		// Basic materialized key
		{
			category:              "Materialized key",
			query:                 "materialized.key.name=\"test\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['materialized.key.name'] = ? AND mapContains(attributes, 'materialized.key.name') = ?)",
			expectedArgs:          []any{"test", true},
			expectedErrorContains: "",
		},

		// Explicit AND
		{
			category:              "Explicit AND",
			query:                 "status=200 AND service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ?)",
			expectedArgs:          []any{float64(200), true, "api"},
			expectedErrorContains: "",
		},
		{
			category:              "Explicit AND",
			query:                 "count>0 AND duration<1000",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND (toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?))",
			expectedArgs:          []any{float64(0), true, float64(1000), true},
			expectedErrorContains: "",
		},
		{
			category:              "Explicit AND",
			query:                 "message CONTAINS \"error\" AND level=\"ERROR\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?) AND (attributes['level'] = ? AND mapContains(attributes, 'level') = ?))",
			expectedArgs:          []any{"%error%", true, "ERROR", true},
			expectedErrorContains: "",
		},

		// Explicit OR
		{
			category:              "Explicit OR",
			query:                 "status=200 OR status=201",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) OR (toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?))",
			expectedArgs:          []any{float64(200), true, float64(201), true},
			expectedErrorContains: "",
		},
		{
			category:              "Explicit OR",
			query:                 "service.name=\"api\" OR service.name=\"web\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (service = ? OR service = ?)",
			expectedArgs:          []any{"api", "web"},
			expectedErrorContains: "",
		},
		{
			category:              "Explicit OR",
			query:                 "count<10 OR count>100",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['count'])) < ? AND mapContains(attributes, 'count') = ?) OR (toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?))",
			expectedArgs:          []any{float64(10), true, float64(100), true},
			expectedErrorContains: "",
		},

		// NOT with various expressions
		{
			category:              "NOT with expressions",
			query:                 "NOT status=200",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?))",
			expectedArgs:          []any{float64(200), true},
			expectedErrorContains: "",
		},
		{
			category:              "NOT with expressions",
			query:                 "NOT service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT (service = ?)",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "NOT with expressions",
			query:                 "NOT count>10",
			shouldPass:            true,
			expectedQuery:         "WHERE NOT ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?))",
			expectedArgs:          []any{float64(10), true},
			expectedErrorContains: "",
		},

		// AND + OR combinations
		{
			category:              "AND + OR combinations",
			query:                 "status=200 AND (service.name=\"api\" OR service.name=\"web\")",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND ((service = ? OR service = ?)))",
			expectedArgs:          []any{float64(200), true, "api", "web"},
			expectedErrorContains: "",
		},
		{
			category:              "AND + OR combinations",
			query:                 "(count>10 AND count<100) OR (duration>1000 AND duration<5000)",
			shouldPass:            true,
			expectedQuery:         "WHERE ((((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND (toFloat64(toFloat64OrNull(attributes['count'])) < ? AND mapContains(attributes, 'count') = ?))) OR (((toFloat64(toFloat64OrNull(attributes['duration'])) > ? AND mapContains(attributes, 'duration') = ?) AND (toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?))))",
			expectedArgs:          []any{float64(10), true, float64(100), true, float64(1000), true, float64(5000), true},
			expectedErrorContains: "",
		},
		{
			category:              "AND + OR combinations",
			query:                 "level=\"ERROR\" OR (level=\"WARN\" AND message CONTAINS \"timeout\")",
			shouldPass:            true,
			expectedQuery:         "WHERE ((attributes['level'] = ? AND mapContains(attributes, 'level') = ?) OR (((attributes['level'] = ? AND mapContains(attributes, 'level') = ?) AND (attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?))))",
			expectedArgs:          []any{"ERROR", true, "WARN", true, "%timeout%", true},
			expectedErrorContains: "",
		},

		// AND + NOT combinations
		{
			category:              "AND + NOT combinations",
			query:                 "status=200 AND NOT service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND NOT (service = ?))",
			expectedArgs:          []any{float64(200), true, "api"},
			expectedErrorContains: "",
		},
		{
			category:              "AND + NOT combinations",
			query:                 "count>0 AND NOT error.code EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND NOT (mapContains(attributes, 'error.code') = ?))",
			expectedArgs:          []any{float64(0), true, true},
			expectedErrorContains: "",
		},

		// OR + NOT combinations
		{
			category:              "OR + NOT combinations",
			query:                 "NOT status=200 OR NOT service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (NOT ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)) OR NOT (service = ?))",
			expectedArgs:          []any{float64(200), true, "api"},
			expectedErrorContains: "",
		},
		{
			category:              "OR + NOT combinations",
			query:                 "NOT count>0 OR NOT error.code EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE (NOT ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?)) OR NOT (mapContains(attributes, 'error.code') = ?))",
			expectedArgs:          []any{float64(0), true, true},
			expectedErrorContains: "",
		},

		// AND + OR + NOT combinations
		{
			category:              "AND + OR + NOT combinations",
			query:                 "status=200 AND (service.name=\"api\" OR NOT duration>1000)",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND ((service = ? OR NOT ((toFloat64(toFloat64OrNull(attributes['duration'])) > ? AND mapContains(attributes, 'duration') = ?)))))",
			expectedArgs:          []any{float64(200), true, "api", float64(1000), true},
			expectedErrorContains: "",
		},
		{
			category:              "AND + OR + NOT combinations",
			query:                 "(level=\"ERROR\" OR level=\"FATAL\") AND NOT message CONTAINS \"expected\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((((attributes['level'] = ? AND mapContains(attributes, 'level') = ?) OR (attributes['level'] = ? AND mapContains(attributes, 'level') = ?))) AND NOT ((attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?)))",
			expectedArgs:          []any{"ERROR", true, "FATAL", true, "%expected%", true},
			expectedErrorContains: "",
		},
		{
			category:              "AND + OR + NOT combinations",
			query:                 "NOT (status=200 AND service.name=\"api\") OR count>0",
			shouldPass:            true,
			expectedQuery:         "WHERE (NOT ((((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ?))) OR (toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?))",
			expectedArgs:          []any{float64(200), true, "api", float64(0), true},
			expectedErrorContains: "",
		},

		// Multiple expressions without explicit AND
		{
			category:              "Implicit AND",
			query:                 "status=200 service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ?)",
			expectedArgs:          []any{float64(200), true, "api"},
			expectedErrorContains: "",
		},
		{
			category:              "Implicit AND",
			query:                 "count>0 duration<1000",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND (toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?))",
			expectedArgs:          []any{float64(0), true, float64(1000), true},
			expectedErrorContains: "",
		},
		{
			category:              "Implicit AND",
			query:                 "message CONTAINS \"error\" level=\"ERROR\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?) AND (attributes['level'] = ? AND mapContains(attributes, 'level') = ?))",
			expectedArgs:          []any{"%error%", true, "ERROR", true},
			expectedErrorContains: "",
		},

		// Mixed implicit and explicit AND
		{
			category:              "Mixed implicit/explicit AND",
			query:                 "status=200 AND service.name=\"api\" duration<1000",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ? AND (toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?))",
			expectedArgs:          []any{float64(200), true, "api", float64(1000), true},
			expectedErrorContains: "",
		},
		{
			category:              "Mixed implicit/explicit AND",
			query:                 "count>0 level=\"ERROR\" AND message CONTAINS \"error\"",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND (attributes['level'] = ? AND mapContains(attributes, 'level') = ?) AND (attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?))",
			expectedArgs:          []any{float64(0), true, "ERROR", true, "%error%", true},
			expectedErrorContains: "",
		},

		// Simple grouping
		{
			category:              "Simple grouping",
			query:                 "(status=200)",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?))",
			expectedArgs:          []any{float64(200), true},
			expectedErrorContains: "",
		},
		{
			category:              "Simple grouping",
			query:                 "service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE service = ?",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "Simple grouping",
			query:                 "(count>0)",
			shouldPass:            true,
			expectedQuery:         "WHERE ((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?))",
			expectedArgs:          []any{float64(0), true},
			expectedErrorContains: "",
		},

		// Nested grouping
		{
			category:              "Nested grouping",
			query:                 "((status=200))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)))",
			expectedArgs:          []any{float64(200), true},
			expectedErrorContains: "",
		},
		{
			category:              "Nested grouping",
			query:                 "(((service.name=\"api\")))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((service = ?)))",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "Nested grouping",
			query:                 "((count>0) AND (duration<1000))",
			shouldPass:            true,
			expectedQuery:         "WHERE ((((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?)) AND ((toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?))))",
			expectedArgs:          []any{float64(0), true, float64(1000), true},
			expectedErrorContains: "",
		},

		// Complex nested grouping
		{
			category:              "Complex nested grouping",
			query:                 "(status=200 AND (service.name=\"api\" OR service.name=\"web\"))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND ((service = ? OR service = ?))))",
			expectedArgs:          []any{float64(200), true, "api", "web"},
			expectedErrorContains: "",
		},
		{
			category:              "Complex nested grouping",
			query:                 "((count>0 AND count<100) OR (duration>1000 AND duration<5000))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND (toFloat64(toFloat64OrNull(attributes['count'])) < ? AND mapContains(attributes, 'count') = ?))) OR (((toFloat64(toFloat64OrNull(attributes['duration'])) > ? AND mapContains(attributes, 'duration') = ?) AND (toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?)))))",
			expectedArgs:          []any{float64(0), true, float64(100), true, float64(1000), true, float64(5000), true},
			expectedErrorContains: "",
		},
		{
			category:              "Complex nested grouping",
			query:                 "(level=\"ERROR\" OR (level=\"WARN\" AND (message CONTAINS \"timeout\" OR message CONTAINS \"connection\")))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((attributes['level'] = ? AND mapContains(attributes, 'level') = ?) OR (((attributes['level'] = ? AND mapContains(attributes, 'level') = ?) AND (((attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?) OR (attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?)))))))",
			expectedArgs:          []any{"ERROR", true, "WARN", true, "%timeout%", true, "%connection%", true},
			expectedErrorContains: "",
		},

		// Deep nesting with mixed operators
		{
			category:              "Deep nesting",
			query:                 "(((status=200 OR status=201) AND service.name=\"api\") OR ((status=202 OR status=203) AND service.name=\"web\"))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((((((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) OR (toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?))) AND service = ?)) OR (((((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) OR (toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?))) AND service = ?))))",
			expectedArgs:          []any{float64(200), true, float64(201), true, "api", float64(202), true, float64(203), true, "web"},
			expectedErrorContains: "",
		},
		{
			category:              "Deep nesting",
			query:                 "(count>0 AND ((duration<1000 AND service.name=\"api\") OR (duration<500 AND service.name=\"web\")))",
			shouldPass:            true,
			expectedQuery:         "WHERE (((toFloat64(toFloat64OrNull(attributes['count'])) > ? AND mapContains(attributes, 'count') = ?) AND (((((toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?) AND service = ?)) OR (((toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?) AND service = ?))))))",
			expectedArgs:          []any{float64(0), true, float64(1000), true, "api", float64(500), true, "web"},
			expectedErrorContains: "",
		},

		// String values with different quote styles
		{
			category:              "String quote styles",
			query:                 "service.name=\"api\"",
			shouldPass:            true,
			expectedQuery:         "WHERE service = ?",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "String quote styles",
			query:                 "service.name='api'",
			shouldPass:            true,
			expectedQuery:         "WHERE service = ?",
			expectedArgs:          []any{"api"},
			expectedErrorContains: "",
		},
		{
			category:              "String quote styles",
			query:                 "message=\"This is a \\\"quoted\\\" message\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['message'] = ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:          []any{"This is a \\\"quoted\\\" message", true},
			expectedErrorContains: "",
		},
		{
			category:              "String quote styles",
			query:                 "message='This is a \\'quoted\\' message'",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['message'] = ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:          []any{"This is a 'quoted' message", true},
			expectedErrorContains: "",
		},

		// Numeric values
		{
			category:              "Numeric values",
			query:                 "status=200",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)",
			expectedArgs:          []any{float64(200), true},
			expectedErrorContains: "",
		},
		{
			category:              "Numeric values",
			query:                 "count=0",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['count'])) = ? AND mapContains(attributes, 'count') = ?)",
			expectedArgs:          []any{float64(0), true},
			expectedErrorContains: "",
		},
		{
			category:              "Numeric values",
			query:                 "duration=1000.5",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['duration'])) = ? AND mapContains(attributes, 'duration') = ?)",
			expectedArgs:          []any{float64(1000.5), true},
			expectedErrorContains: "",
		},
		{
			category:              "Numeric values",
			query:                 "amount=-10.25",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['amount'])) = ? AND mapContains(attributes, 'amount') = ?)",
			expectedArgs:          []any{float64(-10.25), true},
			expectedErrorContains: "",
		},

		// Boolean values
		{
			category:              "Boolean values",
			query:                 "isEnabled=true",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['isEnabled'] = 'true' = ? AND mapContains(attributes, 'isEnabled') = ?)",
			expectedArgs:          []any{true, true},
			expectedErrorContains: "",
		},
		{
			category:              "Boolean values",
			query:                 "isDisabled=false",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['isDisabled'] = 'true' = ? AND mapContains(attributes, 'isDisabled') = ?)",
			expectedArgs:          []any{false, true},
			expectedErrorContains: "",
		},
		{
			category:              "Boolean values",
			query:                 "is_valid=TRUE",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['is_valid'] = 'true' = ? AND mapContains(attributes, 'is_valid') = ?)",
			expectedArgs:          []any{true, true},
			expectedErrorContains: "",
		},
		{
			category:              "Boolean values",
			query:                 "is_invalid=FALSE",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['is_invalid'] = 'true' = ? AND mapContains(attributes, 'is_invalid') = ?)",
			expectedArgs:          []any{false, true},
			expectedErrorContains: "",
		},

		// Null and undefined values, there is no such type as "null" or "undefined"
		// they are just strings
		{
			category:              "Null values",
			query:                 "value=null",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['value'] = ? AND mapContains(attributes, 'value') = ?)",
			expectedArgs:          []any{"null", true},
			expectedErrorContains: "",
		},
		{
			category:              "Null values",
			query:                 "value=undefined",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['value'] = ? AND mapContains(attributes, 'value') = ?)",
			expectedArgs:          []any{"undefined", true},
			expectedErrorContains: "",
		},

		// Nested object paths
		{
			category:              "Nested object paths",
			query:                 "user.profile.name=\"John\"",
			shouldPass:            true,
			expectedQuery:         "WHERE (attributes['user.profile.name'] = ? AND mapContains(attributes, 'user.profile.name') = ?)",
			expectedArgs:          []any{"John", true},
			expectedErrorContains: "",
		},
		{
			category:              "Nested object paths",
			query:                 "request.headers.authorization EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE mapContains(attributes, 'request.headers.authorization') = ?",
			expectedArgs:          []any{true},
			expectedErrorContains: "",
		},
		{
			category:              "Nested object paths",
			query:                 "response.body.data.items[].id=123",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "key `response.body.data.items[].id` not found",
		},
		{
			category:              "Nested object paths",
			query:                 "metadata.dimensions.width>1000",
			shouldPass:            true,
			expectedQuery:         "WHERE (toFloat64(toFloat64OrNull(attributes['metadata.dimensions.width'])) > ? AND mapContains(attributes, 'metadata.dimensions.width') = ?)",
			expectedArgs:          []any{float64(1000), true},
			expectedErrorContains: "",
		},

		{category: "Only keywords", query: "AND", shouldPass: false, expectedErrorContains: "expecting one of {(, ), FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'AND'"},
		{category: "Only keywords", query: "OR", shouldPass: false, expectedErrorContains: "expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'OR'"},
		{category: "Only keywords", query: "NOT", shouldPass: false, expectedErrorContains: "expecting one of {(, ), FREETEXT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got EOF"},

		{category: "Only functions", query: "has", shouldPass: false, expectedErrorContains: "expecting one of {(, )} but got EOF"},
		{category: "Only functions", query: "hasAny", shouldPass: false, expectedErrorContains: "expecting one of {(, )} but got EOF"},
		{category: "Only functions", query: "hasAll", shouldPass: false, expectedErrorContains: "expecting one of {(, )} but got EOF"},

		{category: "Values without keys", query: "=200", shouldPass: false, expectedErrorContains: "but got '='"},
		{category: "Values without keys", query: "=\"api\"", shouldPass: false, expectedErrorContains: "but got '='"},
		{category: "Values without keys", query: ">10", shouldPass: false, expectedErrorContains: "but got '>'"},
		{category: "Values without keys", query: "BETWEEN 1 AND 10", shouldPass: false, expectedErrorContains: "but got 'BETWEEN'"},

		// Testing NOT > AND > OR precedence
		{
			category:      "Operator precedence",
			query:         "NOT status=200 AND service.name=\"api\"",
			shouldPass:    true,
			expectedQuery: "WHERE (NOT ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)) AND service = ?)",
			expectedArgs:  []any{float64(200), true, "api"}, // Should be (NOT status=200) AND service.name="api"
		},
		{
			category:      "Operator precedence",
			query:         "status=200 AND service.name=\"api\" OR service.name=\"web\"",
			shouldPass:    true,
			expectedQuery: "WHERE (((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ?) OR service = ?)",
			expectedArgs:  []any{float64(200), true, "api", "web"}, // Should be (status=200 AND service.name="api") OR service.name="web"
		},
		{
			category:      "Operator precedence",
			query:         "NOT status=200 OR NOT service.name=\"api\"",
			shouldPass:    true,
			expectedQuery: "WHERE (NOT ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)) OR NOT (service = ?))",
			expectedArgs:  []any{float64(200), true, "api"}, // Should be (NOT status=200) OR (NOT service.name="api")
		},
		{
			category:      "Operator precedence",
			query:         "status=200 OR service.name=\"api\" AND level=\"ERROR\"",
			shouldPass:    true,
			expectedQuery: "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) OR (service = ? AND (attributes['level'] = ? AND mapContains(attributes, 'level') = ?)))",
			expectedArgs:  []any{float64(200), true, "api", "ERROR", true}, // Should be status=200 OR (service.name="api" AND level="ERROR")
		},

		// Different whitespace patterns
		{
			category:              "Whitespace patterns",
			query:                 "status=200AND service.name=\"api\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text} but got '200AND'",
		},
		{
			category:              "Whitespace patterns",
			query:                 "status=200ANDservice.name=\"api\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text} but got '200ANDservice.name'",
		},
		{
			category:      "Whitespace patterns",
			query:         "status=200  AND     service.name=\"api\"",
			shouldPass:    true,
			expectedQuery: "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ?)",
			expectedArgs:  []any{float64(200), true, "api"}, // Multiple spaces
		},

		// More Unicode characters
		{
			category:      "Unicode",
			query:         "message=\"café\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['message'] = ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:  []any{"café", true},
		},
		{
			category:      "Unicode",
			query:         "user.name=\"Jörg Müller\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['user.name'] = ? AND mapContains(attributes, 'user.name') = ?)",
			expectedArgs:  []any{"Jörg Müller", true},
		},
		{
			category:      "Unicode",
			query:         "location=\"東京\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['location'] = ? AND mapContains(attributes, 'location') = ?)",
			expectedArgs:  []any{"東京", true},
		},
		{
			category:      "Unicode",
			query:         "description=\"Это тестовое сообщение\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['description'] = ? AND mapContains(attributes, 'description') = ?)",
			expectedArgs:  []any{"Это тестовое сообщение", true},
		},

		// Special characters
		{
			category:      "Special characters",
			query:         "path=\"/special/!@#$%^&*()\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['path'] = ? AND mapContains(attributes, 'path') = ?)",
			expectedArgs:  []any{"/special/!@#$%^&*()", true},
		},
		{
			category:      "Special characters",
			query:         "query=\"?param1=value1&param2=value2\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['query'] = ? AND mapContains(attributes, 'query') = ?)",
			expectedArgs:  []any{"?param1=value1&param2=value2", true},
		},

		// Special characters in keys
		{
			category:      "Special characters in keys",
			query:         "special-key=\"value\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['special-key'] = ? AND mapContains(attributes, 'special-key') = ?)",
			expectedArgs:  []any{"value", true}, // With hyphen
		},
		{
			category:      "Special characters in keys",
			query:         "special.key=\"value\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['special.key'] = ? AND mapContains(attributes, 'special.key') = ?)",
			expectedArgs:  []any{"value", true}, // With dot
		},
		{
			category:      "Special characters in keys",
			query:         "special_key=\"value\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['special_key'] = ? AND mapContains(attributes, 'special_key') = ?)",
			expectedArgs:  []any{"value", true}, // With underscore
		},

		// Using operator keywords as keys
		{
			category:              "Operator keywords as keys",
			query:                 "and=true",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "line 1:0 expecting one of {(, ), FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'and'",
		},
		{
			category:              "Operator keywords as keys",
			query:                 "or=false",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "line 1:0 expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'or'",
		},
		{
			category:              "Operator keywords as keys",
			query:                 "not=\"value\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "line 1:3 expecting one of {(, ), FREETEXT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got '='",
		},
		{
			category:              "Operator keywords as keys",
			query:                 "between=10",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "line 1:0 expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'between'",
		},
		{
			category:              "Operator keywords as keys",
			query:                 "in=\"list\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "line 1:0 expecting one of {(, ), AND, FREETEXT, NOT, boolean, has(), hasAll(), hasAny(), hasToken(), number, quoted text} but got 'in'",
		},

		// Using function keywords as keys
		{
			category:              "Function keywords as keys",
			query:                 "has=\"function\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {(, )} but got '='",
		},
		{
			category:              "Function keywords as keys",
			query:                 "hasAny=\"function\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {(, )} but got '='",
		},
		{
			category:              "Function keywords as keys",
			query:                 "hasAll=\"function\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {(, )} but got '='",
		},

		// Common filter patterns
		{
			category:      "Common filters",
			query:         "status IN (\"pending\", \"processing\", \"completed\") AND NOT is_deleted=true",
			shouldPass:    true,
			expectedQuery: "WHERE (((toString(toFloat64OrNull(attributes['status'])) = ? OR toString(toFloat64OrNull(attributes['status'])) = ? OR toString(toFloat64OrNull(attributes['status'])) = ?) AND mapContains(attributes, 'status') = ?) AND NOT ((attributes['is_deleted'] = 'true' = ? AND mapContains(attributes, 'is_deleted') = ?)))",
			expectedArgs:  []any{"pending", "processing", "completed", true, true, true},
		},
		{
			category:      "Common filters",
			query:         "(first_name LIKE \"John%\" OR last_name LIKE \"Smith%\") AND age>=18",
			shouldPass:    true,
			expectedQuery: "WHERE ((((attributes['first_name'] LIKE ? AND mapContains(attributes, 'first_name') = ?) OR (attributes['last_name'] LIKE ? AND mapContains(attributes, 'last_name') = ?))) AND (toFloat64(toFloat64OrNull(attributes['age'])) >= ? AND mapContains(attributes, 'age') = ?))",
			expectedArgs:  []any{"John%", true, "Smith%", true, float64(18), true},
		},
		{
			category:              "Common filters",
			query:                 "user_id=12345 AND (subscription_type=\"premium\" OR has(features, \"advanced\"))",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "key `user_id` not found",
		},

		// More common filter patterns
		{
			category:      "More common filters",
			query:         "service.name=\"api\" AND (status>=500 OR duration>1000) AND NOT message CONTAINS \"expected\"",
			shouldPass:    true,
			expectedQuery: "WHERE (service = ? AND (((toFloat64(toFloat64OrNull(attributes['status'])) >= ? AND mapContains(attributes, 'status') = ?) OR (toFloat64(toFloat64OrNull(attributes['duration'])) > ? AND mapContains(attributes, 'duration') = ?))) AND NOT ((attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?)))",
			expectedArgs:  []any{"api", float64(500), true, float64(1000), true, "%expected%", true},
		},

		// Edge cases
		{
			category:              "empty array",
			query:                 "key IN ()",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text}",
		},
		{
			category:              "empty array",
			query:                 "key IN []",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text}",
		},
		{
			category:              "empty array",
			query:                 "key NOT IN ()",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text}",
		},
		{
			category:              "empty array",
			query:                 "key NOT IN []",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text}",
		},

		// Values with escaping
		{
			category:      "Escaped values",
			query:         "message=\"Line 1\\nLine 2\\tTabbed\\rCarriage return\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['message'] = ? AND mapContains(attributes, 'message') = ?)",
			expectedArgs:  []any{"Line 1\\nLine 2\\tTabbed\\rCarriage return", true},
		},
		{
			category:      "Escaped values",
			query:         "path=\"C:\\\\Program Files\\\\Application\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['path'] = ? AND mapContains(attributes, 'path') = ?)",
			expectedArgs:  []any{"C:\\Program Files\\Application", true},
		},
		{
			category:      "Escaped values",
			query:         "path=\"^prefix\\\\.suffix$\\\\d+\\\\w+\"",
			shouldPass:    true,
			expectedQuery: "WHERE (attributes['path'] = ? AND mapContains(attributes, 'path') = ?)",
			expectedArgs:  []any{"^prefix\\.suffix$\\d+\\w+", true},
		},

		// Inconsistent/unusual whitespace
		{
			category:      "Unusual whitespace",
			query:         "status   =    200    AND   service.name    =   \"api\"",
			shouldPass:    true,
			expectedQuery: "WHERE ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?) AND service = ?)",
			expectedArgs:  []any{float64(200), true, "api"},
		},
		{
			category:              "Unusual whitespace",
			query:                 "status=200AND service.name=\"api\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text} but got '200AND'",
		},
		{
			category:              "Unusual whitespace",
			query:                 "status=200ANDservice.name=\"api\"",
			shouldPass:            false,
			expectedQuery:         "",
			expectedArgs:          nil,
			expectedErrorContains: "expecting one of {boolean, number, quoted text} but got '200ANDservice.name'",
		},

		// Very complex query example
		{
			category: "Complex query example",
			query: `
		  (
			(
			  (status>=200 AND status<300) OR
			  (status>=400 AND status<500 AND NOT status=404)
			) AND
			(
			  service.name IN ("api", "web", "auth") OR
			  (
				service.type="internal" AND
				NOT service.deprecated=true
			  )
			)
		  ) AND
		  (
			(
			  duration<1000 OR
			  (
				duration BETWEEN 1000 AND 5000
			  )
			) AND
			(
			  environment!="test" OR
			  (
				environment="test" AND
				is_automated_test=true
			  )
			)
		  ) AND
		  NOT (
			(
			  message CONTAINS "warning" OR
			  message CONTAINS "deprecated"
			) AND
			severity="low"
		  )
		`,
			shouldPass:    true,
			expectedQuery: "WHERE ((((((((toFloat64(toFloat64OrNull(attributes['status'])) >= ? AND mapContains(attributes, 'status') = ?) AND (toFloat64(toFloat64OrNull(attributes['status'])) < ? AND mapContains(attributes, 'status') = ?))) OR (((toFloat64(toFloat64OrNull(attributes['status'])) >= ? AND mapContains(attributes, 'status') = ?) AND (toFloat64(toFloat64OrNull(attributes['status'])) < ? AND mapContains(attributes, 'status') = ?) AND NOT ((toFloat64(toFloat64OrNull(attributes['status'])) = ? AND mapContains(attributes, 'status') = ?)))))) AND (((service = ? OR service = ? OR service = ?) OR (((attributes['service.type'] = ? AND mapContains(attributes, 'service.type') = ?) AND NOT ((attributes['service.deprecated'] = ? AND mapContains(attributes, 'service.deprecated') = ?)))))))) AND (((((toFloat64(toFloat64OrNull(attributes['duration'])) < ? AND mapContains(attributes, 'duration') = ?) OR ((toFloat64(toFloat64OrNull(attributes['duration'])) BETWEEN ? AND ? AND mapContains(attributes, 'duration') = ?)))) AND ((attributes['environment'] <> ? OR (((attributes['environment'] = ? AND mapContains(attributes, 'environment') = ?) AND (attributes['is_automated_test'] = 'true' = ? AND mapContains(attributes, 'is_automated_test') = ?))))))) AND NOT ((((((attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?) OR (attributes['message'] ILIKE ? AND mapContains(attributes, 'message') = ?))) AND (attributes['severity'] = ? AND mapContains(attributes, 'severity') = ?)))))",
			expectedArgs: []any{
				float64(200), true, float64(300), true, float64(400), true, float64(500), true, float64(404), true,
				"api", "web", "auth",
				"internal", true, true, true,
				float64(1000), true, float64(1000), float64(5000), true,
				"test", "test", true, true, true,
				"%warning%", true, "%deprecated%", true,
				"low", true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s: %s", tc.category, limitString(tc.query, 50)), func(t *testing.T) {

			clause, err := querybuilder.PrepareWhereClause(tc.query, opts)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Failed to parse query: %s\nError: %v\n", tc.query, err)
					return
				}

				if clause.IsEmpty() {
					t.Errorf("Expected clause for query: %s\n", tc.query)
					return
				}

				// Build the SQL and print it for debugging
				sql, args := clause.WhereClause.BuildWithFlavor(datastoresql.Flavor)

				require.Equal(t, tc.expectedQuery, sql)
				require.Equal(t, tc.expectedArgs, args)
			} else {
				require.Error(t, err, "Expected error for query: %s", tc.query)
				require.True(t, detailContains(err, tc.expectedErrorContains))
			}
		})
	}
}

// TestFilterExprLogs tests a comprehensive set of query patterns for logs search.
func TestFilterExprLogsConflictNegation(t *testing.T) {
	fl := flaggertest.New(t)
	fm := NewFieldMapper(fl)
	cb := NewConditionBuilder(fm, fl)

	// Define a comprehensive set of field keys to support all test cases
	releaseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	keys := buildCompleteFieldKeyMap(releaseTime)

	keys["body"] = []*telemetrytypes.TelemetryFieldKey{
		{
			Name:          "body",
			FieldContext:  telemetrytypes.FieldContextLog,
			FieldDataType: telemetrytypes.FieldDataTypeString,
		},
		{
			Name:          "body",
			FieldContext:  telemetrytypes.FieldContextAttribute,
			FieldDataType: telemetrytypes.FieldDataTypeString,
		},
	}

	opts := querybuilder.FilterExprVisitorOpts{
		Context:          context.Background(),
		Logger:           instrumentationtest.New().Logger(),
		FieldMapper:      fm,
		ConditionBuilder: cb,
		FieldKeys:        keys,
		FullTextColumn:   DefaultFullTextColumn,
		JsonKeyToKey:     GetBodyJSONKey,
	}

	testCases := []struct {
		category              string
		query                 string
		shouldPass            bool
		expectedQuery         string
		expectedArgs          []any
		expectedErrorContains string
	}{
		{
			category:              "not_contains",
			query:                 "body NOT CONTAINS 'done'",
			shouldPass:            true,
			expectedQuery:         "WHERE (body NOT ILIKE ? AND attributes['body'] NOT ILIKE ?)",
			expectedArgs:          []any{"%done%", "%done%"},
			expectedErrorContains: "",
		},
		{
			category:   "not_like",
			query:      "body NOT LIKE 'done'",
			shouldPass: true,
			// lower index search on body even for LIKE
			expectedQuery:         "WHERE (body NOT ILIKE ? AND attributes['body'] NOT LIKE ?)",
			expectedArgs:          []any{"done", "done"},
			expectedErrorContains: "",
		},
		{
			category:              "not_equal",
			query:                 "body != 'done'",
			shouldPass:            true,
			expectedQuery:         "WHERE (body <> ? AND attributes['body'] <> ?)",
			expectedArgs:          []any{"done", "done"},
			expectedErrorContains: "",
		},
		{
			category:              "not_regex",
			query:                 "body NOT REGEXP 'done'",
			shouldPass:            true,
			expectedQuery:         "WHERE (NOT match(LOWER(body), LOWER(?)) AND NOT match(attributes['body'], ?))",
			expectedArgs:          []any{"done", "done"},
			expectedErrorContains: "",
		},
		{
			category:              "exists",
			query:                 "body EXISTS",
			shouldPass:            true,
			expectedQuery:         "WHERE (body <> ? OR mapContains(attributes, 'body') = ?)",
			expectedArgs:          []any{"", true},
			expectedErrorContains: "",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s: %s", tc.category, limitString(tc.query, 50)), func(t *testing.T) {

			clause, err := querybuilder.PrepareWhereClause(tc.query, opts)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Failed to parse query: %s\nError: %v\n", tc.query, err)
					return
				}

				if clause.IsEmpty() {
					t.Errorf("Expected clause for query: %s\n", tc.query)
					return
				}

				// Build the SQL and print it for debugging
				sql, args := clause.WhereClause.BuildWithFlavor(datastoresql.Flavor)

				require.Equal(t, tc.expectedQuery, sql)
				require.Equal(t, tc.expectedArgs, args)
			} else {
				require.Error(t, err, "Expected error for query: %s", tc.query)
				require.True(t, detailContains(err, tc.expectedErrorContains))
			}
		})
	}
}
