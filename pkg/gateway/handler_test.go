package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/gatewaytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
)

const orgID = "0198c4e2-4f6a-7c1a-8a2b-000000000001"

// recorder answers the five id-addressed calls and keeps the id it was handed.
// That id is the assertion: it is the path segment the ROUTER matched, so a
// handler reading the wrong half of the URL records "" and every row fails.
// The embedded interface carries the three list/create methods these routes
// never reach — nil, so a stray call panics loudly instead of passing quietly.
type recorder struct {
	Gateway
	id string
}

func (rec *recorder) UpdateIngestionKey(_ context.Context, _ valuer.UUID, keyID, _ string, _ []string, _ time.Time) error {
	rec.id = keyID
	return nil
}

func (rec *recorder) DeleteIngestionKey(_ context.Context, _ valuer.UUID, keyID string) error {
	rec.id = keyID
	return nil
}

func (rec *recorder) CreateIngestionKeyLimit(_ context.Context, _ valuer.UUID, keyID, _ string, _ gatewaytypes.LimitConfig, _ []string) (*gatewaytypes.GettableCreatedIngestionKeyLimit, error) {
	rec.id = keyID
	return &gatewaytypes.GettableCreatedIngestionKeyLimit{ID: keyID}, nil
}

func (rec *recorder) UpdateIngestionKeyLimit(_ context.Context, _ valuer.UUID, limitID string, _ gatewaytypes.LimitConfig, _ []string) error {
	rec.id = limitID
	return nil
}

func (rec *recorder) DeleteIngestionKeyLimit(_ context.Context, _ valuer.UUID, limitID string) error {
	rec.id = limitID
	return nil
}

// serve drives one request through a REAL router registered at template and
// returns what the gateway was handed, plus the response body for diagnosis.
func serve(t *testing.T, template, method, target, body string, pick func(Handler) http.HandlerFunc) (string, string) {
	t.Helper()

	rec := &recorder{}
	router := routing.Serve(method, template, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pick(NewHandler(rec))(w, r.WithContext(authtypes.NewContextWithClaims(r.Context(), authtypes.Claims{OrgID: orgID})))
	}))

	req := httptest.NewRequest(method, target, strings.NewReader(body))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return rec.id, w.Body.String()
}

func TestHandlerReadsTheIDTheRouterMatched(t *testing.T) {
	for _, c := range []struct {
		name     string
		template string
		method   string
		target   string
		body     string
		want     string
		pick     func(Handler) http.HandlerFunc
	}{
		{
			name:     "UpdateIngestionKey",
			template: "/v1/o11y/gateway/ingestion_keys/{keyId}",
			method:   http.MethodPut,
			target:   "/v1/o11y/gateway/ingestion_keys/key-1",
			body:     `{"name":"renamed"}`,
			want:     "key-1",
			pick:     func(h Handler) http.HandlerFunc { return h.UpdateIngestionKey },
		},
		{
			name:     "DeleteIngestionKey",
			template: "/v1/o11y/gateway/ingestion_keys/{keyId}",
			method:   http.MethodDelete,
			target:   "/v1/o11y/gateway/ingestion_keys/key-2",
			want:     "key-2",
			pick:     func(h Handler) http.HandlerFunc { return h.DeleteIngestionKey },
		},
		{
			name:     "CreateIngestionKeyLimit",
			template: "/v1/o11y/gateway/ingestion_keys/{keyId}/limits",
			method:   http.MethodPost,
			target:   "/v1/o11y/gateway/ingestion_keys/key-3/limits",
			body:     `{"signal":"logs","config":{}}`,
			want:     "key-3",
			pick:     func(h Handler) http.HandlerFunc { return h.CreateIngestionKeyLimit },
		},
		{
			name:     "UpdateIngestionKeyLimit",
			template: "/v1/o11y/gateway/ingestion_keys/limits/{limitId}",
			method:   http.MethodPut,
			target:   "/v1/o11y/gateway/ingestion_keys/limits/limit-1",
			body:     `{"config":{}}`,
			want:     "limit-1",
			pick:     func(h Handler) http.HandlerFunc { return h.UpdateIngestionKeyLimit },
		},
		{
			name:     "DeleteIngestionKeyLimit",
			template: "/v1/o11y/gateway/ingestion_keys/limits/{limitId}",
			method:   http.MethodDelete,
			target:   "/v1/o11y/gateway/ingestion_keys/limits/limit-2",
			want:     "limit-2",
			pick:     func(h Handler) http.HandlerFunc { return h.DeleteIngestionKeyLimit },
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, body := serve(t, c.template, c.method, c.target, c.body, c.pick)
			assert.Equal(t, c.want, got, "handler answered: %s", body)
		})
	}
}

// The id lives in the PATH. A query value of the same name is a different value
// carried by a different half of the URL, and the handler must not answer to it
// — otherwise a caller could address a key it never named in the route.
func TestHandlerDoesNotReadTheIDFromTheQuery(t *testing.T) {
	got, body := serve(t,
		"/v1/o11y/gateway/ingestion_keys/{keyId}",
		http.MethodDelete,
		"/v1/o11y/gateway/ingestion_keys/path-key?keyId=query-key",
		"",
		func(h Handler) http.HandlerFunc { return h.DeleteIngestionKey },
	)

	assert.Equal(t, "path-key", got, "handler answered: %s", body)
}
