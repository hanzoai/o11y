package constants

import (
	"os"
)

var LicenseHanzoAi = "https://license.observe.hanzo.ai/api/v1"
var LicenseAPIKey = GetOrDefaultEnv("HANZO_LICENSE_API_KEY", "")
var SaasSegmentKey = GetOrDefaultEnv("HANZO_SAAS_SEGMENT_KEY", "")
var FetchFeatures = GetOrDefaultEnv("FETCH_FEATURES", "false")
var ZeusFeaturesURL = GetOrDefaultEnv("ZEUS_FEATURES_URL", "ZeusFeaturesURL")

func GetOrDefaultEnv(key string, fallback string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return fallback
	}
	return v
}

// constant functions that override env vars

const DotMetricsEnabled = "DOT_METRICS_ENABLED"

var IsDotMetricsEnabled = false

func init() {
	if GetOrDefaultEnv(DotMetricsEnabled, "true") == "true" {
		IsDotMetricsEnabled = true
	}
}
