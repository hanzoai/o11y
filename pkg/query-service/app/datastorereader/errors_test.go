package datastorereader

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/query-service/constants"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/stretchr/testify/assert"
)

// Exceptions read the event fact table, scoped to the caller's org, and never
// the retired error index.
func TestListErrorsQueryReadsFactByOrg(t *testing.T) {
	q := listErrorsQuery("event.fact", "", &model.ListErrorsParams{
		OrderParam: "lastSeen", Order: constants.Descending, Limit: 10, Offset: 5,
	})
	assert.Contains(t, q, "FROM event.fact WHERE org = @org AND signal = 'error'")
	assert.Contains(t, q, "GROUP BY issue")
	assert.Contains(t, q, "ORDER BY lastSeen DESC LIMIT @limit OFFSET @offset")
	assert.False(t, strings.Contains(q, "error_index"))
}

func TestErrorFactsNeedsTenant(t *testing.T) {
	r := &DatastoreReader{TraceDB: "event", errorTable: "fact"}
	_, _, apiErr := r.errorFacts(context.Background())
	if assert.NotNil(t, apiErr) {
		assert.Equal(t, model.ErrorForbidden, apiErr.Typ)
	}
}
