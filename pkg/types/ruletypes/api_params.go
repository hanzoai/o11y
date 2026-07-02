package ruletypes

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/prometheus/alertmanager/config"

	o11yError "github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/alertmanagertypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type AlertType string

const (
	AlertTypeMetric     AlertType = "METRIC_BASED_ALERT"
	AlertTypeTraces     AlertType = "TRACES_BASED_ALERT"
	AlertTypeLogs       AlertType = "LOGS_BASED_ALERT"
	AlertTypeExceptions AlertType = "EXCEPTIONS_BASED_ALERT"
)

// Enum implements jsonschema.Enum; returns the acceptable values for AlertType.
func (AlertType) Enum() []any {
	return []any{
		AlertTypeMetric,
		AlertTypeTraces,
		AlertTypeLogs,
		AlertTypeExceptions,
	}
}

const (
	DefaultSchemaVersion  = "v1"
	SchemaVersionV2Alpha1 = "v2alpha1"
)

type RuleDataKind string

const (
	RuleDataKindJson RuleDataKind = "json"
)

// PostableRule is used to create alerting rule from HTTP api.
type PostableRule struct {
	AlertName   string              `json:"alert" required:"true"`
	AlertType   AlertType           `json:"alertType" required:"true"`
	Description string              `json:"description,omitempty"`
	RuleType    RuleType            `json:"ruleType" required:"true"`
	EvalWindow  valuer.TextDuration `json:"evalWindow,omitzero"`
	Frequency   valuer.TextDuration `json:"frequency,omitzero"`

	RuleCondition *RuleCondition    `json:"condition" required:"true"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`

	Disabled bool `json:"disabled"`

	// Source captures the source url where rule has been created
	Source string `json:"source,omitempty"`

	PreferredChannels []string `json:"preferredChannels,omitempty"`

	Version string `json:"version"`

	Evaluation    *EvaluationEnvelope `json:"evaluation,omitempty"`
	SchemaVersion string              `json:"schemaVersion,omitempty"`

	NotificationSettings *NotificationSettings `json:"notificationSettings,omitempty"`
}

type NotificationSettings struct {
	GroupBy   []string `json:"groupBy,omitempty"`
	Renotify  Renotify `json:"renotify,omitzero"`
	UsePolicy bool     `json:"usePolicy,omitempty"`
	// NewGroupEvalDelay is the grace period for new series to be excluded from alerts evaluation
	NewGroupEvalDelay valuer.TextDuration `json:"newGroupEvalDelay,omitzero"`
}

type Renotify struct {
	Enabled          bool                `json:"enabled"`
	ReNotifyInterval valuer.TextDuration `json:"interval,omitzero"`
	AlertStates      []AlertState        `json:"alertStates,omitempty"`
}

func (ns *NotificationSettings) GetAlertManagerNotificationConfig() alertmanagertypes.NotificationConfig {
	var renotifyInterval time.Duration
	var noDataRenotifyInterval time.Duration
	if ns.Renotify.Enabled {
		if slices.Contains(ns.Renotify.AlertStates, StateNoData) {
			noDataRenotifyInterval = ns.Renotify.ReNotifyInterval.Duration()
		}
		if slices.Contains(ns.Renotify.AlertStates, StateFiring) {
			renotifyInterval = ns.Renotify.ReNotifyInterval.Duration()
		}
	} else {
		renotifyInterval = 8760 * time.Hour //1 year for no renotify substitute
		noDataRenotifyInterval = 8760 * time.Hour
	}
	return alertmanagertypes.NewNotificationConfig(ns.GroupBy, renotifyInterval, noDataRenotifyInterval, ns.UsePolicy)
}

// Channels returns all unique channel names referenced by the rule's thresholds.
func (r *PostableRule) Channels() []string {
	if r.RuleCondition == nil || r.RuleCondition.Thresholds == nil {
		return nil
	}
	threshold, err := r.RuleCondition.Thresholds.GetRuleThreshold()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var channels []string
	for _, receiver := range threshold.GetRuleReceivers() {
		for _, ch := range receiver.Channels {
			if _, ok := seen[ch]; !ok {
				seen[ch] = struct{}{}
				channels = append(channels, ch)
			}
		}
	}
	return channels
}

