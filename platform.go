package o11y

// The PLATFORM face of the o11y product — instant queries, dashboard
// variables, product events, usage, the dependency graph, version/health,
// disks, first-user registration, retention and apdex settings, licenses,
// global config, feature flags, org stats, filter suggestions, k8s onboarding,
// metric metadata, span percentiles and query-filter analysis — as TYPED ops.
//
// These twenty-three routes reached traffic only through the /v1/o11y/*
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. Typing them is
// what puts the operations in the document and therefore in every projection
// built from it.
//
// THE WIRE DOES NOT MOVE, the same way logs.go's nine reads and telemetry.go's
// five sentry reads do not move it: each op hands the call to the SAME runtime
// handler the wildcard delegates to, through the ONE shared seam relay (relay.go)
// — so identity resolution, the org gate, the role check (OpenAccess, ViewAccess
// and AdminAccess exactly as the mux tree declares them), the audit record and
// the success envelope are all still the runtime's, executed in the order they
// always were. What is new here is the TYPE — the In a caller may send and the
// Out they get back — and the prose that goes with it. This face is /v1/o11y,
// the same root as logs, and every face shares the one seam in relay.go
// rather than minting its own: one seam, not one per face.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these paths dispatch here
// and every other path under the prefix still reaches the runtime untouched.
//
// The three service probes of this prefix — healthz, readyz, livez — are NOT
// here: they already dispatch natively through health.go's probe group, whose
// fall-through-when-unset behavior is load-bearing (health_test.go pins it),
// and a second registration of the same literals would only shadow it.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/zap-proto/zip"
)

// mountPlatform registers the typed platform ops on the native router, in the
// order the mux tree declares them (routes_misc.go, routes_settings.go,
// routes_licenses.go, then the o11yapiserver singletons). It registers on the
// SAME o11yRoot group logs.go mounts on, ahead of the delegation wildcard.
func mountPlatform(app *zip.App) {
	g := app.Group(o11yRoot)

	// routes_misc.go
	zip.Get(g, "/query", promQuery)
	zip.Post(g, "/variables/query", dashboardVars)
	zip.Post(g, "/event", event)
	zip.Get(g, "/usage", usage)
	zip.Post(g, "/dependency_graph", dependencyGraph)
	zip.Get(g, "/version", version)
	zip.Get(g, "/health", health)
	zip.Get(g, "/disks", disks)
	zip.Post(g, "/span_percentile", spanPercentile)
	zip.Post(g, "/query_filter/analyze", analyzeQueryFilter)
	zip.Get(g, "/filter_suggestions", filterSuggestions)
	zip.Get(g, "/infra_onboarding/k8s/status", k8sOnboarding)
	zip.Get(g, "/metric/metric_metadata", legacyMetricMetadata)

	// routes_settings.go
	zip.Post(g, "/settings/ttl", setRetention)
	zip.Get(g, "/settings/ttl", retention)
	zip.Post(g, "/settings/apdex", setApdex)
	zip.Get(g, "/settings/apdex", apdex)

	// routes_licenses.go
	zip.Get(g, "/licenses", licenses)
	zip.Get(g, "/licenses/active", activateLicense)

	// o11yapiserver: global.go, flagger.go, statsreporter.go
	zip.Get(g, "/global/config", globalConfig)
	zip.Get(g, "/features", features)
	zip.Get(g, "/stats", orgStats)
}

// ── the operations ────────────────────────────────────────────────────────────

// promQuery evaluates one instant PromQL query against the org's metrics and
// returns the result at a single point in time.
//
// The result is polymorphic by PromQL's own contract — a matrix, vector,
// scalar or string, discriminated by resultType — so it is carried verbatim
// rather than forced into one of its shapes.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func promQuery(ctx context.Context, in *O11yPromQueryIn) (*O11yPromQueryOut, error) {
	out := new(O11yPromQueryOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/query", query(
		"query", in.Query,
		"time", in.Time,
		"stats", in.Stats,
		"timeout", in.Timeout,
	), nil, out)
}

// dashboardVars evaluates a dashboard variable query and returns the values the
// variable may take.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardVars(ctx context.Context, in *O11yDashboardVarsIn) (*O11yDashboardVarsOut, error) {
	out := new(O11yDashboardVarsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/variables/query", nil, in, out)
}

