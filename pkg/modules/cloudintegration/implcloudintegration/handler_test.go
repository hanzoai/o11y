package implcloudintegration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	citypes "github.com/hanzoai/o11y/pkg/types/cloudintegrationtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The per-account service route carries THREE path segments, and each one names
// a different thing the module is asked for: which cloud, which integration,
// which service. What has to hold is that the values reaching the module are the
// segments the ROUTER matched — so the request is driven through a real router
// registered at the real template rather than by injecting vars. A handler that
// read a name the route does not declare would still pass against an injector
// and would answer 400 in production.
const accountServiceRoute = "/v1/o11y/cloud_integrations/{cloud_provider}/accounts/{id}/services/{service_id}"

// recordingModule answers the two calls GetAccountService makes and records what
// it was asked for. Every other method is left to the nil embedded interface, so
// a call this test does not expect panics instead of passing quietly.
type recordingModule struct {
	cloudintegration.Module
	provider  citypes.CloudProviderType
	accountID valuer.UUID
	serviceID citypes.ServiceID
}

func (m *recordingModule) GetConnectedAccount(_ context.Context, _, accountID valuer.UUID, provider citypes.CloudProviderType) (*citypes.Account, error) {
	m.accountID, m.provider = accountID, provider
	return &citypes.Account{}, nil
}

func (m *recordingModule) GetService(_ context.Context, _ valuer.UUID, serviceID citypes.ServiceID, _ citypes.CloudProviderType, _ valuer.UUID) (*citypes.Service, error) {
	m.serviceID = serviceID
	return &citypes.Service{}, nil
}

func serveAccountService(t *testing.T, module cloudintegration.Module, target string) *httptest.ResponseRecorder {
	t.Helper()

	router := mux.NewRouter()
	router.HandleFunc(accountServiceRoute, NewHandler(module).GetAccountService).Methods(http.MethodGet)

	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	req = req.WithContext(authtypes.NewContextWithClaims(req.Context(), authtypes.Claims{
		OrgID:     valuer.GenerateUUID().String(),
		UserID:    valuer.GenerateUUID().String(),
		Email:     "u@acme",
		Principal: authtypes.PrincipalUser,
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGetAccountServiceActsOnTheSegmentsTheRouterMatched(t *testing.T) {
	accountID := valuer.GenerateUUID()
	module := new(recordingModule)

	rec := serveAccountService(t, module, "/v1/o11y/cloud_integrations/aws/accounts/"+accountID.String()+"/services/ec2")

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, citypes.CloudProviderTypeAWS, module.provider, "the {cloud_provider} segment")
	assert.Equal(t, accountID, module.accountID, "the {id} segment")
	assert.Equal(t, citypes.AWSServiceEC2, module.serviceID, "the {service_id} segment")
}

// A provider the platform does not support is refused before anything is looked
// up — an unread or unrecognised segment must never reach the module as a query
// for whatever the zero value matches.
func TestGetAccountServiceRefusesAnUnknownProviderSegment(t *testing.T) {
	module := new(recordingModule)

	rec := serveAccountService(t, module, "/v1/o11y/cloud_integrations/notacloud/accounts/"+valuer.GenerateUUID().String()+"/services/ec2")

	require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, valuer.UUID{}, module.accountID, "the module must not be reached")
}