func (r *PostableRule) GetRuleRouteRequest(ruleID string) ([]*alertmanagertypes.PostableRoutePolicy, error) {
	threshold, err := r.RuleCondition.Thresholds.GetRuleThreshold()
	if err != nil {
		return nil, err
	}
	receivers := threshold.GetRuleReceivers()
	routeRequests := make([]*alertmanagertypes.PostableRoutePolicy, 0)
	for _, receiver := range receivers {
		expression := fmt.Sprintf(`%s == "%s" && %s == "%s"`, LabelThresholdName, receiver.Name, LabelRuleID, ruleID)
		routeRequests = append(routeRequests, &alertmanagertypes.PostableRoutePolicy{
			Expression:     expression,
			ExpressionKind: alertmanagertypes.RuleBasedExpression,
			Channels:       receiver.Channels,
			Name:           ruleID,
			Description:    fmt.Sprintf("Auto-generated route for rule %s", ruleID),
			Tags:           []string{"auto-generated", "rule-based"},
		})
	}
	return routeRequests, nil
}

func (r *PostableRule) GetInhibitRules(ruleID string) ([]config.InhibitRule, error) {
	threshold, err := r.RuleCondition.Thresholds.GetRuleThreshold()
	if err != nil {
		return nil, err
	}
	var groups []string
	if r.NotificationSettings != nil {
		for k := range r.NotificationSettings.GetAlertManagerNotificationConfig().NotificationGroup {
			groups = append(groups, string(k))
		}
	}
	receivers := threshold.GetRuleReceivers()
	var inhibitRules []config.InhibitRule
	for i := 0; i < len(receivers)-1; i++ {
		rule := config.InhibitRule{
			SourceMatchers: config.Matchers{
				{
					Name:  LabelThresholdName,
					Value: receivers[i].Name,
				},
				{
					Name:  LabelRuleID,
					Value: ruleID,
				},
			},
			TargetMatchers: config.Matchers{
				{
					Name:  LabelThresholdName,
					Value: receivers[i+1].Name,
				},
				{
					Name:  LabelRuleID,
					Value: ruleID,
				},
			},
			Equal: groups,
		}
		inhibitRules = append(inhibitRules, rule)
	}
	return inhibitRules, nil
}

func (ns *NotificationSettings) UnmarshalJSON(data []byte) error {
	type Alias NotificationSettings
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(ns),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Validate states after unmarshaling
	for _, state := range ns.Renotify.AlertStates {
		if state != StateFiring && state != StateNoData {
			return o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "invalid alert state: %s", state)

		}
	}
	return nil
}

// processRuleDefaults applies the default values
// for the rule options that are blank or unset.
func (r *PostableRule) processRuleDefaults() {
	if r.SchemaVersion == "" {
		r.SchemaVersion = DefaultSchemaVersion
	}

	// v2alpha1 uses the Evaluation envelope for window/frequency;
	// only default top-level fields for v1.
	if r.SchemaVersion != SchemaVersionV2Alpha1 {
		if r.EvalWindow.IsZero() {
			r.EvalWindow = valuer.MustParseTextDuration("5m")
		}

		if r.Frequency.IsZero() {
			r.Frequency = valuer.MustParseTextDuration("1m")
		}
	}

	if r.RuleCondition != nil && r.RuleCondition.CompositeQuery != nil {
		switch r.RuleCondition.CompositeQuery.QueryType {
		case QueryTypeBuilder:
			if r.RuleType.IsZero() {
				r.RuleType = RuleTypeThreshold
			}
		case QueryTypePromQL:
			r.RuleType = RuleTypeProm
		}

		if r.SchemaVersion == DefaultSchemaVersion {
			thresholdName := CriticalThresholdName
			if r.Labels != nil {
				if severity, ok := r.Labels["severity"]; ok {
					thresholdName = severity
				}
			}

			// For anomaly detection with ValueIsBelow, negate the target
			targetValue := r.RuleCondition.Target
			if r.RuleType == RuleTypeAnomaly && r.RuleCondition.CompareOperator == ValueIsBelow && targetValue != nil {
				negated := -1 * *targetValue
				targetValue = &negated
			}

			thresholdData := RuleThresholdData{
				Kind: BasicThresholdKind,
				Spec: BasicRuleThresholds{{
					Name:            thresholdName,
					TargetUnit:      r.RuleCondition.TargetUnit,
					TargetValue:     targetValue,
					MatchType:       r.RuleCondition.MatchType,
					CompareOperator: r.RuleCondition.CompareOperator,
					Channels:        r.PreferredChannels,
				}},
			}
			r.RuleCondition.Thresholds = &thresholdData
			r.Evaluation = &EvaluationEnvelope{RollingEvaluation, RollingWindow{EvalWindow: r.EvalWindow, Frequency: r.Frequency}}
			r.NotificationSettings = &NotificationSettings{
				Renotify: Renotify{
					Enabled:          true,
					ReNotifyInterval: valuer.MustParseTextDuration("4h"),
					AlertStates:      []AlertState{StateFiring},
				},
			}
			if r.RuleCondition.AlertOnAbsent {
				r.NotificationSettings.Renotify.AlertStates = append(r.NotificationSettings.Renotify.AlertStates, StateNoData)
			}
		}
	}
}

