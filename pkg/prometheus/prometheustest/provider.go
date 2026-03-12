package prometheustest

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/prometheus/clickhouseprometheus"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
)

var _ prometheus.Prometheus = (*Provider)(nil)

type Provider struct {
	queryable storage.SampleAndChunkQueryable
	engine    *prometheus.Engine
}

var stCallback = func() (int64, error) {
	return int64(model.Latest), nil
}

func New(ctx context.Context, providerSettings factory.ProviderSettings, config prometheus.Config, telemetryStore telemetrystore.TelemetryStore) *Provider {

	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/prometheus/prometheustest")

	engine := prometheus.NewEngine(settings.Logger(), config)

	readClient := clickhouseprometheus.NewReadClient(settings, telemetryStore)

	queryable := remote.NewSampleAndChunkQueryableClient(readClient, labels.EmptyLabels(), []*labels.Matcher{}, false, stCallback)

	return &Provider{
		engine:    engine,
		queryable: queryable,
	}
}

func (provider *Provider) Engine() *prometheus.Engine {
	return provider.engine
}

func (provider *Provider) Storage() storage.Queryable {
	return provider.queryable
}

func (provider *Provider) Close() error {
	if provider.engine != nil {
		provider.engine.Close()
	}
	return nil
}