// event records one product-analytics event for the signed-in user — a track
// event with a name and free-form attributes.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func event(ctx context.Context, in *O11yEventIn) (*O11yMessage, error) {
	out := new(O11yMessage)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/event", nil, in, out)
}

// usage returns ingestion usage counts bucketed over the requested window,
// optionally narrowed to one service.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func usage(ctx context.Context, in *O11yUsageIn) (*O11yUsageList, error) {
	out := new(O11yUsageList)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/usage", query(
		"start", in.Start,
		"end", in.End,
		"step", in.Step,
		"service", in.Service,
	), nil, out)
}

// dependencyGraph returns the service dependency graph over the requested
// window: every parent→child edge observed, with call and error rates and
// latency percentiles per edge.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dependencyGraph(ctx context.Context, in *O11yDependencyGraphIn) (*O11yDependencyList, error) {
	out := new(O11yDependencyList)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/dependency_graph", nil, in, out)
}

// version reports the running build: its version, whether an enterprise
// edition is present ("N" in this build), and whether first-user setup has
// completed.
//
// Open by design; the runtime's own gate is OpenAccess.
func version(ctx context.Context, _ *struct{}) (*O11yVersionOut, error) {
	out := new(O11yVersionOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/version", nil, nil, out)
}

// health reports service health. With live set, the datastore connection is
// checked too and an unhealthy store refuses with 503.
//
// Open by design; the runtime's own gate is OpenAccess.
func health(ctx context.Context, in *O11yHealthIn) (*O11yHealthOut, error) {
	out := new(O11yHealthOut)
	p := url.Values{}
	if in.Live {
		// The runtime tests presence, not value.
		p.Set("live", "true")
	}
	return out, relay(ctx, http.MethodGet, o11yRoot+"/health", p, nil, out)
}

// disks lists the storage disks the datastore reports, with their names and
// types.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func disks(ctx context.Context, _ *struct{}) (*O11yDiskList, error) {
	out := new(O11yDiskList)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/disks", nil, nil, out)
}

// spanPercentile places one span's duration among its peers: the p50/p90/p99
// durations of like spans, and the percentile the given duration lands at.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func spanPercentile(ctx context.Context, in *O11ySpanPercentileIn) (*O11ySpanPercentileOut, error) {
	out := new(O11ySpanPercentileOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/span_percentile", nil, in, out)
}

// analyzeQueryFilter analyzes a query and extracts the metric names it reads
// and the columns it groups by.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func analyzeQueryFilter(ctx context.Context, in *O11yAnalyzeIn) (*O11yAnalyzeOut, error) {
	out := new(O11yAnalyzeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/query_filter/analyze", nil, in, out)
}

// filterSuggestions suggests attribute keys and example filter queries for the
// query builder, seeded by what the org's own telemetry carries. Only the logs
// data source is supported today.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func filterSuggestions(ctx context.Context, in *O11yFilterSuggestionsIn) (*O11yFilterSuggestionsOut, error) {
	out := new(O11yFilterSuggestionsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/filter_suggestions", query(
		"dataSource", in.DataSource,
		"searchText", in.SearchText,
		"existingFilter", in.ExistingFilter,
		"attributesLimit", in.AttributesLimit,
		"examplesLimit", in.ExamplesLimit,
	), nil, out)
}

// k8sOnboarding reports how far Kubernetes infra onboarding has progressed:
// which metric families have arrived and, per pod, which required metadata
// labels are present.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func k8sOnboarding(ctx context.Context, _ *struct{}) (*O11yOnboardingOut, error) {
	out := new(O11yOnboardingOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/infra_onboarding/k8s/status", nil, nil, out)
}

// legacyMetricMetadata serves the OLDER /metric/metric_metadata route. It is
// NOT the same op as metrics.go's metricMetadata (/metrics/metadata): different
// path, different input (this one also scopes by service). Two slices named one
// Go function for two routes; the route is the identity, so the name follows it.
// Renamed rather than merged — collapsing them would silently drop the service
// scope this one accepts.
// It returns one metric's metadata — its type, unit, description,
// temporality, monotonicity and histogram buckets — optionally scoped to the
// metric as one service reports it.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func legacyMetricMetadata(ctx context.Context, in *O11yMetricMetadataIn) (*O11yMetricMetadataOut, error) {
	out := new(O11yMetricMetadataOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/metric/metric_metadata", query(
		"metricName", in.MetricName,
		"serviceName", in.ServiceName,
	), nil, out)
}

