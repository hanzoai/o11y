// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Test factory for rules.Manager. Uses the default-build prometheustest
// (noop PromQL provider) so the closure stays grpc-free.

package rules

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hanzoai/o11y/pkg/alertmanager"
	alertmanagermock "github.com/hanzoai/o11y/pkg/alertmanager/alertmanagertest"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/cache/cachetest"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/prometheus/prometheustest"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/querier/o11yquerier"
	datastoreReader "github.com/hanzoai/o11y/pkg/query-service/app/datastoreReader"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstoretest"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type queryMatcherAny struct{}

func (m *queryMatcherAny) Match(x string, y string) error {
	return nil
}

// TestManagerOptions provides options for customizing the test manager creation.
type TestManagerOptions struct {
	AlertmanagerHook   func(alertmanager.Alertmanager)
	SqlStoreHook       func(sqlstore.SQLStore)
	TelemetryStoreHook func(telemetrystore.TelemetryStore)
	ManagerOptionsHook func(*ManagerOptions)
}

// NewTestManager creates a Manager instance for testing purposes.
// It sets up all the necessary mocks and dependencies required for testing.
// Options can be provided to customize the manager behavior. If nil, default options are used.
func NewTestManager(t *testing.T, testOpts *TestManagerOptions) *Manager {
	fAlert := alertmanagermock.NewMockAlertmanager(t)

	if testOpts != nil && testOpts.AlertmanagerHook != nil {
		testOpts.AlertmanagerHook(fAlert)
	}

	cacheObj, err := cachetest.New(cache.Config{
		Provider: "memory",
		Memory: cache.Memory{
			NumCounters: 1000,
			MaxCost:     1 << 20,
		},
	})
	require.NoError(t, err)

	sqlStore := sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherRegexp)

	if testOpts != nil && testOpts.SqlStoreHook != nil {
		testOpts.SqlStoreHook(sqlStore)
	}

	telemetryStore := telemetrystoretest.New(telemetrystore.Config{}, &queryMatcherAny{})

	if testOpts != nil && testOpts.TelemetryStoreHook != nil {
		testOpts.TelemetryStoreHook(telemetryStore)
	}

	readerCache, err := cachetest.New(cache.Config{
		Provider: "memory",
		Memory: cache.Memory{
			NumCounters: 10 * 1000,
			MaxCost:     1 << 26,
		},
	})
	require.NoError(t, err)

	options := datastoreReader.NewOptions("", "", "archiveNamespace")
	providerSettings := instrumentationtest.New().ToProviderSettings()
	prom := prometheustest.New(context.Background(), providerSettings, prometheus.Config{}, telemetryStore)
	reader := datastoreReader.NewReader(
		nil,
		telemetryStore,
		prom,
		"",
		time.Duration(time.Second),
		nil,
		readerCache,
		options,
	)

	flgr, err := flagger.New(context.Background(), instrumentationtest.New().ToProviderSettings(), flagger.Config{}, flagger.MustNewRegistry())
	if err != nil {
		t.Fatalf("failed to create flagger: %v", err)
	}

	providerFactory := o11yquerier.NewFactory(telemetryStore, prom, readerCache, flgr)
	mockQuerier, err := providerFactory.New(context.Background(), providerSettings, querier.Config{})
	require.NoError(t, err)

	mgrOpts := &ManagerOptions{
		Logger:         zap.NewNop(),
		SLogger:        instrumentationtest.New().Logger(),
		Cache:          cacheObj,
		Alertmanager:   fAlert,
		Querier:        mockQuerier,
		TelemetryStore: telemetryStore,
		Reader:         reader,
		SqlStore:       sqlStore,
	}

	if testOpts != nil && testOpts.ManagerOptionsHook != nil {
		testOpts.ManagerOptionsHook(mgrOpts)
	}

	mgr, err := NewManager(mgrOpts)
	require.NoError(t, err)

	return mgr
}
