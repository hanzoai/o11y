package implrulestatehistory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/types/rulestatehistorytypes"
)

// stubModule answers only the call under test; the embedded interface carries the
// rest, so this fake says exactly which method the route reaches.
type stubModule struct {
	rulestatehistory.Module
	gotRuleID string
}

func (s *stubModule) GetHistoryStats(_ context.Context, ruleID string, _ rulestatehistorytypes.Query) (rulestatehistorytypes.GettableRuleStateHistoryStats, error) {
	s.gotRuleID = ruleID
	return rulestatehistorytypes.GettableRuleStateHistoryStats{}, nil
}

// Every history route names its rule by a PATH segment that sits in the MIDDLE of
// the template — /v1/o11y/rules/{id}/history/stats — and the window arrives as
// QUERY values beside it. Both halves of the URL are read in the same handler, so
// this drives the REAL template through a REAL router and asserts the module was
// asked about the rule the router matched.
func TestGetRuleHistoryStatsActsOnTheRuleTheRouterMatched(t *testing.T) {
	module := &stubModule{}
	router := routing.Serve(http.MethodGet, "/v1/o11y/rules/{id}/history/stats", http.HandlerFunc(NewHandler(module).GetRuleHistoryStats))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/o11y/rules/rule-7/history/stats?start=1&end=2", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "rule-7", module.gotRuleID)
}

// The id is a PATH segment: a query value of the same name must not answer for
// it, or a caller could name one rule in the path and read another's history.
func TestGetRuleHistoryStatsIgnoresAnIDQueryValue(t *testing.T) {
	module := &stubModule{}
	router := routing.Serve(http.MethodGet, "/v1/o11y/rules/{id}/history/stats", http.HandlerFunc(NewHandler(module).GetRuleHistoryStats))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/o11y/rules/rule-7/history/stats?start=1&end=2&id=rule-9", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "rule-7", module.gotRuleID)
}
