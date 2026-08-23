package o11y_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/types"
	citypes "github.com/hanzoai/o11y/pkg/types/cloudintegrationtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/zap-proto/zip"
)

// THE CLOUD-INTEGRATIONS FACE, under proof.
//
// The thirteen cloud_integrations routes are typed ops registered by
// mountIntegrations (integrations.go); this file is the face's evidence, and it
// counts ONLY this face. The census matches on the two path roots the face
// owns — /v1/o11y/cloud_integrations/ and the hyphenated
// /v1/o11y/cloud-integrations/ — never on a bare substring like "/service" or
// "/list", which belong to the APM and access faces too. apm_test.go had to
// grow three exclusions the day this face converted; a prefix needs none.
//
// The gates are NOT asserted here because they are not enforced here. Each op
// hands the call to the same runtime handler the delegation wildcard reached
// (integrationsRelay), so the AdminAccess / ViewAccess middleware declared in
// pkg/apiserver/o11yapiserver/cloudintegration.go stays the one place access is
// decided. What this file pins is that the seam cannot BYPASS that decision:
// identity is propagated and never minted (TestCloudIntegrationsIdentityIsPropagated),
// a refusal keeps the runtime's status (TestCloudIntegrationsRefusalKeepsTheRuntimeStatus),
// and no runtime means no answer (TestCloudIntegrationsFailClosedWithoutARuntime).
//
// The helpers — mounted, runtime, call, member, mustJSON, assertBodyRoundTrips —
// live in telemetry_test.go and apm_test.go.

// ── the payloads, every field populated ──────────────────────────────────────

func ciAt() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC) }

// fullCredentials is the connecting agent's credentials with all four fields set.
func fullCredentials() *citypes.Credentials {
	return &citypes.Credentials{
		O11yAPIURL:   "https://api.hanzo.ai/v1/o11y",
		O11yAPIKey:   "pat_maxpower_7",
		IngestionURL: "https://ingest.hanzo.ai",
		IngestionKey: "ik_9f3c",
	}
}