// setRetention sets the org's retention policy for one signal: the default TTL
// in days, ordered per-label retention rules, and optional cold-storage
// settings.
//
// Admin only, as the mux tree has always gated it (AdminAccess); the runtime's
// own gate enforces it.
func setRetention(ctx context.Context, in *O11yRetentionSetIn) (*O11yRetentionSetOut, error) {
	out := new(O11yRetentionSetOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/settings/ttl", nil, in, out)
}

// retention returns the org's current retention policy: default TTL, custom
// per-label rules, and cold-storage settings where configured.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func retention(ctx context.Context, _ *struct{}) (*O11yRetentionOut, error) {
	out := new(O11yRetentionOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/settings/ttl", nil, nil, out)
}

// setApdex sets one service's apdex threshold and the status codes excluded
// from its score.
//
// Admin only, as the mux tree has always gated it (AdminAccess); the runtime's
// own gate enforces it.
func setApdex(ctx context.Context, in *O11yApdexSetIn) (*O11yApdexSetOut, error) {
	out := new(O11yApdexSetOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/settings/apdex", nil, in, out)
}

// apdex returns apdex settings for the named services.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func apdex(ctx context.Context, in *O11yApdexIn) (*O11yApdexOut, error) {
	out := new(O11yApdexOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/settings/apdex", query(
		"services", in.Services,
	), nil, out)
}

// licenses lists the org's licenses. This build has no enterprise edition, so
// the list is intentionally empty.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func licenses(ctx context.Context, _ *struct{}) (*O11yLicensesOut, error) {
	out := new(O11yLicensesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/licenses", nil, nil, out)
}

// activateLicense activates the enterprise license. This build has no
// enterprise edition, so the licensing provider refuses it as unsupported.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func activateLicense(ctx context.Context, _ *struct{}) (*O11yLicenseActiveOut, error) {
	out := new(O11yLicenseActiveOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/licenses/active", nil, nil, out)
}

