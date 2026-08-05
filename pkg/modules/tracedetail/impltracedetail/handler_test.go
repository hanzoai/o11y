package impltracedetail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
)

// stubModule answers only the call under test; the embedded interface carries the
// rest, so this fake says exactly which method the route reaches.
type stubModule struct {
	tracedetail.Module
	gotTraceID string
}

func (s *stubModule) GetWaterfallV4(_ context.Context, traceID, _ string, _ []string) (*spantypes.GettableWaterfallTrace, error) {
	s.gotTraceID = traceID
	return &spantypes.GettableWaterfallTrace{}, nil
}

// The three trace-detail routes name their trace by a PATH segment and take the
// rest of the request as a BODY, so the only way the handler can address the
// right trace is by reading what the router matched. This drives the REAL
// template through a REAL router and asserts the module was asked about it.
func TestGetWaterfallActsOnTheTraceTheRouterMatched(t *testing.T) {
	module := &stubModule{}
	router := routing.Serve(http.MethodPost, "/v1/o11y/traces/{traceId}/waterfall", http.HandlerFunc(NewHandler(module).GetWaterfallV4))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/o11y/traces/0a1b2c3d/waterfall", strings.NewReader(`{}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "0a1b2c3d", module.gotTraceID)
}

// The trace id is a PATH segment: a query value of the same name must not answer
// for it, or a caller could name one trace in the path and read another.
func TestGetWaterfallIgnoresATraceIDQueryValue(t *testing.T) {
	module := &stubModule{}
	router := routing.Serve(http.MethodPost, "/v1/o11y/traces/{traceId}/waterfall", http.HandlerFunc(NewHandler(module).GetWaterfallV4))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/o11y/traces/0a1b2c3d/waterfall?traceID=deadbeef", strings.NewReader(`{}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "0a1b2c3d", module.gotTraceID)
}