// fullAccounts is one connected AWS account with every field of Account
// populated, including the nested config and the agent's last report.
func fullAccounts() *citypes.GettableAccounts {
	id := "615954921216"
	removed := ciAt().Add(-time.Hour)
	return &citypes.GettableAccounts{Accounts: []*citypes.Account{{
		Identifiable:      types.Identifiable{ID: valuer.MustNewUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")},
		TimeAuditable:     types.TimeAuditable{CreatedAt: ciAt(), UpdatedAt: ciAt().Add(time.Minute)},
		ProviderAccountID: &id,
		Provider:          citypes.CloudProviderTypeAWS,
		RemovedAt:         &removed,
		AgentReport: &citypes.AgentReport{
			TimestampMillis: 1753963200000,
			Data:            map[string]any{"agent_version": "0.4.1", "regions": []any{"us-east-1"}},
		},
		OrgID:  valuer.MustNewUUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8"),
		Config: &citypes.AccountConfig{AWS: &citypes.AWSAccountConfig{Regions: []string{"us-east-1", "eu-west-2"}}},
	}}}
}

// fullServicesMetadata is a services listing with the definition metadata and
// the per-account enabled flag populated.
func fullServicesMetadata() *citypes.GettableServicesMetadata {
	return &citypes.GettableServicesMetadata{Services: []*citypes.ServiceMetadata{{
		ServiceDefinitionMetadata: citypes.ServiceDefinitionMetadata{ID: "ec2", Title: "EC2", Icon: "ec2.svg"},
		Enabled:                   true,
	}, {
		ServiceDefinitionMetadata: citypes.ServiceDefinitionMetadata{ID: "s3", Title: "S3", Icon: "s3.svg"},
		Enabled:                   false,
	}}}
}

// fullService is one collectable service with its overview, assets, supported
// signals, collected data and the account's own service configuration.
func fullService() *citypes.Service {
	return &citypes.Service{
		ServiceDefinitionMetadata: citypes.ServiceDefinitionMetadata{ID: "ec2", Title: "EC2", Icon: "ec2.svg"},
		Overview:                  "# EC2\nCollects EC2 metrics and logs.",
		ServiceAssets: citypes.ServiceAssets{Dashboards: []*citypes.ServiceDashboard{{
			Title: "EC2 overview", Description: "hosts, cpu, disk",
		}}},
		SupportedSignals: citypes.SupportedSignals{Logs: true, Metrics: true},
		DataCollected: citypes.DataCollected{
			Logs:    []citypes.CollectedLogAttribute{{Name: "message", Path: "body", Type: "string"}},
			Metrics: []citypes.CollectedMetric{{Name: "cpu_utilization", Type: "gauge", Unit: "percent", Description: "cpu"}},
		},
		CloudIntegrationService: &citypes.CloudIntegrationService{
			Type:               citypes.AWSServiceEC2,
			CloudIntegrationID: valuer.MustNewUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			Config: &citypes.ServiceConfig{AWS: &citypes.AWSServiceConfig{
				Logs:    &citypes.AWSServiceLogsConfig{Enabled: true, S3Buckets: map[string][]string{"us-east-1": {"logs-bucket"}}},
				Metrics: &citypes.AWSServiceMetricsConfig{Enabled: true},
			}},
		},
	}
}

// fullCheckIn is the runtime's check-in answer, carrying BOTH the current
// fields and the snake_case ones kept for already-deployed agents.
func fullCheckIn() *citypes.GettableAgentCheckIn {
	removed := ciAt()
	return &citypes.GettableAgentCheckIn{
		AccountID:      "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		CloudAccountID: "615954921216",
		OlderIntegrationConfig: &citypes.IntegrationConfig{
			EnabledRegions: []string{"us-east-1"},
			Telemetry:      &citypes.OldAWSCollectionStrategy{Provider: "aws", S3Buckets: map[string][]string{"us-east-1": {"logs-bucket"}}},
		},
		OlderRemovedAt: &removed,
		AgentCheckInResponse: citypes.AgentCheckInResponse{
			CloudIntegrationID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			ProviderAccountID:  "615954921216",
			IntegrationConfig:  &citypes.ProviderIntegrationConfig{AWS: &citypes.AWSIntegrationConfig{}},
			RemovedAt:          &removed,
		},
	}
}

// ── the wire proofs ──────────────────────────────────────────────────────────

// The credentials read answers with the runtime's bytes and asks the runtime for
// the provider the caller named.
func TestConnectionCredentialsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullCredentials())

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/cloud_integrations/aws/credentials", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/cloud_integrations/aws/credentials" || r.Method != http.MethodGet {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
}

// The account listing survives the typed round trip to the byte — a dropped or
// renamed field of Account, its nested config or its agent report shows here.
func TestListAccountsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullAccounts())

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/cloud_integrations/aws/accounts" {
		t.Fatalf("runtime was asked %s", r.URL.Path)
	}
}

// The single-account read puts the id in the path, where the mux registration
// spells it, and answers with the runtime's bytes.
func TestGetAccountCarriesTheIDInThePath(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &fullAccounts().Accounts[0])

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/cloud_integrations/gcp/accounts/acct-7", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/cloud_integrations/gcp/accounts/acct-7" {
		t.Fatalf("runtime was asked %s, want the provider and id in the path", r.URL.Path)
	}
}

