package web

// Settings is what the SPA reads at boot.
//
// Hanzo o11y does NOT ship third-party trackers. The upstream fork wired in
// product analytics, onboarding tours and a support-chat widget (all enabled by
// default in the frontend build), which meant a self-hosted observability tool
// phoned home to third parties with our users' data. They are removed: analytics
// is Hanzo Insights, support chat is Hanzo Chat. Do not reintroduce them.
//
// Sentry stays because we run our own fork (hanzoai/sentry) and it is opt-in —
// it only activates when an operator sets a DSN pointing at our instance.
type Settings struct {
	Sentry Sentry `json:"sentry" required:"true"`
}

type Sentry struct {
	Enabled bool   `json:"enabled" required:"true"`
	DSN     string `json:"dsn"`
	Tunnel  string `json:"tunnel"`
}

func NewSettings(config Config) Settings {
	return Settings{
		Sentry: Sentry{
			Enabled: config.Settings.Sentry.Enabled,
			DSN:     config.Settings.Sentry.DSN,
			Tunnel:  config.Settings.Sentry.Tunnel,
		},
	}
}
