package o11y

// The RULES & ALERTING face — alert rules, their test-fire, planned
// maintenance (downtime) schedules, rule state history, notification channels,
// route policies and the alerts read — as TYPED ops.
//
// These thirty-six routes reached traffic only through the delegation
// wildcard, and a route behind a wildcard is in no document: no SDK method, no
// CLI command, no agent tool, no reference page. A customer could not create,
// test, list or silence an alert rule, wire a notification channel, or read a
// rule's firing history from anything but a hand-written HTTP call. Typing the
// face is what puts the operations in the document and therefore in every
// projection built from it.
//
// THE WIRE DOES NOT MOVE, the same way telemetry.go, infra.go, logs.go and
// identity.go do not move it: no op here re-implements a read or a write. Each
// hands the call to the SAME runtime handler the wildcard delegates to (see
// rulesAlertsRelay), so identity resolution, the org gate, the ROLE CHECK the
// mux registration declared — ViewAccess, EditAccess or AdminAccess — the
// audit record, the timeout and the success envelope are all still the
// runtime's, executed in the order they always were. The gate is NOT re-stated
// here because it is not enforced here; each op's comment names the gate its
// route has always had, and the runtime keeps enforcing it one layer in.
//
// The request and response payloads are the RUNTIME'S OWN types
// (pkg/types/ruletypes, pkg/types/alertmanagertypes,
// pkg/types/rulestatehistorytypes, pkg/types/telemetrytypes and, for the four
// legacy v1 history writes, pkg/query-service/model), not mirrors of them: the
// field list that decodes a request here is the field list the handler decodes
// one layer in, so the two cannot drift. Only the {status, data} envelopes and
// the GET query-parameter shapes are NAMED here, because the runtime spells
// those on the wire rather than in a type.
//
// TWO history faces sit at the same four paths on purpose. The v2 reads answer
// GET with URL query parameters (rulestatehistory.go, o11yapiserver); the v1
// reads answer POST with a JSON body (routes_rules.go, query-service). They are
// distinct method+path operations and each keeps its own contract — the flatten
// left both live, so both are typed rather than one silently chosen.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these paths dispatch here
// and every other path under the prefix still reaches the runtime untouched.

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/hanzoai/o11y/pkg/types/alertmanagertypes"
	"github.com/hanzoai/o11y/pkg/types/rulestatehistorytypes"
	"github.com/hanzoai/o11y/pkg/types/ruletypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/zap-proto/zip"
)