// The connect answers 201 — the status the mux registration declared — and the
// posted account reaches the runtime as the runtime's OWN request type, field
// for field, credentials included.
func TestCreateAccountForwardsThePostedAccountAndAnswers201(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, &citypes.GettableAccountWithConnectionArtifact{
		ID:                 valuer.MustNewUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		ConnectionArtifact: &citypes.ConnectionArtifact{AWS: &citypes.AWSConnectionArtifact{}},
	})

	sent := `{"config":{"AgentVersion":"0.4.1","aws":{"regions":["us-east-1","eu-west-2"]}},` +
		`"credentials":{"o11yApiUrl":"https://api.hanzo.ai/v1/o11y","o11yApiKey":"pat_maxpower_7",` +
		`"ingestionUrl":"https://ingest.hanzo.ai","ingestionKey":"ik_9f3c"}}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/cloud_integrations/aws/accounts", strings.NewReader(sent)))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", status, got)
	}
	if r := *asked; r.Method != http.MethodPost || r.URL.Path != "/v1/o11y/cloud_integrations/aws/accounts" {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	assertBodyRoundTrips[citypes.PostableAccount](t, sent, *asked)
}

// The account update is a PUT on the account path carrying the new config, and
// answers 204 — the status zip gives an op whose handler returns a nil Out,
// which is the status the mux registration declared.
func TestUpdateAccountForwardsTheConfigAndAnswers204(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, nil)

	sent := `{"config":{"aws":{"regions":["us-east-1"]}}}`
	status, got := call(t, app, member(http.MethodPut, "/v1/o11y/cloud_integrations/aws/accounts/acct-7", strings.NewReader(sent)))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if r := *asked; r.Method != http.MethodPut || r.URL.Path != "/v1/o11y/cloud_integrations/aws/accounts/acct-7" {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	assertBodyRoundTrips[citypes.UpdatableAccount](t, sent, *asked)
}

// The disconnect carries no body and answers 204.
func TestDisconnectAccountIsADeleteOnTheAccountPath(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, nil)

	status, got := call(t, app, member(http.MethodDelete, "/v1/o11y/cloud_integrations/azure/accounts/acct-7", nil))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if r := *asked; r.Method != http.MethodDelete || r.URL.Path != "/v1/o11y/cloud_integrations/azure/accounts/acct-7" {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
}

// The services listing carries the cloud_integration_id scope through as a
// QUERY parameter — the runtime reads it from the query string, not the body,
// and a typed op that moved it would silently widen the listing.
func TestListServicesMetadataCarriesTheScopeAsQuery(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullServicesMetadata())

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/cloud_integrations/aws/services?cloud_integration_id=6ba7b810-9dad-11d1-80b4-00c04fd430c8", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	r := *asked
	if r.URL.Path != "/v1/o11y/cloud_integrations/aws/services" {
		t.Fatalf("runtime was asked %s", r.URL.Path)
	}
	if q := r.URL.Query().Get("cloud_integration_id"); q != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Fatalf("cloud_integration_id reached the runtime as %q", q)
	}
}

// The per-account services listing is the same answer under the account path.
func TestListAccountServicesMetadataIsUnderTheAccount(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullServicesMetadata())

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services" {
		t.Fatalf("runtime was asked %s", r.URL.Path)
	}
}

// The service read carries the service id in the path AND the optional scope in
// the query, and answers with the runtime's bytes — overview, assets, collected
// data and the account's own service config included.
func TestGetServiceCarriesPathAndScope(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullService())

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/cloud_integrations/aws/services/ec2?cloud_integration_id=6ba7b810-9dad-11d1-80b4-00c04fd430c8", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	r := *asked
	if r.URL.Path != "/v1/o11y/cloud_integrations/aws/services/ec2" {
		t.Fatalf("runtime was asked %s", r.URL.Path)
	}
	if q := r.URL.Query().Get("cloud_integration_id"); q != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Fatalf("cloud_integration_id reached the runtime as %q", q)
	}
}

// The account-service read is the deeper path — account id AND service id —
// and is a DIFFERENT op from the provider-wide service read above.
func TestGetAccountServiceIsTheDeeperPath(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullService())

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services/ec2", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services/ec2" {
		t.Fatalf("runtime was asked %s, want the account and service ids", r.URL.Path)
	}
}

// The service update carries the new config to the account's service path and
// answers 204.
func TestUpdateServiceForwardsTheConfigAndAnswers204(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, nil)

	sent := `{"config":{"aws":{"logs":{"enabled":true,"s3Buckets":{"us-east-1":["logs-bucket"]}},"metrics":{"enabled":false}}}}`
	status, got := call(t, app, member(http.MethodPut,
		"/v1/o11y/cloud_integrations/aws/accounts/acct-7/services/ec2", strings.NewReader(sent)))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if r := *asked; r.Method != http.MethodPut || r.URL.Path != "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services/ec2" {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	assertBodyRoundTrips[citypes.UpdatableService](t, sent, *asked)
}

// The two check-in bodies the runtime accepts. PostableAgentCheckIn's own
// UnmarshalJSON refuses a body carrying BOTH the snake_case fields older agents
// send and the camelCase ones current agents send — "either, not both" — so
// each path is exercised with the dialect its callers actually speak.
const (
	oldAgentCheckIn = `{"account_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","cloud_account_id":"615954921216","data":{"agent_version":"0.4.1"}}`
	newAgentCheckIn = `{"cloudIntegrationId":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","providerAccountId":"615954921216","data":{"agent_version":"0.4.1"}}`
)

