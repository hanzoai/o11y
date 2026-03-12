package queryparser

import (
	"context"

	"github.com/hanzoai/o11y/pkg/queryparser/queryfilterextractor"
	"github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
)

// QueryParser defines the interface for parsing and analyzing queries.
type QueryParser interface {
	// AnalyzeQueryFilter extracts filter conditions from a given query string.
	AnalyzeQueryFilter(ctx context.Context, queryType querybuildertypesv5.QueryType, query string) (*queryfilterextractor.FilterResult, error)
	// AnalyzeQueryEnvelopes extracts filter conditions from a list of query envelopes.
	// Returns a map of query name to FilterResult.
	AnalyzeQueryEnvelopes(ctx context.Context, queries []qbtypes.QueryEnvelope) (map[string]*queryfilterextractor.FilterResult, error)
}