// mountRulesAlerts registers the rules & alerting face's typed ops on the
// native router. Collection routes sit beside their parameterised siblings
// safely: the router matches the most specific pattern, so /rules/test is never
// swallowed by /rules/:id, the same disambiguation the runtime's own tree makes.
func mountRulesAlerts(app *zip.App) {
	g := app.Group(o11yRoot)

	// alert rules (runtime gate per op: see each op's comment)
	zip.Get(g, "/rules", listRules, zip.WithOperationID("ListRules"))
	zip.Get(g, "/rules/:id", getRuleByID, zip.WithOperationID("GetRuleByID"))
	zip.Post(g, "/rules", createRule, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateRule"))
	zip.Put(g, "/rules/:id", updateRuleByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("UpdateRuleByID"))
	zip.Delete(g, "/rules/:id", deleteRuleByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("DeleteRuleByID"))
	zip.Patch(g, "/rules/:id", patchRuleByID, zip.WithOperationID("PatchRuleByID"))
	zip.Post(g, "/rules/test", testRule, zip.WithOperationID("TestRule"))

	// planned maintenance / downtime schedules
	zip.Get(g, "/downtime_schedules", listDowntimeSchedules, zip.WithOperationID("ListDowntimeSchedules"))
	zip.Get(g, "/downtime_schedules/:id", getDowntimeScheduleByID, zip.WithOperationID("GetDowntimeScheduleByID"))
	zip.Post(g, "/downtime_schedules", createDowntimeSchedule, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateDowntimeSchedule"))
	zip.Put(g, "/downtime_schedules/:id", updateDowntimeScheduleByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("UpdateDowntimeScheduleByID"))
	zip.Delete(g, "/downtime_schedules/:id", deleteDowntimeScheduleByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("DeleteDowntimeScheduleByID"))

	// rule state history — v2 reads (GET, URL query parameters)
	zip.Get(g, "/rules/:id/history/stats", getRuleHistoryStats, zip.WithOperationID("GetRuleHistoryStats"))
	zip.Get(g, "/rules/:id/history/timeline", getRuleHistoryTimeline, zip.WithOperationID("GetRuleHistoryTimeline"))
	zip.Get(g, "/rules/:id/history/top_contributors", getRuleHistoryTopContributors, zip.WithOperationID("GetRuleHistoryTopContributors"))
	zip.Get(g, "/rules/:id/history/filter_keys", getRuleHistoryFilterKeys, zip.WithOperationID("GetRuleHistoryFilterKeys"))
	zip.Get(g, "/rules/:id/history/filter_values", getRuleHistoryFilterValues, zip.WithOperationID("GetRuleHistoryFilterValues"))
	zip.Get(g, "/rules/:id/history/overall_status", getRuleHistoryOverallStatus, zip.WithOperationID("GetRuleHistoryOverallStatus"))

	// rule state history — v1 reads (POST, JSON body) and the legacy test-fire
	zip.Post(g, "/testRule", testRuleNotification, zip.WithOperationID("TestRuleNotification"))
	zip.Post(g, "/rules/:id/history/stats", getRuleStats, zip.WithOperationID("GetRuleStats"))
	zip.Post(g, "/rules/:id/history/timeline", getRuleStateHistory, zip.WithOperationID("GetRuleStateHistory"))
	zip.Post(g, "/rules/:id/history/top_contributors", getRuleStateHistoryTopContributors, zip.WithOperationID("GetRuleStateHistoryTopContributors"))
	zip.Post(g, "/rules/:id/history/overall_status", getOverallStateTransitions, zip.WithOperationID("GetOverallStateTransitions"))

	// notification channels
	zip.Get(g, "/channels", listChannels, zip.WithOperationID("ListChannels"))
	zip.Get(g, "/channels/:id", getChannelByID, zip.WithOperationID("GetChannelByID"))
	zip.Post(g, "/channels", createChannel, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateChannel"))
	zip.Put(g, "/channels/:id", updateChannelByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("UpdateChannelByID"))
	zip.Delete(g, "/channels/:id", deleteChannelByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("DeleteChannelByID"))
	zip.Post(g, "/channels/test", testChannel, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("TestChannel"))
	zip.Post(g, "/testChannel", testChannelDeprecated, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("TestChannelDeprecated"))

	// route policies
	zip.Get(g, "/route_policies", getAllRoutePolicies, zip.WithOperationID("GetAllRoutePolicies"))
	zip.Get(g, "/route_policies/:id", getRoutePolicyByID, zip.WithOperationID("GetRoutePolicyByID"))
	zip.Post(g, "/route_policies", createRoutePolicy, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateRoutePolicy"))
	zip.Put(g, "/route_policies/:id", updateRoutePolicy, zip.WithOperationID("UpdateRoutePolicy"))
	zip.Delete(g, "/route_policies/:id", deleteRoutePolicyByID, zip.WithStatus(http.StatusNoContent), zip.WithOperationID("DeleteRoutePolicyByID"))

	// alerts
	zip.Get(g, "/alerts", getAlerts, zip.WithOperationID("GetAlerts"))
}

// ── alert rules ─────────────────────────────────────────────────────────────

// listRules lists all alert rules with their current evaluation state. Viewer gate.
func listRules(ctx context.Context, _ *struct{}) (*O11yRulesOut, error) {
	out := new(O11yRulesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules", nil, nil, out)
}

// getRuleByID returns one alert rule with its evaluation state, by id. Viewer gate.
func getRuleByID(ctx context.Context, in *O11yRuleRef) (*O11yRuleOut, error) {
	out := new(O11yRuleOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID, nil, nil, out)
}

// createRule creates a new alert rule and answers with the stored rule. Editor gate.
func createRule(ctx context.Context, in *ruletypes.PostableRule) (*O11yRuleOut, error) {
	out := new(O11yRuleOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/rules", nil, in, out)
}

// updateRuleByID replaces an alert rule's definition, by id. Editor gate.
func updateRuleByID(ctx context.Context, in *O11yRuleUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/rules/"+in.ID, nil, &in.PostableRule, nil)
}

// deleteRuleByID removes an alert rule, by id. Editor gate.
func deleteRuleByID(ctx context.Context, in *O11yRuleRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/rules/"+in.ID, nil, nil, nil)
}

// patchRuleByID applies a partial update to an alert rule, by id, answering
// with the stored rule — the common toggle for enabling or muting a rule.
// Editor gate.
func patchRuleByID(ctx context.Context, in *O11yRuleUpdateIn) (*O11yRuleOut, error) {
	out := new(O11yRuleOut)
	return out, relay(ctx, http.MethodPatch, o11yRoot+"/rules/"+in.ID, nil, &in.PostableRule, out)
}

// testRule fires a test notification for a rule definition without saving it,
// answering with how many series would alert. Editor gate.
func testRule(ctx context.Context, in *ruletypes.PostableRule) (*O11yTestRuleOut, error) {
	out := new(O11yTestRuleOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/rules/test", nil, in, out)
}

// ── planned maintenance / downtime schedules ────────────────────────────────

// listDowntimeSchedules lists all planned maintenance windows, optionally
// narrowed to the active ones or the recurring ones. Viewer gate.
func listDowntimeSchedules(ctx context.Context, in *O11yListDowntimeSchedulesIn) (*O11yDowntimeSchedulesOut, error) {
	out := new(O11yDowntimeSchedulesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/downtime_schedules", query(
		"active", in.Active,
		"recurring", in.Recurring,
	), nil, out)
}

// getDowntimeScheduleByID returns one planned maintenance window, by id. Viewer gate.
func getDowntimeScheduleByID(ctx context.Context, in *O11yDowntimeRef) (*O11yDowntimeScheduleOut, error) {
	out := new(O11yDowntimeScheduleOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/downtime_schedules/"+in.ID, nil, nil, out)
}

// createDowntimeSchedule creates a planned maintenance window, answering with
// the stored schedule. Editor gate.
func createDowntimeSchedule(ctx context.Context, in *alertmanagertypes.PostablePlannedMaintenance) (*O11yDowntimeScheduleOut, error) {
	out := new(O11yDowntimeScheduleOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/downtime_schedules", nil, in, out)
}

// updateDowntimeScheduleByID replaces a planned maintenance window, by id. Editor gate.
func updateDowntimeScheduleByID(ctx context.Context, in *O11yDowntimeUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/downtime_schedules/"+in.ID, nil, &in.PostablePlannedMaintenance, nil)
}

// deleteDowntimeScheduleByID removes a planned maintenance window, by id. Editor gate.
func deleteDowntimeScheduleByID(ctx context.Context, in *O11yDowntimeRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/downtime_schedules/"+in.ID, nil, nil, nil)
}

// ── rule state history — v2 (GET, query parameters) ─────────────────────────

// getRuleHistoryStats returns trigger and resolution statistics for a rule over
// the selected time range, current window against the prior one. Viewer gate.
func getRuleHistoryStats(ctx context.Context, in *O11yRuleHistoryBaseIn) (*O11yRuleHistoryStatsOut, error) {
	out := new(O11yRuleHistoryStatsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID+"/history/stats", ruleHistoryBaseParams(in), nil, out)
}

// getRuleHistoryTimeline returns paginated timeline entries for a rule's state
// transitions, filterable by state and a label expression, cursor-paginated. Viewer gate.
func getRuleHistoryTimeline(ctx context.Context, in *O11yRuleHistoryTimelineIn) (*O11yRuleHistoryTimelineOut, error) {
	out := new(O11yRuleHistoryTimelineOut)
	params := ruleHistoryBaseParams(&O11yRuleHistoryBaseIn{Start: in.Start, End: in.End})
	if in.State != "" {
		params.Set("state", in.State)
	}
	if in.FilterExpression != "" {
		params.Set("filterExpression", in.FilterExpression)
	}
	if in.Limit != 0 {
		params.Set("limit", strconv.FormatInt(in.Limit, 10))
	}
	if in.Order != "" {
		params.Set("order", in.Order)
	}
	if in.Cursor != "" {
		params.Set("cursor", in.Cursor)
	}
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID+"/history/timeline", params, nil, out)
}

// getRuleHistoryTopContributors returns the label combinations that contributed
// most to a rule firing over the selected range. Viewer gate.
func getRuleHistoryTopContributors(ctx context.Context, in *O11yRuleHistoryBaseIn) (*O11yRuleHistoryContributorsOut, error) {
	out := new(O11yRuleHistoryContributorsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID+"/history/top_contributors", ruleHistoryBaseParams(in), nil, out)
}

// getRuleHistoryFilterKeys returns the distinct label keys present in a rule's
// history entries over the selected range, for building history filters. Viewer gate.
func getRuleHistoryFilterKeys(ctx context.Context, in *O11yRuleHistoryFilterKeysIn) (*O11yRuleHistoryFilterKeysOut, error) {
	out := new(O11yRuleHistoryFilterKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID+"/history/filter_keys", filterKeysParams(in), nil, out)
}

// getRuleHistoryFilterValues returns the distinct values a given label key has
// taken across a rule's history entries. Viewer gate.
func getRuleHistoryFilterValues(ctx context.Context, in *O11yRuleHistoryFilterValuesIn) (*O11yRuleHistoryFilterValuesOut, error) {
	out := new(O11yRuleHistoryFilterValuesOut)
	params := filterKeysParams(&in.O11yRuleHistoryFilterKeysIn)
	params.Set("name", in.Name)
	if in.ExistingQuery != "" {
		params.Set("existingQuery", in.ExistingQuery)
	}
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID+"/history/filter_values", params, nil, out)
}

// getRuleHistoryOverallStatus returns the overall firing/inactive intervals for
// a rule over the selected range. Viewer gate.
func getRuleHistoryOverallStatus(ctx context.Context, in *O11yRuleHistoryBaseIn) (*O11yRuleHistoryOverallStatusOut, error) {
	out := new(O11yRuleHistoryOverallStatusOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/rules/"+in.ID+"/history/overall_status", ruleHistoryBaseParams(in), nil, out)
}

// ── rule state history — v1 (POST, JSON body) + legacy test-fire ────────────

// testRuleNotification fires a test notification for the posted rule definition
// and answers with how many series alerted and a status message. The legacy
// path; prefer /rules/test. Editor gate.
func testRuleNotification(ctx context.Context, in *ruletypes.PostableRule) (*O11yTestNotificationOut, error) {
	out := new(O11yTestNotificationOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/testRule", nil, in, out)
}

// getRuleStats returns trigger and resolution statistics for a rule, current
// window against the prior one, for the posted query range. Viewer gate.
func getRuleStats(ctx context.Context, in *O11yRuleHistoryQueryIn) (*O11yRuleStatsOut, error) {
	out := new(O11yRuleStatsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/rules/"+in.ID+"/history/stats", nil, &in.QueryRuleStateHistory, out)
}

// getRuleStateHistory returns a rule's state-transition timeline for the posted
// query range, each entry carrying its related-logs or related-traces link. Viewer gate.
func getRuleStateHistory(ctx context.Context, in *O11yRuleHistoryQueryIn) (*O11yRuleStateTimelineOut, error) {
	out := new(O11yRuleStateTimelineOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/rules/"+in.ID+"/history/timeline", nil, &in.QueryRuleStateHistory, out)
}

// getRuleStateHistoryTopContributors returns the label combinations that
// contributed most to a rule firing, for the posted query range. Viewer gate.
func getRuleStateHistoryTopContributors(ctx context.Context, in *O11yRuleHistoryQueryIn) (*O11yRuleStateContributorsOut, error) {
	out := new(O11yRuleStateContributorsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/rules/"+in.ID+"/history/top_contributors", nil, &in.QueryRuleStateHistory, out)
}

// getOverallStateTransitions returns the overall firing/inactive windows for a
// rule, for the posted query range. Viewer gate.
func getOverallStateTransitions(ctx context.Context, in *O11yRuleHistoryQueryIn) (*O11yOverallStateTransitionsOut, error) {
	out := new(O11yOverallStateTransitionsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/rules/"+in.ID+"/history/overall_status", nil, &in.QueryRuleStateHistory, out)
}

// ── notification channels ───────────────────────────────────────────────────

// listChannels lists the org's notification channels. Viewer gate.
func listChannels(ctx context.Context, _ *struct{}) (*O11yChannelsOut, error) {
	out := new(O11yChannelsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/channels", nil, nil, out)
}

// getChannelByID returns one notification channel, by id. Viewer gate.
func getChannelByID(ctx context.Context, in *O11yChannelRef) (*O11yChannelOut, error) {
	out := new(O11yChannelOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/channels/"+in.ID, nil, nil, out)
}

// createChannel creates a notification channel, answering with the stored
// channel. Admin gate.
func createChannel(ctx context.Context, in *alertmanagertypes.PostableChannel) (*O11yChannelOut, error) {
	out := new(O11yChannelOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/channels", nil, in, out)
}

// updateChannelByID replaces a notification channel's receiver, by id. Admin gate.
func updateChannelByID(ctx context.Context, in *O11yChannelUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/channels/"+in.ID, nil, &in.Receiver, nil)
}

// deleteChannelByID removes a notification channel, by id. Admin gate.
func deleteChannelByID(ctx context.Context, in *O11yChannelRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/channels/"+in.ID, nil, nil, nil)
}

// testChannel sends a test notification to the posted receiver. Editor gate.
func testChannel(ctx context.Context, in *alertmanagertypes.Receiver) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPost, o11yRoot+"/channels/test", nil, in, nil)
}

// testChannelDeprecated sends a test notification to the posted receiver. The
// legacy path; prefer /channels/test. Editor gate.
func testChannelDeprecated(ctx context.Context, in *alertmanagertypes.Receiver) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPost, o11yRoot+"/testChannel", nil, in, nil)
}

// ── route policies ──────────────────────────────────────────────────────────

// getAllRoutePolicies lists the org's route policies. Viewer gate.
func getAllRoutePolicies(ctx context.Context, _ *struct{}) (*O11yRoutePoliciesOut, error) {
	out := new(O11yRoutePoliciesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/route_policies", nil, nil, out)
}

// getRoutePolicyByID returns one route policy, by id. Viewer gate.
func getRoutePolicyByID(ctx context.Context, in *O11yRoutePolicyRef) (*O11yRoutePolicyOut, error) {
	out := new(O11yRoutePolicyOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/route_policies/"+in.ID, nil, nil, out)
}

// createRoutePolicy creates a route policy, answering with the stored policy. Admin gate.
func createRoutePolicy(ctx context.Context, in *alertmanagertypes.PostableRoutePolicy) (*O11yRoutePolicyOut, error) {
	out := new(O11yRoutePolicyOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/route_policies", nil, in, out)
}

// updateRoutePolicy replaces a route policy, by id, answering with the stored
// policy. Admin gate.
func updateRoutePolicy(ctx context.Context, in *O11yRoutePolicyUpdateIn) (*O11yRoutePolicyOut, error) {
	out := new(O11yRoutePolicyOut)
	return out, relay(ctx, http.MethodPut, o11yRoot+"/route_policies/"+in.ID, nil, &in.PostableRoutePolicy, out)
}

// deleteRoutePolicyByID removes a route policy, by id. Admin gate.
func deleteRoutePolicyByID(ctx context.Context, in *O11yRoutePolicyRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/route_policies/"+in.ID, nil, nil, nil)
}

// ── alerts ──────────────────────────────────────────────────────────────────

// getAlerts returns the org's current alerts. Viewer gate.
func getAlerts(ctx context.Context, _ *struct{}) (*O11yAlertsOut, error) {
	out := new(O11yAlertsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/alerts", nil, nil, out)
}

// ── query-parameter builders ────────────────────────────────────────────────

// ruleHistoryBaseParams renders the {start, end} window every v2 history read
// shares. The runtime spells these as millisecond query parameters; a zero is
// "unsent", which the runtime then refuses, exactly as it always has.
func ruleHistoryBaseParams(in *O11yRuleHistoryBaseIn) url.Values {
	p := url.Values{}
	if in.Start != 0 {
		p.Set("start", strconv.FormatInt(in.Start, 10))
	}
	if in.End != 0 {
		p.Set("end", strconv.FormatInt(in.End, 10))
	}
	return p
}

// filterKeysParams renders the window, search text and limit the history filter
// reads share. startUnixMilli/endUnixMilli are the runtime's own parameter
// names for this pair of reads.
func filterKeysParams(in *O11yRuleHistoryFilterKeysIn) url.Values {
	p := url.Values{}
	if in.StartUnixMilli != 0 {
		p.Set("startUnixMilli", strconv.FormatInt(in.StartUnixMilli, 10))
	}
	if in.EndUnixMilli != 0 {
		p.Set("endUnixMilli", strconv.FormatInt(in.EndUnixMilli, 10))
	}
	if in.SearchText != "" {
		p.Set("searchText", in.SearchText)
	}
	if in.Limit != 0 {
		p.Set("limit", strconv.Itoa(in.Limit))
	}
	return p
}

// ── inputs ──────────────────────────────────────────────────────────────────
//
// The path-borne id names itself for the URL with `url:` and stays out of the
// body with `json:"-"`. The GET query inputs are named here because the runtime
// spells them as query parameters rather than as a type; no field carries
// validate: the runtime is the one place these are judged, so a bad or missing
// value answers with the status it has always answered with. The write bodies
// are the runtime's own request types, embedded so their fields decode and
// re-encode as the runtime spells them, never mirrored.

// O11yRuleRef addresses one alert rule by id.
type O11yRuleRef struct {
	// ID is the alert rule id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yRuleUpdateIn is a rule id plus the rule definition to store at it.
type O11yRuleUpdateIn struct {
	// ID is the alert rule id.
	ID string `json:"-" url:"id" validate:"required"`
	ruletypes.PostableRule
}

// O11yDowntimeRef addresses one planned maintenance window by id.
type O11yDowntimeRef struct {
	// ID is the downtime schedule id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yDowntimeUpdateIn is a schedule id plus the window to store at it.
type O11yDowntimeUpdateIn struct {
	// ID is the downtime schedule id.
	ID string `json:"-" url:"id" validate:"required"`
	alertmanagertypes.PostablePlannedMaintenance
}

// O11yListDowntimeSchedulesIn narrows the downtime list.
type O11yListDowntimeSchedulesIn struct {
	// Active, when "true" or "false", keeps only the active or inactive windows.
	// Absent lists all.
	Active string `json:"active"`
	// Recurring, when "true" or "false", keeps only the recurring or one-off
	// windows. Absent lists all.
	Recurring string `json:"recurring"`
}

// O11yRuleHistoryBaseIn is the id and time window shared by the stats,
// top-contributors and overall-status v2 reads.
type O11yRuleHistoryBaseIn struct {
	// ID is the alert rule id.
	ID string `json:"-" url:"id" validate:"required"`
	// Start is the window start, unix milliseconds. Required by the runtime.
	Start int64 `json:"start"`
	// End is the window end, unix milliseconds. Required by the runtime.
	End int64 `json:"end"`
}

// O11yRuleHistoryTimelineIn is the timeline read's id, window and paging.
type O11yRuleHistoryTimelineIn struct {
	// ID is the alert rule id.
	ID string `json:"-" url:"id" validate:"required"`
	// Start is the window start, unix milliseconds. Required by the runtime.
	Start int64 `json:"start"`
	// End is the window end, unix milliseconds. Required by the runtime.
	End int64 `json:"end"`
	// State keeps only entries in one alert state, e.g. firing or normal.
	State string `json:"state"`
	// FilterExpression narrows entries to those whose labels match it.
	FilterExpression string `json:"filterExpression"`
	// Limit caps how many entries come back. Absent means 50.
	Limit int64 `json:"limit"`
	// Order sorts by time, asc or desc.
	Order string `json:"order"`
	// Cursor resumes a previous page; opaque, returned as nextCursor.
	Cursor string `json:"cursor"`
}

// O11yRuleHistoryFilterKeysIn is the id, window, search and limit the history
// filter-keys read takes.
type O11yRuleHistoryFilterKeysIn struct {
	// ID is the alert rule id.
	ID string `json:"-" url:"id" validate:"required"`
	// StartUnixMilli is the window start, unix milliseconds.
	StartUnixMilli int64 `json:"startUnixMilli"`
	// EndUnixMilli is the window end, unix milliseconds.
	EndUnixMilli int64 `json:"endUnixMilli"`
	// SearchText narrows the keys to those containing it.
	SearchText string `json:"searchText"`
	// Limit caps how many keys come back. Absent means 50, capped at 200.
	Limit int `json:"limit"`
}

// O11yRuleHistoryFilterValuesIn adds the key to read values for, and an
// optional existing filter to scope them.
type O11yRuleHistoryFilterValuesIn struct {
	O11yRuleHistoryFilterKeysIn
	// Name is the label key whose values to list. Required.
	Name string `json:"name" validate:"required"`
	// ExistingQuery is a filter expression scoping which values appear.
	ExistingQuery string `json:"existingQuery"`
}

// O11yRuleHistoryQueryIn is the rule id plus the posted query range the four v1
// history reads share.
type O11yRuleHistoryQueryIn struct {
	// ID is the alert rule id.
	ID string `json:"-" url:"id" validate:"required"`
	model.QueryRuleStateHistory
}

// O11yChannelRef addresses one notification channel by id.
type O11yChannelRef struct {
	// ID is the notification channel id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yChannelUpdateIn is a channel id plus the receiver to store at it.
type O11yChannelUpdateIn struct {
	// ID is the notification channel id.
	ID string `json:"-" url:"id" validate:"required"`
	alertmanagertypes.Receiver
}

// O11yRoutePolicyRef addresses one route policy by id.
type O11yRoutePolicyRef struct {
	// ID is the route policy id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yRoutePolicyUpdateIn is a policy id plus the policy to store at it.
type O11yRoutePolicyUpdateIn struct {
	// ID is the route policy id.
	ID string `json:"-" url:"id" validate:"required"`
	alertmanagertypes.PostableRoutePolicy
}

// ── outputs ─────────────────────────────────────────────────────────────────
//
// Each Out names the runtime's answer: the {status, data} envelope every read
// and write on this face has always answered with, around the runtime's own
// response type. Status is declared first because the runtime writes it first,
// and a typed op that changes the bytes is a break, not a migration.

// O11yRulesOut is a list of alert rules.
type O11yRulesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rules.
	Data []*ruletypes.Rule `json:"data,omitempty"`
}

// O11yRuleOut is one alert rule.
type O11yRuleOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rule.
	Data ruletypes.Rule `json:"data,omitempty"`
}

// O11yTestRuleOut is the result of a saved-rule test fire.
type O11yTestRuleOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds how many series would alert.
	Data ruletypes.GettableTestRule `json:"data,omitempty"`
}

// O11yDowntimeSchedulesOut is a list of planned maintenance windows.
type O11yDowntimeSchedulesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the schedules.
	Data []*alertmanagertypes.PlannedMaintenance `json:"data,omitempty"`
}

// O11yDowntimeScheduleOut is one planned maintenance window.
type O11yDowntimeScheduleOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the schedule.
	Data alertmanagertypes.PlannedMaintenance `json:"data,omitempty"`
}

// O11yRuleHistoryStatsOut is a rule's trigger/resolution statistics.
type O11yRuleHistoryStatsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the statistics.
	Data rulestatehistorytypes.GettableRuleStateHistoryStats `json:"data,omitempty"`
}

// O11yRuleHistoryTimelineOut is a page of a rule's state transitions.
type O11yRuleHistoryTimelineOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the timeline and its paging cursor.
	Data rulestatehistorytypes.GettableRuleStateTimeline `json:"data,omitempty"`
}

// O11yRuleHistoryContributorsOut is a rule's top firing contributors.
type O11yRuleHistoryContributorsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the contributors.
	Data []rulestatehistorytypes.GettableRuleStateHistoryContributor `json:"data,omitempty"`
}

// O11yRuleHistoryFilterKeysOut is the label keys present in a rule's history.
type O11yRuleHistoryFilterKeysOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the keys.
	Data telemetrytypes.GettableFieldKeys `json:"data,omitempty"`
}

// O11yRuleHistoryFilterValuesOut is the values one history label key has taken.
type O11yRuleHistoryFilterValuesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the values.
	Data telemetrytypes.GettableFieldValues `json:"data,omitempty"`
}

// O11yRuleHistoryOverallStatusOut is a rule's firing/inactive windows.
type O11yRuleHistoryOverallStatusOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the windows.
	Data []rulestatehistorytypes.GettableRuleStateWindow `json:"data,omitempty"`
}

// O11yTestNotificationOut is the result of a legacy rule test fire.
type O11yTestNotificationOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the fired-series count and a status message.
	Data O11yTestNotificationResult `json:"data,omitempty"`
}

// O11yTestNotificationResult is how many series alerted, and a message.
type O11yTestNotificationResult struct {
	// AlertCount is how many series would alert for the tested rule.
	AlertCount int `json:"alertCount"`
	// Message is a human-readable status, e.g. "notification sent".
	Message string `json:"message"`
}

// O11yRuleStatsOut is a rule's v1 trigger/resolution statistics.
type O11yRuleStatsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the statistics.
	Data model.Stats `json:"data,omitempty"`
}

// O11yRuleStateTimelineOut is a rule's v1 state-transition timeline.
type O11yRuleStateTimelineOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the timeline.
	Data model.RuleStateTimeline `json:"data,omitempty"`
}

// O11yRuleStateContributorsOut is a rule's v1 top firing contributors.
type O11yRuleStateContributorsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the contributors.
	Data []model.RuleStateHistoryContributor `json:"data,omitempty"`
}

// O11yOverallStateTransitionsOut is a rule's v1 firing/inactive windows.
type O11yOverallStateTransitionsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the windows.
	Data []model.ReleStateItem `json:"data,omitempty"`
}

// O11yChannelsOut is a list of notification channels.
type O11yChannelsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the channels.
	Data []*alertmanagertypes.Channel `json:"data,omitempty"`
}

// O11yChannelOut is one notification channel.
type O11yChannelOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the channel.
	Data alertmanagertypes.Channel `json:"data,omitempty"`
}

// O11yRoutePoliciesOut is a list of route policies.
type O11yRoutePoliciesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the policies.
	Data []*alertmanagertypes.GettableRoutePolicy `json:"data,omitempty"`
}

// O11yRoutePolicyOut is one route policy.
type O11yRoutePolicyOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the policy.
	Data alertmanagertypes.GettableRoutePolicy `json:"data,omitempty"`
}

// O11yAlertsOut is the org's current alerts.
type O11yAlertsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the alerts.
	Data alertmanagertypes.DeprecatedGettableAlerts `json:"data,omitempty"`
}