// BOTH check-ins reach the runtime, each on ITS OWN path. The hyphenated
// agent-check-in is the one already-deployed agents call and must keep its
// spelling; accounts/check_in is its replacement. Two ops, two paths, one
// handler — and the deprecated one must not be silently rewritten onto the new
// path, or old agents would start hitting a route their build never named.
//
// The body must also survive the seam INTACT: the op re-marshals the check-in
// on its way through, and a re-marshal that emitted the other dialect's zero
// values alongside the caller's own would trip the runtime's "either, not both"
// validator — a 400 the caller never earned. assertBodyRoundTrips decodes the
// forwarded bytes through that very validator, so this pins it.
func TestBothAgentCheckInsKeepTheirOwnPath(t *testing.T) {
	for _, tc := range []struct{ name, target, sent string }{
		{"deprecated hyphenated path", "/v1/o11y/cloud-integrations/aws/agent-check-in", oldAgentCheckIn},
		{"account check_in path", "/v1/o11y/cloud_integrations/aws/accounts/check_in", newAgentCheckIn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			wrote, asked := runtime(t, fullCheckIn())

			status, got := call(t, app, member(http.MethodPost, tc.target, strings.NewReader(tc.sent)))
			if status != http.StatusOK {
				t.Fatalf("status=%d body=%s, want 200", status, got)
			}
			if !bytes.Equal(got, *wrote) {
				t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
			}
			if r := *asked; r.Method != http.MethodPost || r.URL.Path != tc.target {
				t.Fatalf("runtime was asked %s %s, want POST %s", r.Method, r.URL.Path, tc.target)
			}
			assertBodyRoundTrips[citypes.PostableAgentCheckIn](t, tc.sent, *asked)
		})
	}
}

// ── census, document, identity, refusal, fail-closed ─────────────────────────

