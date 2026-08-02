package o11yruler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Both per-id families here — a rule and a downtime schedule — name their
// resource by a PATH segment, so what has to hold is that the segment the ROUTER
// matched is the id the handler acts on. servedID drives one request through a
// REAL router registered at the REAL template: asserting the reader against the
// injector instead would prove only that the two halves of coretypes agree with
// each other.
func servedID(t *testing.T, template, target string) (valuer.UUID, error) {
	t.Helper()

	var (
		id  valuer.UUID
		err error
	)
	router := routing.Serve(http.MethodGet, template, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		id, err = idFromPath(req)
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the read")
	return id, err
}

func TestIDFromPathReadsTheMatchedRuleSegment(t *testing.T) {
	want := valuer.GenerateUUID()

	id, err := servedID(t, "/v1/o11y/rules/{id}", "/v1/o11y/rules/"+want.String())

	require.NoError(t, err)
	assert.Equal(t, want, id)
}

func TestIDFromPathReadsTheMatchedScheduleSegment(t *testing.T) {
	want := valuer.GenerateUUID()

	id, err := servedID(t, "/v1/o11y/downtime_schedules/{id}", "/v1/o11y/downtime_schedules/"+want.String())

	require.NoError(t, err)
	assert.Equal(t, want, id)
}

// The id is a PATH segment and nothing else: a query value of the same name must
// not answer for it, or a caller could name one rule in the path and act on
// another.
func TestIDFromPathIgnoresAQueryValueOfTheSameName(t *testing.T) {
	want := valuer.GenerateUUID()
	other := valuer.GenerateUUID()

	id, err := servedID(t, "/v1/o11y/rules/{id}", "/v1/o11y/rules/"+want.String()+"?id="+other.String())

	require.NoError(t, err)
	assert.Equal(t, want, id)
}

// A segment that is not a uuid is refused, not answered with the zero uuid — a
// malformed id must never reach the ruler as a lookup for "whatever the zero
// value matches".
func TestIDFromPathRefusesANonUUIDSegment(t *testing.T) {
	id, err := servedID(t, "/v1/o11y/rules/{id}", "/v1/o11y/rules/not-a-uuid")

	require.Error(t, err)
	assert.True(t, errors.Asc(err, errors.CodeInvalidInput), "want the invalid-input code, got %v", err)
	assert.Equal(t, valuer.UUID{}, id)
}