// globalConfig returns the deployment's global configuration: its public
// endpoints and which identity providers are enabled. Open by design — the
// sign-in page reads it before anyone is signed in.
//
// Open by design; the runtime's own gate is OpenAccess.
func globalConfig(ctx context.Context, _ *struct{}) (*O11yGlobalConfigOut, error) {
	out := new(O11yGlobalConfigOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/global/config", nil, nil, out)
}

// features returns the supported feature flags and their resolved values for
// the caller's org.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func features(ctx context.Context, _ *struct{}) (*O11yFeaturesOut, error) {
	out := new(O11yFeaturesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/features", nil, nil, out)
}

// orgStats returns the collected usage statistics for the caller's org, as the
// stats reporter aggregates them — a map whose keys are the reporter's own
// counter names.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func orgStats(ctx context.Context, _ *struct{}) (*O11yOrgStatsOut, error) {
	out := new(O11yOrgStatsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/stats", nil, nil, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────
//
// Every type here carries the O11y qualifier because these names enter a
// fleet-wide document, where one name may have exactly one shape. An operation
// that takes no input takes *struct{}, as logs.go's do.

// O11yPromQueryIn is one instant PromQL query.
type O11yPromQueryIn struct {
	// Query is the PromQL expression to evaluate. Required.
	Query string `json:"query" validate:"required"`
	// Time is the evaluation timestamp — epoch seconds or RFC3339. Empty
	// evaluates at now.
	Time string `json:"time,omitempty"`
	// Stats set to any non-empty value includes query statistics in the answer.
	Stats string `json:"stats,omitempty"`
	// Timeout caps evaluation time, as a duration in seconds.
	Timeout string `json:"timeout,omitempty"`
}

// O11yDashboardVarsIn is one dashboard variable query.
type O11yDashboardVarsIn struct {
	// Query is the variable query to evaluate. Required.
	Query string `json:"query" validate:"required"`
	// Variables are the current values of the other dashboard variables, for
	// queries that reference them.
	Variables map[string]any `json:"variables,omitempty"`
}

// O11yEventIn is one product-analytics event.
type O11yEventIn struct {
	// EventType is the kind of event — track, identify or group. Required.
	EventType string `json:"eventType" validate:"required"`
	// EventName names the event; required for track events.
	EventName string `json:"eventName,omitempty"`
	// Attributes are free-form event properties.
	Attributes map[string]any `json:"attributes,omitempty"`
	// RateLimited marks an event the reporting client rate-limited.
	RateLimited bool `json:"rateLimited,omitempty"`
}

// O11yUsageIn selects a usage window.
type O11yUsageIn struct {
	// Start is the window start, as epoch nanoseconds. Required.
	Start string `json:"start" validate:"required"`
	// End is the window end, as epoch nanoseconds. Required.
	End string `json:"end" validate:"required"`
	// Step is the bucket width in seconds. The runtime requires it.
	Step int `json:"step,omitempty"`
	// Service narrows usage to one service. Empty covers all.
	Service string `json:"service,omitempty"`
}

// O11yDependencyGraphIn selects the window the dependency graph is computed
// over.
type O11yDependencyGraphIn struct {
	// Start is the window start, as epoch nanoseconds. Required.
	Start string `json:"start" validate:"required"`
	// End is the window end, as epoch nanoseconds. Required.
	End string `json:"end" validate:"required"`
	// Tags narrow the graph to spans matching every condition.
	Tags []O11yTagFilter `json:"tags,omitempty"`
}

// O11yTagFilter is one span-tag condition.
type O11yTagFilter struct {
	// Key is the tag to test.
	Key string `json:"key"`
	// TagType says which kind of value the tag holds: string, number or bool.
	TagType string `json:"tagType"`
	// StringValues are the values matched when the tag holds strings.
	StringValues []string `json:"stringValues,omitempty"`
	// BoolValues are the values matched when the tag holds booleans.
	BoolValues []bool `json:"boolValues,omitempty"`
	// NumberValues are the values matched when the tag holds numbers.
	NumberValues []float64 `json:"numberValues,omitempty"`
	// Operator is the comparison to apply — in, not_in, equals, contains and
	// the other operators the trace filter grammar names.
	Operator string `json:"operator"`
}

// O11yHealthIn selects how deep the health check goes.
type O11yHealthIn struct {
	// Live also checks the datastore connection; an unreachable store refuses
	// with 503.
	Live bool `json:"live,omitempty"`
}

// O11ySpanPercentileIn names one span and the peer group to place it in.
type O11ySpanPercentileIn struct {
	// SpanDuration is the span's duration in nanoseconds.
	SpanDuration int64 `json:"spanDuration"`
	// Name is the span name whose peers are compared. Required.
	Name string `json:"name" validate:"required"`
	// ServiceName is the service the span belongs to. Required.
	ServiceName string `json:"serviceName" validate:"required"`
	// ResourceAttributes narrow the peer group to spans carrying them all.
	ResourceAttributes map[string]string `json:"resourceAttributes,omitempty"`
	// Start is the window start, as epoch nanoseconds.
	Start uint64 `json:"start"`
	// End is the window end, as epoch nanoseconds.
	End uint64 `json:"end"`
}

// O11yAnalyzeIn is one query to analyze.
type O11yAnalyzeIn struct {
	// Query is the query text. Required.
	Query string `json:"query" validate:"required"`
	// QueryType says which language the query is in — promql or the datastore's
	// SQL dialect. Required.
	QueryType string `json:"queryType" validate:"required"`
}

// O11yFilterSuggestionsIn seeds the suggestion engine.
type O11yFilterSuggestionsIn struct {
	// DataSource is the signal suggestions are drawn from; only logs is
	// supported today. Required.
	DataSource string `json:"dataSource" validate:"required"`
	// SearchText narrows attribute suggestions to keys containing it.
	SearchText string `json:"searchText,omitempty"`
	// ExistingFilter is the current filter set, JSON base64url-encoded, so
	// example queries build on it rather than repeat it.
	ExistingFilter string `json:"existingFilter,omitempty"`
	// AttributesLimit caps how many attribute keys come back.
	AttributesLimit int `json:"attributesLimit,omitempty"`
	// ExamplesLimit caps how many example queries come back.
	ExamplesLimit int `json:"examplesLimit,omitempty"`
}

// O11yMetricMetadataIn names the metric to read.
type O11yMetricMetadataIn struct {
	// MetricName is the metric to read.
	MetricName string `json:"metricName"`
	// ServiceName scopes the metadata to the metric as one service reports it.
	ServiceName string `json:"serviceName,omitempty"`
}

// O11yRetentionSetIn is one signal's retention policy.
type O11yRetentionSetIn struct {
	// Type is the signal the policy applies to — traces, metrics or logs.
	Type string `json:"type"`
	// DefaultTTLDays is the retention for data no rule matches, in days.
	DefaultTTLDays int `json:"defaultTTLDays"`
	// TTLConditions are ordered per-label rules; the first matching rule wins.
	TTLConditions []O11yRetentionRule `json:"ttlConditions,omitempty"`
	// ColdStorageVolume names the volume aged data moves to, when set.
	ColdStorageVolume string `json:"coldStorageVolume,omitempty"`
	// ColdStorageDurationDays is how old data must be before it moves, in days.
	ColdStorageDurationDays int64 `json:"coldStorageDurationDays,omitempty"`
}

// O11yApdexSetIn is one service's apdex settings.
type O11yApdexSetIn struct {
	// ServiceName is the service the threshold applies to.
	ServiceName string `json:"serviceName"`
	// Threshold is the satisfied-response time in seconds.
	Threshold float64 `json:"threshold"`
	// ExcludeStatusCodes are status codes excluded from the score, comma
	// separated.
	ExcludeStatusCodes string `json:"excludeStatusCodes"`
}

// O11yApdexIn names the services to read.
type O11yApdexIn struct {
	// Services are the service names, comma separated.
	Services string `json:"services,omitempty"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the envelope the route has always answered with. Two envelopes
// exist on this face and each Out mirrors the one its route uses: the reads
// that answer {"status":"success","data":…} say so, the reads that answer the
// payload bare say that instead. The data field carries omitempty exactly where
// the runtime's render.Success does, so an empty answer renders to the same
// bytes on both sides. Nothing embeds, because the schema builder walks
// reflected fields and cannot name an embedded one; runtime types that embed
// (User, the global config) are written FLAT here in their exact wire order.

// O11yMessage is an acknowledgment: the message the runtime answered with.
type O11yMessage struct {
	// Data is the acknowledgment message.
	Data string `json:"data"`
}

// O11yPromQueryOut is an instant query result.
type O11yPromQueryOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the evaluation result.
	Data O11yPromResult `json:"data"`
}

// O11yPromResult is a PromQL evaluation result.
type O11yPromResult struct {
	// ResultType discriminates Result: matrix, vector, scalar or string.
	ResultType string `json:"resultType"`
	// Result is the result in PromQL's own wire shape for ResultType, carried
	// verbatim.
	Result json.RawMessage `json:"result"`
	// Stats holds query statistics when the caller asked for them.
	Stats json.RawMessage `json:"stats,omitempty"`
}

// O11yDashboardVarsOut is a dashboard variable's value set.
type O11yDashboardVarsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the values.
	Data O11yDashboardVarValues `json:"data"`
}

// O11yDashboardVarValues are the values a dashboard variable may take.
type O11yDashboardVarValues struct {
	// VariableValues are the values, in the order the query produced them.
	VariableValues []any `json:"variableValues"`
}

// O11yUsageList is ingestion usage, one item per bucket, oldest first.
type O11yUsageList []O11yUsageItem

// O11yUsageItem is one usage bucket.
type O11yUsageItem struct {
	// Time is the bucket start.
	Time time.Time `json:"time,omitempty"`
	// Timestamp is the bucket start, as epoch nanoseconds.
	Timestamp uint64 `json:"timestamp"`
	// Count is how many spans were ingested in the bucket.
	Count uint64 `json:"count"`
}

// O11yDependencyList is the service dependency graph, one item per edge.
type O11yDependencyList []O11yDependency

// O11yDependency is one parent→child edge of the service graph.
type O11yDependency struct {
	// Parent is the calling service.
	Parent string `json:"parent"`
	// Child is the called service.
	Child string `json:"child"`
	// CallCount is how many calls crossed the edge in the window.
	CallCount uint64 `json:"callCount"`
	// CallRate is calls per second.
	CallRate float64 `json:"callRate"`
	// ErrorRate is the percentage of calls that erred.
	ErrorRate float64 `json:"errorRate"`
	// P99 is the 99th-percentile call duration, in nanoseconds.
	P99 float64 `json:"p99"`
	// P95 is the 95th-percentile call duration, in nanoseconds.
	P95 float64 `json:"p95"`
	// P90 is the 90th-percentile call duration, in nanoseconds.
	P90 float64 `json:"p90"`
	// P75 is the 75th-percentile call duration, in nanoseconds.
	P75 float64 `json:"p75"`
	// P50 is the median call duration, in nanoseconds.
	P50 float64 `json:"p50"`
}

// O11yVersionOut is the running build.
type O11yVersionOut struct {
	// Version is the build version.
	Version string `json:"version"`
	// EE says whether an enterprise edition is present; "N" in this build.
	EE string `json:"ee"`
	// SetupCompleted says whether the first user has been created.
	SetupCompleted bool `json:"setupCompleted"`
}

// O11yHealthOut is the health verdict.
type O11yHealthOut struct {
	// Status is "ok".
	Status string `json:"status"`
}

// O11yDiskList is the datastore's disks.
type O11yDiskList []O11yDisk

// O11yDisk is one storage disk.
type O11yDisk struct {
	// Name is the disk's name.
	Name string `json:"name,omitempty"`
	// Type is the disk's type, e.g. local or s3.
	Type string `json:"type,omitempty"`
}

// O11ySpanPercentileOut is a span's percentile placement.
type O11ySpanPercentileOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the placement.
	Data O11ySpanPercentile `json:"data"`
}

// O11ySpanPercentile places one span among its peers.
type O11ySpanPercentile struct {
	// Percentiles are the peer group's duration percentiles.
	Percentiles O11yPercentiles `json:"percentiles"`
	// Position is where the given duration lands.
	Position O11yPercentilePosition `json:"position"`
}

// O11yPercentiles are duration percentiles, in nanoseconds.
type O11yPercentiles struct {
	// P50 is the median.
	P50 float64 `json:"p50"`
	// P90 is the 90th percentile.
	P90 float64 `json:"p90"`
	// P99 is the 99th percentile.
	P99 float64 `json:"p99"`
}

// O11yPercentilePosition is one duration's rank among its peers.
type O11yPercentilePosition struct {
	// Percentile is the percentile the duration lands at.
	Percentile float64 `json:"percentile"`
	// Description says the same in words.
	Description string `json:"description"`
}

// O11yAnalyzeOut is a query analysis.
type O11yAnalyzeOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the analysis.
	Data O11yQueryFilterAnalysis `json:"data"`
}

// O11yQueryFilterAnalysis is what a query reads and groups by.
type O11yQueryFilterAnalysis struct {
	// MetricNames are the metrics the query reads.
	MetricNames []string `json:"metricNames"`
	// Groups are the columns the query groups by.
	Groups []O11yColumnInfo `json:"groups"`
}

// O11yColumnInfo is one grouping column.
type O11yColumnInfo struct {
	// Name is the column's name.
	Name string `json:"columnName"`
	// Alias is the column's alias in the query, when it has one.
	Alias string `json:"columnAlias"`
}

// O11yFilterSuggestionsOut is the query builder's suggestions.
type O11yFilterSuggestionsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the suggestions.
	Data O11yFilterSuggestions `json:"data"`
}

// O11yFilterSuggestions are attribute keys worth filtering on and example
// queries built from them.
type O11yFilterSuggestions struct {
	// Attributes are the suggested attribute keys.
	Attributes []O11yAttributeKey `json:"attributes"`
	// ExampleQueries are ready-to-run filter sets.
	ExampleQueries []O11yFilterSet `json:"example_queries"`
}

// O11yAttributeKey is one telemetry attribute.
type O11yAttributeKey struct {
	// Key is the attribute's name.
	Key string `json:"key"`
	// DataType is the attribute's value type — string, int64, float64 or bool.
	DataType string `json:"dataType"`
	// Type says where the attribute lives: tag or resource.
	Type string `json:"type"`
	// IsColumn marks an attribute materialized as its own column.
	IsColumn bool `json:"isColumn"`
	// IsJSON marks an attribute extracted from a JSON body.
	IsJSON bool `json:"isJSON"`
}

// O11yFilterSet is one filter: conditions combined with one operator.
type O11yFilterSet struct {
	// Operator combines the items — AND or OR.
	Operator string `json:"op,omitempty"`
	// Items are the conditions.
	Items []O11yFilterItem `json:"items"`
}

// O11yFilterItem is one filter condition.
type O11yFilterItem struct {
	// Key is the attribute tested.
	Key O11yAttributeKey `json:"key"`
	// Value is what it is tested against; its type follows the attribute's.
	Value any `json:"value"`
	// Operator is the comparison, e.g. =, !=, in, contains.
	Operator string `json:"op"`
}

// O11yOnboardingOut is Kubernetes onboarding progress.
type O11yOnboardingOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the progress report.
	Data O11yK8sOnboarding `json:"data"`
}

// O11yK8sOnboarding says which infra metric families have arrived.
type O11yK8sOnboarding struct {
	// DidSendPodMetrics says whether pod metrics have arrived.
	DidSendPodMetrics bool `json:"didSendPodMetrics"`
	// DidSendNodeMetrics says whether node metrics have arrived.
	DidSendNodeMetrics bool `json:"didSendNodeMetrics"`
	// DidSendClusterMetrics says whether cluster metrics have arrived.
	DidSendClusterMetrics bool `json:"didSendClusterMetrics"`
	// IsSendingOptionalPodMetrics says whether optional pod metrics are
	// flowing.
	IsSendingOptionalPodMetrics bool `json:"isSendingOptionalPodMetrics"`
	// IsSendingRequiredMetadata reports, per pod, which required metadata
	// labels are present.
	IsSendingRequiredMetadata []O11yPodOnboarding `json:"isSendingRequiredMetadata"`
}

// O11yPodOnboarding is one pod's metadata completeness.
type O11yPodOnboarding struct {
	// ClusterName is the pod's cluster.
	ClusterName string `json:"clusterName"`
	// NodeName is the pod's node.
	NodeName string `json:"nodeName"`
	// NamespaceName is the pod's namespace.
	NamespaceName string `json:"namespaceName"`
	// PodName is the pod.
	PodName string `json:"podName"`
	// HasClusterName says whether the cluster label is present.
	HasClusterName bool `json:"hasClusterName"`
	// HasNodeName says whether the node label is present.
	HasNodeName bool `json:"hasNodeName"`
	// HasNamespaceName says whether the namespace label is present.
	HasNamespaceName bool `json:"hasNamespaceName"`
	// HasDeploymentName says whether the deployment label is present.
	HasDeploymentName bool `json:"hasDeploymentName"`
	// HasStatefulsetName says whether the statefulset label is present.
	HasStatefulsetName bool `json:"hasStatefulsetName"`
	// HasDaemonsetName says whether the daemonset label is present.
	HasDaemonsetName bool `json:"hasDaemonsetName"`
	// HasCronjobName says whether the cronjob label is present.
	HasCronjobName bool `json:"hasCronjobName"`
	// HasJobName says whether the job label is present.
	HasJobName bool `json:"hasJobName"`
}

// O11yRetentionSetOut acknowledges a retention change.
type O11yRetentionSetOut struct {
	// Message says what was done.
	Message string `json:"message"`
}

// O11yRetentionOut is the org's retention policy.
type O11yRetentionOut struct {
	// Version is the policy format version.
	Version string `json:"version"`
	// Status is the last TTL operation's state.
	Status string `json:"status"`
	// ExpectedLogsTTLHours is the pending logs TTL, in hours.
	ExpectedLogsTTLHours int `json:"expected_logs_ttl_duration_hrs,omitempty"`
	// ExpectedLogsMoveTTLHours is the pending logs cold-storage move TTL, in
	// hours.
	ExpectedLogsMoveTTLHours int `json:"expected_logs_move_ttl_duration_hrs,omitempty"`
	// DefaultTTLDays is the retention for data no rule matches, in days.
	DefaultTTLDays int `json:"default_ttl_days,omitempty"`
	// TTLConditions are the ordered per-label rules; the first match wins.
	TTLConditions []O11yRetentionRule `json:"ttl_conditions,omitempty"`
	// ColdStorageVolume names the volume aged data moves to.
	ColdStorageVolume string `json:"cold_storage_volume,omitempty"`
	// ColdStorageTTLDays is how old data must be before it moves, in days.
	ColdStorageTTLDays int `json:"cold_storage_ttl_days,omitempty"`
}

// O11yRetentionRule is one per-label retention rule.
type O11yRetentionRule struct {
	// Conditions all have to hold for the rule to match.
	Conditions []O11yRetentionMatch `json:"conditions"`
	// TTLDays is the retention applied when it does, in days.
	TTLDays int `json:"ttlDays"`
}

// O11yRetentionMatch is one label condition of a retention rule.
type O11yRetentionMatch struct {
	// Key is the label to test.
	Key string `json:"key"`
	// Values are the label values the rule matches.
	Values []string `json:"values"`
}

// O11yApdexSetOut acknowledges an apdex change.
type O11yApdexSetOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the acknowledgment.
	Data O11yMessage `json:"data"`
}

// O11yApdexOut is the requested services' apdex settings.
type O11yApdexOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one settings row per service.
	Data []O11yApdexSettings `json:"data,omitempty"`
}

// O11yApdexSettings is one service's apdex settings.
type O11yApdexSettings struct {
	// ID is the settings row's id.
	ID string `json:"id"`
	// OrgID is the org the settings belong to.
	OrgID string `json:"orgId"`
	// ServiceName is the service.
	ServiceName string `json:"serviceName"`
	// Threshold is the satisfied-response time in seconds.
	Threshold float64 `json:"threshold"`
	// ExcludeStatusCodes are status codes excluded from the score, comma
	// separated.
	ExcludeStatusCodes string `json:"excludeStatusCodes"`
}

// O11yLicensesOut is the org's licenses — empty in this build, which has no
// enterprise edition.
type O11yLicensesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data are the licenses.
	Data []any `json:"data,omitempty"`
}

// O11yLicenseActiveOut is the activated license, in the licensing provider's
// own shape.
type O11yLicenseActiveOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the license.
	Data any `json:"data,omitempty"`
}

// O11yGlobalConfigOut is the deployment's global configuration.
type O11yGlobalConfigOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the configuration.
	Data O11yGlobalConfig `json:"data"`
}

// O11yGlobalConfig is the deployment's endpoints and identity providers, in
// the runtime's own wire order.
type O11yGlobalConfig struct {
	// ExternalURL is the deployment's public URL.
	ExternalURL string `json:"external_url"`
	// IngestionURL is where telemetry is sent.
	IngestionURL string `json:"ingestion_url"`
	// MCPURL is the MCP endpoint, when one is exposed.
	MCPURL *string `json:"mcp_url"`
	// AIAssistantURL is the AI assistant endpoint, when one is exposed.
	AIAssistantURL *string `json:"ai_assistant_url"`
	// IdentN says which identity providers are enabled.
	IdentN O11yIdentN `json:"identN"`
}

// O11yIdentN is the identity provider switchboard.
type O11yIdentN struct {
	// Tokenizer is the token-based provider.
	Tokenizer O11yToggle `json:"tokenizer"`
	// APIKey is the API-key provider.
	APIKey O11yToggle `json:"apikey"`
	// Impersonation is the impersonation provider.
	Impersonation O11yToggle `json:"impersonation"`
}

// O11yToggle is one on/off switch.
type O11yToggle struct {
	// Enabled says whether the feature is on.
	Enabled bool `json:"enabled"`
}

// O11yFeaturesOut is the supported feature flags.
type O11yFeaturesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data are the features.
	Data []O11yFeature `json:"data,omitempty"`
}

// O11yFeature is one feature flag.
type O11yFeature struct {
	// Name is the flag's name.
	Name string `json:"name"`
	// Kind is the flag's value kind, e.g. boolean.
	Kind string `json:"kind"`
	// Stage is the flag's lifecycle stage, e.g. stable.
	Stage string `json:"stage"`
	// Description says what the flag gates.
	Description string `json:"description"`
	// DefaultVariant is the variant used when nothing overrides it.
	DefaultVariant string `json:"defaultVariant"`
	// Variants are the flag's possible values, by variant name.
	Variants map[string]any `json:"variants"`
	// ResolvedValue is the value resolved for the caller's org.
	ResolvedValue any `json:"resolvedValue"`
}

// O11yOrgStatsOut is the org's collected usage statistics.
type O11yOrgStatsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data are the statistics, keyed by the reporter's own counter names.
	Data map[string]any `json:"data,omitempty"`
}
