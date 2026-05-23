package alertmanagerserver

// DispatcherMetrics is the alertmanager dispatcher's instrumentation.
//
// NOTE: This file is the prometheus/alertmanager-library boundary —
// dispatch.NewDispatcher, silence.New, notify.NewIntegration etc. all
// require metric.Registerer from prometheus/client_golang. We
// haven't replaced prometheus/alertmanager yet, so this single file
// still pulls prometheus/client_golang.
//
// Everything else in o11y (instrumentation/factory/test) uses luxfi/metric.
// The alertmanager rip is its own workstream — see project notes.

import "github.com/luxfi/metric"

type DispatcherMetrics struct {
	aggrGroups            metric.Gauge
	processingDuration    metric.Summary
	aggrGroupLimitReached metric.Counter
}

// NewDispatcherMetrics returns a new registered DispatchMetrics.
func NewDispatcherMetrics(registerLimitMetrics bool, r metric.Registerer) *DispatcherMetrics {
	m := DispatcherMetrics{
		aggrGroups: metric.NewGauge(
			metric.GaugeOpts{
				Name: "o11y_alertmanager_dispatcher_aggregation_groups",
				Help: "Number of active aggregation groups",
			},
		),
		processingDuration: metric.NewSummary(
			metric.SummaryOpts{
				Name: "o11y_alertmanager_dispatcher_alert_processing_duration_seconds",
				Help: "Summary of latencies for the processing of alerts.",
			},
		),
		aggrGroupLimitReached: metric.NewCounter(
			metric.CounterOpts{
				Name: "o11y_alertmanager_dispatcher_aggregation_group_limit_reached_total",
				Help: "Number of times when dispatcher failed to create new aggregation group due to limit.",
			},
		),
	}

	if r != nil {
		r.MustRegister(m.aggrGroups, m.processingDuration)
		if registerLimitMetrics {
			r.MustRegister(m.aggrGroupLimitReached)
		}
	}

	return &m
}
