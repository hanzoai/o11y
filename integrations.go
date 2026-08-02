package o11y

// The INTEGRATIONS face — the integration catalog and its install lifecycle,
// the cloud-integrations account/service surface, the gateway ingestion keys
// and their limits, and the Zeus deployment profile/host writes — as TYPED
// ops.
//
// These twenty-nine routes reached traffic only through the /v1/o11y
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. A workspace could
// install an integration, connect an AWS account, mint an ingestion key or
// report a deployment profile only from a hand-written HTTP call. Typing the
// face is what puts the operations in the document and therefore in every
// projection built from it.
//
// THE WIRE DOES NOT MOVE, the same way telemetry.go's and infra.go's ops do
// not move it: no op here re-implements a handler. Each hands the call to the
// SAME runtime handler the wildcard delegates to (see integrationsRelay), so
// identity resolution, the org gate, the ROLE CHECK the mux registration
// declared and the success envelope are all still the runtime's, executed in
// the order they always were. Every route carried its gate on mux and still
// does, one layer in:
//
//   - cloud-integrations account/service ops (cloudintegration.go): AdminAccess
//   - the two agent check-ins (cloudintegration.go): ViewAccess (agent = viewer)
//   - gateway ingestion keys and limits (gateway.go): EditAccess
//   - Zeus PutProfile / PutHost (zeus.go): AdminAccess; GetHosts: ViewAccess
//   - integration catalog + install lifecycle (routes_integrations.go): ViewAccess
//
// The gate is not re-stated here because it is not enforced here; each op's
// comment names the gate its route has always had, and the runtime keeps
// enforcing it — its IdentN middleware rebuilds the caller's claims from the
// forwarded identity headers, and AdminAccess/EditAccess/ViewAccess resolve the
// role from those claims exactly as before.
//
// The request and response payloads are the RUNTIME'S OWN types
// (pkg/types/cloudintegrationtypes, pkg/types/gatewaytypes, pkg/types/zeustypes,
// pkg/query-service/app/integrations), not mirrors of them: the field list that
// decodes a request here is the field list the handler decodes one layer in, so
// the two cannot drift, and a field the runtime grows flows through without this
// file changing. Only the {status, data} envelopes, the id/parameter carriers a
// path spells, and the GET query shapes are NAMED here, because the runtime
// spells those on the wire rather than in a type.

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/query-service/app/integrations"
	citypes "github.com/hanzoai/o11y/pkg/types/cloudintegrationtypes"
	"github.com/hanzoai/o11y/pkg/types/gatewaytypes"
	"github.com/hanzoai/o11y/pkg/types/zeustypes"
	"github.com/zap-proto/zip"
)

