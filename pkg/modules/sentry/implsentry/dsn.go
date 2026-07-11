package implsentry

import (
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// mintDSN builds the DSN an operator hands to an app to report into a project. The
// key is the reused errortracking derivation keyed on the PROJECT id + version; the
// endpoint is the CLEAN Hanzo route — no /api/ segment:
//
//	https://<version>:<hmac>@<host>/v1/sentry/<projectID>
//
// A Sentry SDK given this DSN derives its ingest URL as
// https://<host>/v1/sentry/<projectID>/envelope/ — exactly the route addSentryRoutes
// registers. (Official-SDK builds that hardcode an extra /api/ segment are an
// edge-rewrite compat concern for the gateway, never baked in here.)
func mintDSN(secret []byte, host string, projectID valuer.UUID, version int) string {
	key := implerrortracking.PublicKeyForVersion(secret, projectID.String(), version)
	return "https://" + key + "@" + host + "/v1/sentry/" + projectID.String()
}