func (r *PostableRule) MarshalJSON() ([]byte, error) {
	type Alias PostableRule

	switch r.SchemaVersion {
	case DefaultSchemaVersion:
		copyStruct := *r
		aux := Alias(copyStruct)
		if aux.RuleCondition != nil {
			aux.RuleCondition.Thresholds = nil
		}
		aux.Evaluation = nil
		aux.SchemaVersion = ""
		aux.NotificationSettings = nil
		return json.Marshal(aux)
	case SchemaVersionV2Alpha1:
		copyStruct := *r
		aux := Alias(copyStruct)
		return json.Marshal(aux)
	default:
		copyStruct := *r
		aux := Alias(copyStruct)
		return json.Marshal(aux)
	}
}

func (r *PostableRule) UnmarshalJSON(bytes []byte) error {
	type Alias PostableRule
	aux := (*Alias)(r)
	if err := json.Unmarshal(bytes, aux); err != nil {
		return o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "failed to parse json: %v", err)
	}
	r.processRuleDefaults()
	return r.validate()
}

func isValidLabelName(ln string) bool {
	if len(ln) == 0 {
		return false
	}
	for i, b := range ln {
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == '.' || (b >= '0' && b <= '9' && i > 0)) { //nolint:staticcheck // QF1001: De Morgan form is less readable here
			return false
		}
	}
	return true
}

func isValidLabelValue(v string) bool {
	return utf8.ValidString(v)
}

func isAllQueriesDisabled(compositeQuery *AlertCompositeQuery) bool {
	if compositeQuery == nil || len(compositeQuery.Queries) == 0 {
		return false
	}
	for _, query := range compositeQuery.Queries {
		var disabled bool
		switch spec := query.Spec.(type) {
		case qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]:
			disabled = spec.Disabled
		case qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]:
			disabled = spec.Disabled
		case qbtypes.QueryBuilderQuery[qbtypes.MetricAggregation]:
			disabled = spec.Disabled
		case qbtypes.PromQuery:
			disabled = spec.Disabled
		case qbtypes.DatastoreQuery:
			disabled = spec.Disabled
		default:
			continue
		}
		if !disabled {
			return false
		}
	}
	return true
}

func (r *PostableRule) validate() error {
	var errs []error

	if r.RuleCondition == nil {
		// will get panic if we try to access CompositeQuery, so return here
		return o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "rule condition is required")
	}
	if r.RuleCondition.CompositeQuery == nil {
		errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "composite query is required"))
	}

	if r.Version != "v5" {
		errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "only version v5 is supported, got %q", r.Version))
	}

	if isAllQueriesDisabled(r.RuleCondition.CompositeQuery) {
		errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "all queries are disabled in rule condition"))
	}

	for k, v := range r.Labels {
		if !isValidLabelName(k) {
			errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "invalid label name: %s", k))
		}
		if !isValidLabelValue(v) {
			errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "invalid label value: %s", v))
		}
	}

	for k := range r.Annotations {
		if !isValidLabelName(k) {
			errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "invalid annotation name: %s", k))
		}
	}

	errs = append(errs, testTemplateParsing(r)...)
	return o11yError.Join(errs...)
}

// Validate is the exported entry point for rule validation.
func (r *PostableRule) Validate() error {
	return r.validate()
}