// mountIntegrations registers the integrations face's typed ops on the native
// router. Static segments sit beside their parameterised siblings safely — the
// router matches the most specific pattern, so /integrations/install can never
// be swallowed by /integrations/:integrationId, the same disambiguation the
// runtime's own mux tree makes.
func mountIntegrations(app *zip.App) {
	g := app.Group(o11yRoot)

	// integration catalog and install lifecycle (runtime gate: ViewAccess)
	zip.Get(g, "/integrations", listIntegrations, zip.WithOperationID("ListIntegrations"))
	zip.Get(g, "/integrations/:integrationId", getIntegration, zip.WithOperationID("GetIntegration"))
	zip.Get(g, "/integrations/:integrationId/connection_status", getIntegrationConnectionStatus, zip.WithOperationID("GetIntegrationConnectionStatus"))
	zip.Post(g, "/integrations/install", installIntegration, zip.WithOperationID("InstallIntegration"))
	zip.Post(g, "/integrations/uninstall", uninstallIntegration, zip.WithOperationID("UninstallIntegration"))

	// cloud integrations — accounts and services (runtime gate: AdminAccess)
	zip.Get(g, "/cloud_integrations/:cloud_provider/credentials", getConnectionCredentials, zip.WithOperationID("GetConnectionCredentials"))
	zip.Post(g, "/cloud_integrations/:cloud_provider/accounts", createAccount, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateAccount"))
	zip.Get(g, "/cloud_integrations/:cloud_provider/accounts", listAccounts, zip.WithOperationID("ListAccounts"))
	zip.Get(g, "/cloud_integrations/:cloud_provider/accounts/:id", getAccount, zip.WithOperationID("GetAccount"))
	zip.Put(g, "/cloud_integrations/:cloud_provider/accounts/:id", updateAccount, zip.WithOperationID("UpdateAccount"))
	zip.Delete(g, "/cloud_integrations/:cloud_provider/accounts/:id", disconnectAccount, zip.WithOperationID("DisconnectAccount"))
	zip.Get(g, "/cloud_integrations/:cloud_provider/services", listServicesMetadata, zip.WithOperationID("ListServicesMetadata"))
	zip.Get(g, "/cloud_integrations/:cloud_provider/accounts/:id/services", listAccountServicesMetadata, zip.WithOperationID("ListAccountServicesMetadata"))
	zip.Get(g, "/cloud_integrations/:cloud_provider/services/:service_id", getService, zip.WithOperationID("GetService"))
	zip.Put(g, "/cloud_integrations/:cloud_provider/accounts/:id/services/:service_id", updateService, zip.WithOperationID("UpdateService"))
	zip.Get(g, "/cloud_integrations/:cloud_provider/accounts/:id/services/:service_id", getAccountService, zip.WithOperationID("GetAccountService"))
	// The deprecated agent check-in keeps its hyphenated path for the agents
	// already deployed against it; the check_in path below is its replacement.
	zip.Post(g, "/cloud-integrations/:cloud_provider/agent-check-in", agentCheckInDeprecated, zip.WithOperationID("AgentCheckInDeprecated"))
	zip.Post(g, "/cloud_integrations/:cloud_provider/accounts/check_in", agentCheckIn, zip.WithOperationID("AgentCheckIn"))

	// gateway ingestion keys and their limits (runtime gate: EditAccess)
	zip.Get(g, "/gateway/ingestion_keys", getIngestionKeys, zip.WithOperationID("GetIngestionKeys"))
	zip.Get(g, "/gateway/ingestion_keys/search", searchIngestionKeys, zip.WithOperationID("SearchIngestionKeys"))
	zip.Post(g, "/gateway/ingestion_keys", createIngestionKey, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateIngestionKey"))
	zip.Patch(g, "/gateway/ingestion_keys/:keyId", updateIngestionKey, zip.WithOperationID("UpdateIngestionKey"))
	zip.Delete(g, "/gateway/ingestion_keys/:keyId", deleteIngestionKey, zip.WithOperationID("DeleteIngestionKey"))
	zip.Post(g, "/gateway/ingestion_keys/:keyId/limits", createIngestionKeyLimit, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateIngestionKeyLimit"))
	zip.Patch(g, "/gateway/ingestion_keys/limits/:limitId", updateIngestionKeyLimit, zip.WithOperationID("UpdateIngestionKeyLimit"))
	zip.Delete(g, "/gateway/ingestion_keys/limits/:limitId", deleteIngestionKeyLimit, zip.WithOperationID("DeleteIngestionKeyLimit"))

	// Zeus — deployment profile and host (runtime gate: AdminAccess writes, ViewAccess read)
	zip.Put(g, "/zeus/profiles", putProfile, zip.WithOperationID("PutProfile"))
	zip.Get(g, "/zeus/hosts", getHosts, zip.WithOperationID("GetHosts"))
	zip.Put(g, "/zeus/hosts", putHost, zip.WithOperationID("PutHost"))
}

// ── integration catalog and install lifecycle ────────────────────────────────

// listIntegrations lists the available integrations and whether each is
// installed in the caller's org, optionally narrowed to installed or
// not-installed. Viewer gate.
func listIntegrations(ctx context.Context, in *O11yListIntegrationsIn) (*O11yIntegrationsListOut, error) {
	out := new(O11yIntegrationsListOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/integrations", query("is_installed", in.IsInstalled), nil, out)
}

// getIntegration returns one integration's full detail — its overview,
// configuration steps, collected data and assets — together with its
// installation record when the org has installed it. Viewer gate.
func getIntegration(ctx context.Context, in *O11yIntegrationRef) (*O11yIntegrationOut, error) {
	out := new(O11yIntegrationOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/integrations/"+in.IntegrationID, nil, nil, out)
}

// getIntegrationConnectionStatus reports whether the integration's logs and
// metrics have been received over the lookback window, so the console can show
// a live connection state. An integration that is not installed answers with an
// empty status rather than an error. Viewer gate.
func getIntegrationConnectionStatus(ctx context.Context, in *O11yConnectionStatusIn) (*O11yConnectionStatusOut, error) {
	out := new(O11yConnectionStatusOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/integrations/"+in.IntegrationID+"/connection_status", query("lookback_seconds", in.LookbackSeconds), nil, out)
}

// installIntegration installs an integration into the caller's org from its id
// and configuration, answering with the installed catalog item. Viewer gate.
func installIntegration(ctx context.Context, in *integrations.InstallIntegrationRequest) (*O11yInstallOut, error) {
	out := new(O11yInstallOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/integrations/install", nil, in, out)
}

// uninstallIntegration removes an integration from the caller's org by id.
// Viewer gate.
func uninstallIntegration(ctx context.Context, in *integrations.UninstallIntegrationRequest) (*O11yIntegrationAck, error) {
	out := new(O11yIntegrationAck)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/integrations/uninstall", nil, in, out)
}

// ── cloud integrations — accounts ─────────────────────────────────────────────

// getConnectionCredentials returns the credentials the connecting agent needs
// to establish the cloud integration, for the given cloud provider. Admin gate.
func getConnectionCredentials(ctx context.Context, in *O11yCloudProviderRef) (*O11yCredentialsOut, error) {
	out := new(O11yCredentialsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/credentials", nil, nil, out)
}

// createAccount connects a new cloud-integration account for the given
// provider from its posted config and credentials, answering with the account
// and the artifact the agent deploys to complete the connection. Admin gate.
func createAccount(ctx context.Context, in *O11yCreateAccountIn) (*O11yCreateAccountOut, error) {
	out := new(O11yCreateAccountOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts", nil, in, out)
}

// listAccounts lists the cloud-integration accounts connected for the given
// provider. Admin gate.
func listAccounts(ctx context.Context, in *O11yCloudProviderRef) (*O11yAccountsOut, error) {
	out := new(O11yAccountsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts", nil, nil, out)
}

// getAccount returns one connected account for the given provider, by id. Admin
// gate.
func getAccount(ctx context.Context, in *O11yAccountRef) (*O11yAccountOut, error) {
	out := new(O11yAccountOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/"+in.ID, nil, nil, out)
}

// updateAccount changes a connected account's configuration for the given
// provider, by id. Admin gate.
func updateAccount(ctx context.Context, in *O11yUpdateAccountIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/"+in.ID, nil, in, nil)
}

// disconnectAccount tears down a connected account for the given provider, by
// id. Admin gate.
func disconnectAccount(ctx context.Context, in *O11yAccountRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/"+in.ID, nil, nil, nil)
}

// ── cloud integrations — services ─────────────────────────────────────────────

// listServicesMetadata lists the services the given provider can collect from,
// optionally scoped to one cloud integration. Admin gate.
func listServicesMetadata(ctx context.Context, in *O11yListServicesMetadataIn) (*O11yServicesMetadataOut, error) {
	out := new(O11yServicesMetadataOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/services", query("cloud_integration_id", in.CloudIntegrationID), nil, out)
}

// listAccountServicesMetadata lists the services metadata for one connected
// account of the given provider, by account id. Admin gate.
func listAccountServicesMetadata(ctx context.Context, in *O11yAccountRef) (*O11yServicesMetadataOut, error) {
	out := new(O11yServicesMetadataOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/"+in.ID+"/services", nil, nil, out)
}

// getService returns one service the given provider can collect from, by
// service id, optionally scoped to one cloud integration. Admin gate.
func getService(ctx context.Context, in *O11yGetServiceIn) (*O11yServiceOut, error) {
	out := new(O11yServiceOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/services/"+in.ServiceID, query("cloud_integration_id", in.CloudIntegrationID), nil, out)
}

// updateService changes a service's configuration for one connected account of
// the given provider, by account id and service id. Admin gate.
func updateService(ctx context.Context, in *O11yUpdateServiceIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/"+in.ID+"/services/"+in.ServiceID, nil, in, nil)
}

// getAccountService returns one service and its configuration for a connected
// account of the given provider, by account id and service id. Admin gate.
func getAccountService(ctx context.Context, in *O11yAccountServiceRef) (*O11yServiceOut, error) {
	out := new(O11yServiceOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/"+in.ID+"/services/"+in.ServiceID, nil, nil, out)
}

// agentCheckInDeprecated is the deployed agent's check-in on its original
// hyphenated path, kept for backward compatibility with agents already
// running. Viewer gate — the agent's role is viewer.
func agentCheckInDeprecated(ctx context.Context, in *O11yAgentCheckInIn) (*O11yAgentCheckInOut, error) {
	out := new(O11yAgentCheckInOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/cloud-integrations/"+in.CloudProvider+"/agent-check-in", nil, in, out)
}

// agentCheckIn is the deployed agent's check-in — the path consistent with the
// account surface, reporting the agent's account and telemetry state so the
// connection can be tracked. Viewer gate — the agent's role is viewer.
func agentCheckIn(ctx context.Context, in *O11yAgentCheckInIn) (*O11yAgentCheckInOut, error) {
	out := new(O11yAgentCheckInOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/cloud_integrations/"+in.CloudProvider+"/accounts/check_in", nil, in, out)
}

// ── gateway ingestion keys ────────────────────────────────────────────────────

// getIngestionKeys lists the workspace's ingestion keys, paginated. Editor
// gate.
func getIngestionKeys(ctx context.Context, in *O11yIngestionKeysIn) (*O11yIngestionKeysOut, error) {
	out := new(O11yIngestionKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/gateway/ingestion_keys", query("page", in.Page, "per_page", in.PerPage), nil, out)
}

// searchIngestionKeys lists the workspace's ingestion keys whose name matches
// the search, paginated. Editor gate.
func searchIngestionKeys(ctx context.Context, in *O11ySearchIngestionKeysIn) (*O11yIngestionKeysOut, error) {
	out := new(O11yIngestionKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/gateway/ingestion_keys/search", query("name", in.Name, "page", in.Page, "per_page", in.PerPage), nil, out)
}

// createIngestionKey mints an ingestion key for the workspace, answering with
// the created key. Editor gate.
func createIngestionKey(ctx context.Context, in *gatewaytypes.PostableIngestionKey) (*O11yCreatedIngestionKeyOut, error) {
	out := new(O11yCreatedIngestionKeyOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/gateway/ingestion_keys", nil, in, out)
}

// updateIngestionKey changes an ingestion key, by id. Editor gate.
func updateIngestionKey(ctx context.Context, in *O11yUpdateIngestionKeyIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPatch, o11yRoot+"/gateway/ingestion_keys/"+in.KeyID, nil, in, nil)
}

// deleteIngestionKey removes an ingestion key, by id. Editor gate.
func deleteIngestionKey(ctx context.Context, in *O11yIngestionKeyRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/gateway/ingestion_keys/"+in.KeyID, nil, nil, nil)
}

// createIngestionKeyLimit sets a signal limit on an ingestion key, by key id,
// answering with the created limit. Editor gate.
func createIngestionKeyLimit(ctx context.Context, in *O11yCreateLimitIn) (*O11yCreatedLimitOut, error) {
	out := new(O11yCreatedLimitOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/gateway/ingestion_keys/"+in.KeyID+"/limits", nil, in, out)
}

// updateIngestionKeyLimit changes an ingestion key limit, by limit id. Editor
// gate.
func updateIngestionKeyLimit(ctx context.Context, in *O11yUpdateLimitIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPatch, o11yRoot+"/gateway/ingestion_keys/limits/"+in.LimitID, nil, in, nil)
}

// deleteIngestionKeyLimit removes an ingestion key limit, by limit id. Editor
// gate.
func deleteIngestionKeyLimit(ctx context.Context, in *O11yLimitRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/gateway/ingestion_keys/limits/"+in.LimitID, nil, nil, nil)
}

// ── Zeus — deployment profile and host ────────────────────────────────────────

// putProfile records the deployment's profile in Zeus — how the team uses
// observability today and what they plan — overwriting any prior one. Admin
// gate.
func putProfile(ctx context.Context, in *zeustypes.PostableProfile) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/zeus/profiles", nil, in, nil)
}

// getHosts returns the deployment's host info from Zeus. Viewer gate.
func getHosts(ctx context.Context, _ *struct{}) (*O11yGettableHostOut, error) {
	out := new(O11yGettableHostOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/zeus/hosts", nil, nil, out)
}

// putHost records the deployment's host in Zeus, overwriting any prior one.
// Admin gate.
func putHost(ctx context.Context, in *zeustypes.PostableHost) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/zeus/hosts", nil, in, nil)
}

// ── inputs the URL carries ────────────────────────────────────────────────────

// O11yListIntegrationsIn narrows the catalog listing.
type O11yListIntegrationsIn struct {
	// IsInstalled, when "true" or "false", keeps only integrations in that
	// installed state; empty lists them all.
	IsInstalled string `json:"is_installed"`
}

// O11yIntegrationRef names one integration by id.
type O11yIntegrationRef struct {
	// IntegrationID is the integration's id.
	IntegrationID string `json:"-" url:"integrationId" validate:"required"`
}

// O11yConnectionStatusIn names one integration and the window to report over.
type O11yConnectionStatusIn struct {
	// IntegrationID is the integration's id.
	IntegrationID string `json:"-" url:"integrationId" validate:"required"`
	// LookbackSeconds is how far back to look for received telemetry, in
	// seconds.
	LookbackSeconds int `json:"lookback_seconds"`
}

// O11yCloudProviderRef names one cloud provider.
type O11yCloudProviderRef struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
}

// O11yAccountRef names one connected account under a cloud provider.
type O11yAccountRef struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	// ID is the connected account's id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yAccountServiceRef names one service of one connected account.
type O11yAccountServiceRef struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	// ID is the connected account's id.
	ID string `json:"-" url:"id" validate:"required"`
	// ServiceID is the service's id.
	ServiceID string `json:"-" url:"service_id" validate:"required"`
}

// O11yListServicesMetadataIn scopes the provider's services listing.
type O11yListServicesMetadataIn struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	// CloudIntegrationID, when set, scopes the listing to one cloud integration.
	CloudIntegrationID string `json:"cloud_integration_id"`
}

// O11yGetServiceIn names one service under a provider, optionally scoped.
type O11yGetServiceIn struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	// ServiceID is the service's id.
	ServiceID string `json:"-" url:"service_id" validate:"required"`
	// CloudIntegrationID, when set, scopes the service to one cloud integration.
	CloudIntegrationID string `json:"cloud_integration_id"`
}

// O11yCreateAccountIn carries the provider in the path and the posted account
// in the body.
type O11yCreateAccountIn struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	citypes.PostableAccount
}

// O11yUpdateAccountIn carries the provider and account id in the path and the
// account's new config in the body.
type O11yUpdateAccountIn struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	// ID is the connected account's id.
	ID string `json:"-" url:"id" validate:"required"`
	citypes.UpdatableAccount
}

// O11yUpdateServiceIn carries the provider, account id and service id in the
// path and the service's new config in the body.
type O11yUpdateServiceIn struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	// ID is the connected account's id.
	ID string `json:"-" url:"id" validate:"required"`
	// ServiceID is the service's id.
	ServiceID string `json:"-" url:"service_id" validate:"required"`
	citypes.UpdatableService
}

// O11yAgentCheckInIn carries the provider in the path and the agent's check-in
// in the body.
type O11yAgentCheckInIn struct {
	// CloudProvider is the cloud provider — aws, azure or gcp.
	CloudProvider string `json:"-" url:"cloud_provider" validate:"required"`
	citypes.PostableAgentCheckIn
}

// O11yIngestionKeysIn paginates the ingestion-key listing.
type O11yIngestionKeysIn struct {
	// Page is the 1-based page number.
	Page int `json:"page"`
	// PerPage is the page size.
	PerPage int `json:"per_page"`
}

// O11ySearchIngestionKeysIn names and paginates the ingestion-key search.
type O11ySearchIngestionKeysIn struct {
	// Name is the substring to match ingestion-key names against.
	Name string `json:"name"`
	// Page is the 1-based page number.
	Page int `json:"page"`
	// PerPage is the page size.
	PerPage int `json:"per_page"`
}

// O11yIngestionKeyRef names one ingestion key by id.
type O11yIngestionKeyRef struct {
	// KeyID is the ingestion key's id.
	KeyID string `json:"-" url:"keyId" validate:"required"`
}

// O11yUpdateIngestionKeyIn carries the key id in the path and the key's new
// fields in the body.
type O11yUpdateIngestionKeyIn struct {
	// KeyID is the ingestion key's id.
	KeyID string `json:"-" url:"keyId" validate:"required"`
	gatewaytypes.PostableIngestionKey
}

// O11yCreateLimitIn carries the key id in the path and the posted limit in the
// body.
type O11yCreateLimitIn struct {
	// KeyID is the ingestion key's id.
	KeyID string `json:"-" url:"keyId" validate:"required"`
	gatewaytypes.PostableIngestionKeyLimit
}

// O11yUpdateLimitIn carries the limit id in the path and the limit's new fields
// in the body.
type O11yUpdateLimitIn struct {
	// LimitID is the ingestion key limit's id.
	LimitID string `json:"-" url:"limitId" validate:"required"`
	gatewaytypes.UpdatableIngestionKeyLimit
}

// O11yLimitRef names one ingestion key limit by id.
type O11yLimitRef struct {
	// LimitID is the ingestion key limit's id.
	LimitID string `json:"-" url:"limitId" validate:"required"`
}

// ── the {status, data} envelopes the runtime returns ─────────────────────────

// O11yIntegrationsListOut is the catalog listing.
type O11yIntegrationsListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the available integrations and their installed state.
	Data integrations.IntegrationsListResponse `json:"data,omitempty"`
}

// O11yIntegrationOut is one integration's full detail.
type O11yIntegrationOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the integration and its installation record.
	Data integrations.Integration `json:"data,omitempty"`
}

// O11yConnectionStatusOut is an integration's live connection state.
type O11yConnectionStatusOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the logs and metrics connection status.
	Data integrations.IntegrationConnectionStatus `json:"data,omitempty"`
}

// O11yInstallOut is the installed catalog item.
type O11yInstallOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the installed integration.
	Data integrations.IntegrationsListItem `json:"data,omitempty"`
}

// O11yIntegrationAck acknowledges an uninstall.
type O11yIntegrationAck struct {
	// Status is "success".
	Status string `json:"status"`
}

// O11yCredentialsOut is the connecting agent's credentials.
type O11yCredentialsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the connection credentials.
	Data citypes.Credentials `json:"data,omitempty"`
}

// O11yCreateAccountOut is a newly connected account with its deploy artifact.
type O11yCreateAccountOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the account and the connection artifact.
	Data citypes.GettableAccountWithConnectionArtifact `json:"data,omitempty"`
}

// O11yAccountsOut is the provider's connected accounts.
type O11yAccountsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the connected accounts.
	Data citypes.GettableAccounts `json:"data,omitempty"`
}

// O11yAccountOut is one connected account.
type O11yAccountOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the account.
	Data citypes.Account `json:"data,omitempty"`
}

// O11yServicesMetadataOut is a provider's collectable services.
type O11yServicesMetadataOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the services metadata.
	Data citypes.GettableServicesMetadata `json:"data,omitempty"`
}

// O11yServiceOut is one collectable service and its configuration.
type O11yServiceOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the service.
	Data citypes.Service `json:"data,omitempty"`
}

// O11yAgentCheckInOut is the runtime's answer to an agent check-in.
type O11yAgentCheckInOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the check-in result.
	Data citypes.GettableAgentCheckIn `json:"data,omitempty"`
}

// O11yIngestionKeysOut is the workspace's ingestion keys.
type O11yIngestionKeysOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the ingestion keys.
	Data gatewaytypes.GettableIngestionKeys `json:"data,omitempty"`
}

// O11yCreatedIngestionKeyOut is a newly minted ingestion key.
type O11yCreatedIngestionKeyOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the created ingestion key.
	Data gatewaytypes.GettableCreatedIngestionKey `json:"data,omitempty"`
}

// O11yCreatedLimitOut is a newly set ingestion key limit.
type O11yCreatedLimitOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the created limit.
	Data gatewaytypes.GettableCreatedIngestionKeyLimit `json:"data,omitempty"`
}

// O11yGettableHostOut is the deployment's host info from Zeus.
type O11yGettableHostOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the host info.
	Data zeustypes.GettableHost `json:"data,omitempty"`
}