// THE ROUTES, exactly the thirteen the mux registers in
// pkg/apiserver/o11yapiserver/cloudintegration.go: eleven on the underscored
// root, the accounts/check_in twelfth, and the hyphenated agent-check-in that
// is its own real path. No more (the face grew no route) and no fewer (every
// route reached the document).
func TestCloudIntegrationsRoutesAreTheSameThirteen(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{
		"GET /v1/o11y/cloud_integrations/:cloud_provider/credentials":                       true,
		"POST /v1/o11y/cloud_integrations/:cloud_provider/accounts":                         true,
		"GET /v1/o11y/cloud_integrations/:cloud_provider/accounts":                          true,
		"GET /v1/o11y/cloud_integrations/:cloud_provider/accounts/:id":                      true,
		"PUT /v1/o11y/cloud_integrations/:cloud_provider/accounts/:id":                      true,
		"DELETE /v1/o11y/cloud_integrations/:cloud_provider/accounts/:id":                   true,
		"GET /v1/o11y/cloud_integrations/:cloud_provider/services":                          true,
		"GET /v1/o11y/cloud_integrations/:cloud_provider/accounts/:id/services":             true,
		"GET /v1/o11y/cloud_integrations/:cloud_provider/services/:service_id":              true,
		"PUT /v1/o11y/cloud_integrations/:cloud_provider/accounts/:id/services/:service_id": true,
		"GET /v1/o11y/cloud_integrations/:cloud_provider/accounts/:id/services/:service_id": true,
		"POST /v1/o11y/cloud_integrations/:cloud_provider/accounts/check_in":                true,
		"POST /v1/o11y/cloud-integrations/:cloud_provider/agent-check-in":                   true,
	}
	if len(want) != 13 {
		t.Fatalf("the census itself is wrong: %d", len(want))
	}

	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		// TWO ROOTS, matched as PREFIXES. A substring would be wrong three ways
		// here: "/services" is also APM's and access's, "/accounts" is also the
		// service-account face's, and "/integrations" alone would swallow the
		// integration catalog that shares integrations.go. The prefix is the
		// only predicate that names this face and nothing else.
		if strings.HasPrefix(r.Path, "/v1/o11y/cloud_integrations/") ||
			strings.HasPrefix(r.Path, "/v1/o11y/cloud-integrations/") {
			got[r.Method+" "+r.Path] = true
		}
	}
	for route := range want {
		if !got[route] {
			t.Errorf("%s is not registered", route)
		}
	}
	for route := range got {
		if !want[route] {
			t.Errorf("%s is registered and was not before — the face grew a route", route)
		}
	}
}

// THE POINT OF THE PORT: all thirteen operations are in the document, each with
// the operation id the face has always published on mux and each with its prose.
// A route behind the delegation wildcard had none of that — no SDK method, no
// CLI command, no agent tool, no reference page — which is what kept an operator
// from connecting an AWS account with anything but a hand-written HTTP call.
func TestCloudIntegrationsReachTheDocument(t *testing.T) {
	app := mounted(t)
	raw, err := json.Marshal(app.OpenAPISpec())
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("spec: %v", err)
	}

	// path -> method -> the operation id the mux registration declared. The
	// document spells parameters in braces; the router spells them with a colon.
	for path, byMethod := range map[string]map[string]string{
		"/v1/o11y/cloud_integrations/{cloud_provider}/credentials": {"get": "GetConnectionCredentials"},
		"/v1/o11y/cloud_integrations/{cloud_provider}/accounts": {
			"post": "CreateAccount", "get": "ListAccounts",
		},
		"/v1/o11y/cloud_integrations/{cloud_provider}/accounts/{id}": {
			"get": "GetAccount", "put": "UpdateAccount", "delete": "DisconnectAccount",
		},
		"/v1/o11y/cloud_integrations/{cloud_provider}/services":               {"get": "ListServicesMetadata"},
		"/v1/o11y/cloud_integrations/{cloud_provider}/accounts/{id}/services": {"get": "ListAccountServicesMetadata"},
		"/v1/o11y/cloud_integrations/{cloud_provider}/services/{service_id}":  {"get": "GetService"},
		"/v1/o11y/cloud_integrations/{cloud_provider}/accounts/{id}/services/{service_id}": {
			"put": "UpdateService", "get": "GetAccountService",
		},
		"/v1/o11y/cloud_integrations/{cloud_provider}/accounts/check_in": {"post": "AgentCheckIn"},
		"/v1/o11y/cloud-integrations/{cloud_provider}/agent-check-in":    {"post": "AgentCheckInDeprecated"},
	} {
		for method, id := range byMethod {
			op, ok := spec.Paths[path][method]
			if !ok {
				t.Errorf("%s %s is not in the document", strings.ToUpper(method), path)
				continue
			}
			if op.OperationID != id {
				t.Errorf("%s %s is documented as %q, want %q — the id the mux declared", strings.ToUpper(method), path, op.OperationID, id)
			}
			if len(op.Summary) < 20 {
				t.Errorf("%s %s has no prose in the document: %q", strings.ToUpper(method), path, op.Summary)
			}
		}
	}
}

