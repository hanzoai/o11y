package implmetricreductionrule

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The three per-rule routes address a rule by a PATH segment, so what has to
// hold is that the segment the ROUTER matched is the id the handler acts on.
const ruleRoute = "/v1/o11y/metric_reduction_rules/{id}"

// servedRule drives one request through a REAL router registered at the real
// template and returns what idFromPath read. Injecting vars instead would prove
// only that the two halves of coretypes agree with each other.
func servedRule(t *testing.T, target string) (valuer.UUID, error) {
	t.Helper()

	var (
		id  valuer.UUID
		err error
	)
	router := mux.NewRouter()
	router.HandleFunc(ruleRoute, func(_ http.ResponseWriter, req *http.Request) {
		id, err = idFromPath(req)
	}).Methods(http.MethodGet)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the read")
	return id, err
}

func TestIDFromPathReadsTheMatchedSegment(t *testing.T) {
	want := valuer.GenerateUUID()

	id, err := servedRule(t, "/v1/o11y/metric_reduction_rules/"+want.String())

	require.NoError(t, err)
	assert.Equal(t, want, id)
}

// A segment that is not a uuid is refused, not answered with the zero uuid — a
// malformed id must never reach the module as a lookup for whatever the zero
// value matches.
func TestIDFromPathRefusesANonUUIDSegment(t *testing.T) {
	id, err := servedRule(t, "/v1/o11y/metric_reduction_rules/not-a-uuid")

	require.Error(t, err)
	assert.True(t, errors.Asc(err, errors.CodeInvalidInput), "want the invalid-input code, got %v", err)
	assert.Equal(t, valuer.UUID{}, id)
}
