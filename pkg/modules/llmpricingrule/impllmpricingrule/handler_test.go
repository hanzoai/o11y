package impllmpricingrule

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/llmpricingruletypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The two per-rule routes address a rule by a PATH segment, so what has to hold
// is that the segment the ROUTER matched is the id the handler acts on. Reading
// it through coretypes.Param instead of naming a router is the whole change;
// this pins the behaviour that change must preserve.
func servedRule(t *testing.T, target string) (valuer.UUID, error) {
	t.Helper()

	var (
		id  valuer.UUID
		err error
	)
	router := mux.NewRouter()
	router.HandleFunc("/v1/o11y/llm_pricing_rules/{id}", func(_ http.ResponseWriter, req *http.Request) {
		id, err = ruleIDFromPath(req)
	}).Methods(http.MethodGet)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the read")
	return id, err
}

func TestRuleIDFromPathReadsTheMatchedSegment(t *testing.T) {
	want := valuer.GenerateUUID()

	id, err := servedRule(t, "/v1/o11y/llm_pricing_rules/"+want.String())

	require.NoError(t, err)
	assert.Equal(t, want, id)
}

// A segment that is not a uuid is refused with the resource's own code, not
// answered with the zero uuid — an empty or malformed id must never reach the
// module as a lookup for "whatever the zero value matches".
func TestRuleIDFromPathRefusesANonUUIDSegment(t *testing.T) {
	id, err := servedRule(t, "/v1/o11y/llm_pricing_rules/not-a-uuid")

	require.Error(t, err)
	assert.True(t, errors.Asc(err, llmpricingruletypes.ErrCodePricingRuleInvalidInput), "want the pricing-rule invalid-input code, got %v", err)
	assert.Equal(t, valuer.UUID{}, id)
}
