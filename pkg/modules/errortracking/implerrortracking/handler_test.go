package implerrortracking

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIngestFixture(t *testing.T, secret []byte) (errortracking.Handler, errortracking.Module) {
	t.Helper()
	mod := NewModule(NewStore(newTestStore(t)), NewNoopSink())
	return NewHandler(mod, secret, false, nil), mod
}

func envelopeFor(project string) []byte {
	return []byte(`{"event_id":"deadbeef","dsn":"https://k@h/` + project + `"}
{"type":"event"}
{"event_id":"deadbeef","platform":"python","exception":{"values":[{"type":"ValueError","value":"bad input","stacktrace":{"frames":[{"function":"handle","module":"app.svc","in_app":true}]}}]}}
`)
}

func ingestReq(project, key string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/"+project+"/envelope/", bytes.NewReader(body))
	if key != "" {
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_key="+key)
	}
	return mux.SetURLVars(req, map[string]string{"project_id": project})
}

func TestIngest_EndToEnd_ValidKeyStoresIssueUnderResolvedOrg(t *testing.T) {
	secret := []byte("kms-secret")
	h, mod := newIngestFixture(t, secret)

	key := publicKeyFor(secret, "acme")
	w := httptest.NewRecorder()
	h.EnvelopeIngest(w, ingestReq("acme", key, envelopeFor("acme")))
	require.Equal(t, http.StatusOK, w.Code)

	orgID, _ := orgUUIDFromProject("acme")
	list, total, err := mod.ListIssues(context.Background(), orgID, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, list, 1)
	assert.Equal(t, "ValueError", list[0].Type)
	assert.Equal(t, "bad input", list[0].Value)
	assert.Equal(t, "python", list[0].Platform)
}

func TestIngest_RejectsBadKey(t *testing.T) {
	secret := []byte("kms-secret")
	h, mod := newIngestFixture(t, secret)

	w := httptest.NewRecorder()
	h.EnvelopeIngest(w, ingestReq("acme", "wrong-key", envelopeFor("acme")))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	orgID, _ := orgUUIDFromProject("acme")
	_, total, err := mod.ListIssues(context.Background(), orgID, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, total, "a rejected ingest must persist nothing")
}

func TestIngest_MissingKeyRejected(t *testing.T) {
	secret := []byte("kms-secret")
	h, _ := newIngestFixture(t, secret)
	w := httptest.NewRecorder()
	h.EnvelopeIngest(w, ingestReq("acme", "", envelopeFor("acme")))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// A key minted for org "acme" must not authorize writing to a different project.
func TestIngest_CrossOrgKeyRejected(t *testing.T) {
	secret := []byte("kms-secret")
	h, mod := newIngestFixture(t, secret)

	acmeKey := publicKeyFor(secret, "acme")
	w := httptest.NewRecorder()
	// Present acme's key but target project "victim".
	h.EnvelopeIngest(w, ingestReq("victim", acmeKey, envelopeFor("victim")))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	victimOrg, _ := orgUUIDFromProject("victim")
	_, total, err := mod.ListIssues(context.Background(), victimOrg, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, total, "acme's key must not write into victim's org")
}

func TestIngest_DisabledWithoutSecret(t *testing.T) {
	h, _ := newIngestFixture(t, nil) // no KMS secret => ingest disabled
	w := httptest.NewRecorder()
	h.EnvelopeIngest(w, ingestReq("acme", "anything", envelopeFor("acme")))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestIngest_LegacyStoreEndpoint(t *testing.T) {
	secret := []byte("kms-secret")
	h, mod := newIngestFixture(t, secret)
	key := publicKeyFor(secret, "acme")

	body := []byte(`{"event_id":"1","exception":{"values":[{"type":"KeyError","value":"missing"}]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/acme/store/", bytes.NewReader(body))
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key="+key)
	req = mux.SetURLVars(req, map[string]string{"project_id": "acme"})
	w := httptest.NewRecorder()
	h.StoreIngest(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	orgID, _ := orgUUIDFromProject("acme")
	_, total, err := mod.ListIssues(context.Background(), orgID, &errortrackingtypes.IssuesQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}
