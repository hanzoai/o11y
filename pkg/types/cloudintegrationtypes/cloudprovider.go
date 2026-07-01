package cloudintegrationtypes

import (
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type CloudProviderType struct{ valuer.String }

var (
	// cloud providers.
	CloudProviderTypeAWS   = CloudProviderType{valuer.NewString("aws")}
	CloudProviderTypeAzure = CloudProviderType{valuer.NewString("azure")}

	ErrCodeCloudProviderInvalidInput = errors.MustNewCode("cloud_integration_invalid_cloud_provider")
)

func NewCloudProvider(provider string) (CloudProviderType, error) {
	switch provider {
	case CloudProviderTypeAWS.StringValue():
		return CloudProviderTypeAWS, nil
	case CloudProviderTypeAzure.StringValue():
		return CloudProviderTypeAzure, nil
	default:
		return CloudProviderType{}, errors.NewInvalidInputf(ErrCodeCloudProviderInvalidInput, "invalid cloud provider: %s", provider)
	}
}