// The caller's identity travels to the runtime as the gateway asserted it —
// PROPAGATED, never minted. This is what keeps the runtime's AdminAccess gate
// the one enforcement point: it re-resolves the role from these headers, so an
// op that dropped them would fail closed and an op that forged them would be
// the bypass. The seam does neither.
func TestCloudIntegrationsIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, fullCredentials())

	r := member(http.MethodGet, "/v1/o11y/cloud_integrations/aws/credentials", nil)
	r.Header.Set(zip.HeaderUserAdmin, "true")
	if status, body := call(t, app, r); status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	got := (*asked).Header
	for header, want := range map[string]string{
		zip.HeaderOrg:       "maxpower",
		zip.HeaderUser:      "z",
		zip.HeaderUserEmail: "z@hanzo.ai",
		zip.HeaderUserAdmin: "true",
	} {
		if got.Get(header) != want {
			t.Errorf("%s reached the runtime as %q, want %q", header, got.Get(header), want)
		}
	}
}

// THE GATE IS THE RUNTIME'S. When the runtime's AdminAccess middleware refuses,
// the op hands the caller that refusal — same status, same reason — instead of
// swallowing it into a 200 or a 500. A viewer calling an admin-gated account
// route sees the 403 it always saw.
func TestCloudIntegrationsRefusalKeepsTheRuntimeStatus(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"status":"error","errorType":"forbidden","error":"caller is not an admin"}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts", nil))
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (the runtime's own)", status)
	}
	if !strings.Contains(string(got), "caller is not an admin") {
		t.Fatalf("the reason was lost: %s", got)
	}
}

// No runtime, no answer: ALL THIRTEEN routes of this face fail closed with the
// same 503 the delegation wildcard gives before a handler is registered. An op
// that answered anything else would be answering without the gate having run.
//
// Each write carries a body its own In accepts. That is not test convenience —
// the ops embed the runtime's PostableAccount / UpdatableAccount /
// UpdatableService / PostableAgentCheckIn, so those types' UnmarshalJSON
// validators run at the seam's binder and an `{}` is refused with the RUNTIME's
// own reason before the relay is reached. Reusing the runtime's types is what
// makes one validator serve both deployments; the price is that a fail-closed
// probe must send something the validator lets through, or it measures the
// binder instead of the seam.
func TestCloudIntegrationsFailClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	postableAccount := `{"config":{"aws":{"regions":["us-east-1"]}},"credentials":` +
		`{"o11yApiUrl":"https://api.hanzo.ai/v1/o11y","o11yApiKey":"pat_maxpower_7",` +
		`"ingestionUrl":"https://ingest.hanzo.ai","ingestionKey":"ik_9f3c"}}`
	updatableAccount := `{"config":{"aws":{"regions":["us-east-1"]}}}`
	updatableService := `{"config":{"aws":{"logs":{"enabled":true},"metrics":{"enabled":true}}}}`

	for _, probe := range []struct{ method, target, body string }{
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/credentials", ""},
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts", ""},
		{http.MethodPost, "/v1/o11y/cloud_integrations/aws/accounts", postableAccount},
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts/acct-7", ""},
		{http.MethodPut, "/v1/o11y/cloud_integrations/aws/accounts/acct-7", updatableAccount},
		{http.MethodDelete, "/v1/o11y/cloud_integrations/aws/accounts/acct-7", ""},
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/services", ""},
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services", ""},
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/services/ec2", ""},
		{http.MethodPut, "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services/ec2", updatableService},
		{http.MethodGet, "/v1/o11y/cloud_integrations/aws/accounts/acct-7/services/ec2", ""},
		{http.MethodPost, "/v1/o11y/cloud_integrations/aws/accounts/check_in", newAgentCheckIn},
		{http.MethodPost, "/v1/o11y/cloud-integrations/aws/agent-check-in", oldAgentCheckIn},
	} {
		var body io.Reader
		if probe.body != "" {
			body = strings.NewReader(probe.body)
		}
		if status, got := call(t, app, member(probe.method, probe.target, body)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", probe.method, probe.target, status, got)
		}
	}
}
