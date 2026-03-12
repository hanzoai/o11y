package factorytest

import (
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
)

func NewSettings() factory.ProviderSettings {
	return instrumentationtest.New().ToProviderSettings()
}
