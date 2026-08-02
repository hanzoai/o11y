package implspanmapper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The two registered templates, verbatim from the apiserver. The names matter as
// much as the shapes: the group id is carried as {groupId} on BOTH, so a helper
// reading "id" would answer "" on every route and the mappers of one group would
// be served as the mappers of none.
const (
	groupRoute  = "/v1/o11y/span_mapper_groups/{groupId}"
	mapperRoute = "/v1/o11y/span_mapper_groups/{groupId}/span_mappers/{mapperId}"
)

// served drives one request through a REAL router registered at template and
// hands back the request the handler saw, so the helpers below are exercised
// against what the ROUTER matched rather than against injected vars.
func served(t *testing.T, template, target string) *http.Request {
	t.Helper()

	var got *http.Request
	router := routing.Serve(http.MethodGet, template, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { got = req }))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the read")
	require.NotNil(t, got)
	return got
}

func TestPathIDsAreTheSegmentsTheRouterMatched(t *testing.T) {
	group, mapper := valuer.GenerateUUID(), valuer.GenerateUUID()

	req := served(t, mapperRoute, "/v1/o11y/span_mapper_groups/"+group.String()+"/span_mappers/"+mapper.String())

	gotGroup, err := groupIDFromPath(req)
	require.NoError(t, err)
	assert.Equal(t, group, gotGroup)

	gotMapper, err := mapperIDFromPath(req)
	require.NoError(t, err)
	assert.Equal(t, mapper, gotMapper)
}

// The group routes carry the same segment name one level up, and the group
// handlers read it with the same helper.
func TestGroupIDIsReadOnTheGroupRoute(t *testing.T) {
	group := valuer.GenerateUUID()

	got, err := groupIDFromPath(served(t, groupRoute, "/v1/o11y/span_mapper_groups/"+group.String()))

	require.NoError(t, err)
	assert.Equal(t, group, got)
}

// A segment that is not a uuid is refused with the mapping resource's own code,
// not answered with the zero uuid — an empty or malformed id must never reach the
// module as a lookup for whatever the zero value matches.
func TestPathIDsRefuseANonUUIDSegment(t *testing.T) {
	req := served(t, mapperRoute, "/v1/o11y/span_mapper_groups/not-a-uuid/span_mappers/also-not-a-uuid")

	group, err := groupIDFromPath(req)
	require.Error(t, err)
	assert.True(t, errors.Asc(err, spantypes.ErrCodeMappingInvalidInput), "want the mapping invalid-input code, got %v", err)
	assert.Equal(t, valuer.UUID{}, group)

	mapper, err := mapperIDFromPath(req)
	require.Error(t, err)
	assert.True(t, errors.Asc(err, spantypes.ErrCodeMappingInvalidInput), "want the mapping invalid-input code, got %v", err)
	assert.Equal(t, valuer.UUID{}, mapper)
}