func testTemplateParsing(rl *PostableRule) (errs []error) {
	if rl.AlertName == "" {
		// Not an alerting rule.
		return errs
	}

	// Trying to parse templates.
	tmplData := AlertTemplateData(make(map[string]string), "0", "0")
	defs := "{{$labels := .Labels}}{{$value := .Value}}{{$threshold := .Threshold}}"
	parseTest := func(text string) error {
		tmpl := NewTemplateExpander(
			context.TODO(),
			defs+text,
			"__alert_"+rl.AlertName,
			tmplData,
			nil,
		)
		return tmpl.ParseTest()
	}

	// Parsing Labels.
	for _, val := range rl.Labels {
		err := parseTest(val)
		if err != nil {
			errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "template parsing error: %s", err.Error()))
		}
	}

	// Parsing Annotations.
	for _, val := range rl.Annotations {
		err := parseTest(val)
		if err != nil {
			errs = append(errs, o11yError.NewInvalidInputf(o11yError.CodeInvalidInput, "template parsing error: %s", err.Error()))
		}
	}

	return errs
}

// GettableRules has info for all stored rules.
type GettableTestRule struct {
	AlertCount int    `json:"alertCount"`
	Message    string `json:"message"`
}

type GettableRules struct {
	Rules []*GettableRule `json:"rules"`
}

// GettableRule has info for an alerting rules.
type GettableRule struct {
	Id    string     `json:"id" required:"true"`
	State AlertState `json:"state" required:"true"`
	PostableRule
	CreatedAt time.Time `json:"createAt" required:"true"`
	CreatedBy *string   `json:"createBy" nullable:"true"`
	UpdatedAt time.Time `json:"updateAt" required:"true"`
	UpdatedBy *string   `json:"updateBy" nullable:"true"`
}

func (g *GettableRule) MarshalJSON() ([]byte, error) {
	type Alias GettableRule

	switch g.SchemaVersion {
	case DefaultSchemaVersion:
		copyStruct := *g
		aux := Alias(copyStruct)
		if aux.RuleCondition != nil {
			aux.RuleCondition.Thresholds = nil
		}
		aux.Evaluation = nil
		aux.SchemaVersion = ""
		aux.NotificationSettings = nil
		return json.Marshal(aux)
	case SchemaVersionV2Alpha1:
		copyStruct := *g
		aux := Alias(copyStruct)
		return json.Marshal(aux)
	default:
		copyStruct := *g
		aux := Alias(copyStruct)
		return json.Marshal(aux)
	}
}

// Rule is the v2 API read model for an alerting rule. It aligns audit fields
// with the canonical types.TimeAuditable / types.UserAuditable shape used by
// PlannedMaintenance and other entities. v1 handlers keep serializing
// GettableRule directly for back-compat with existing SDK / Terraform clients.
type Rule struct {
	Id    string     `json:"id" required:"true"`
	State AlertState `json:"state" required:"true"`
	PostableRule
	types.TimeAuditable
	types.UserAuditable
}

func NewRule(g *GettableRule) *Rule {
	r := &Rule{
		Id:           g.Id,
		State:        g.State,
		PostableRule: g.PostableRule,
	}
	r.CreatedAt = g.CreatedAt
	r.UpdatedAt = g.UpdatedAt
	if g.CreatedBy != nil {
		r.CreatedBy = *g.CreatedBy
	}
	if g.UpdatedBy != nil {
		r.UpdatedBy = *g.UpdatedBy
	}
	return r
}

func (r *Rule) MarshalJSON() ([]byte, error) {
	type Alias Rule

	switch r.SchemaVersion {
	case DefaultSchemaVersion:
		copyStruct := *r
		aux := Alias(copyStruct)
		if aux.RuleCondition != nil {
			aux.RuleCondition.Thresholds = nil
		}
		aux.Evaluation = nil
		aux.SchemaVersion = ""
		aux.NotificationSettings = nil
		return json.Marshal(aux)
	case SchemaVersionV2Alpha1:
		copyStruct := *r
		aux := Alias(copyStruct)
		return json.Marshal(aux)
	default:
		copyStruct := *r
		aux := Alias(copyStruct)
		return json.Marshal(aux)
	}
}
