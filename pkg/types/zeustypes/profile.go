package zeustypes

type PostableProfile struct {
	UsesOtel                     bool     `json:"uses_otel" required:"true"`
	HasExistingObservabilityTool bool     `json:"has_existing_observability_tool" required:"true"`
	ExistingObservabilityTool    string   `json:"existing_observability_tool" required:"true"`
	ReasonsForInterestInHanzoO11y   []string `json:"reasons_for_interest_in_o11y" required:"true"`
	LogsScalePerDayInGB          int64    `json:"logs_scale_per_day_in_gb" required:"true"`
	NumberOfServices             int64    `json:"number_of_services" required:"true"`
	NumberOfHosts                int64    `json:"number_of_hosts" required:"true"`
	WhereDidYouDiscoverHanzoO11y    string   `json:"where_did_you_discover_o11y" required:"true"`
	TimelineForMigratingToHanzoO11y string   `json:"timeline_for_migrating_to_o11y" required:"true"`
}
