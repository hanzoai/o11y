package implsentry

import (
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// mintDSN builds the DSN an operator hands to an app to report into a project. The
// key is the reused errortracking derivation keyed on the PROJECT id + version; the
// endpoint is the CLEAN Hanzo route — no /api/ segment:
//
//	https://<version>:<hmac>@<host>/v1/event/<projectID>
//
// THE INGEST DOOR IS /v1/event, and it is the same door the product event stream
// uses. @hanzo/event derives its ingest URL from the DSN's OWN path — base plus
// /envelope/ — so this line decides where every reporting client posts, and it is
// the one place the server states it.
//
// The product FACE — issues, projects, stats, traces, the reads a console makes —
// is /v1/o11y/sentinel. Ingest and face are different nouns and do not share a root:
// ingest is keyed by a DSN and answers a beacon, the face is keyed by a session
// and answers a person.
func mintDSN(secret []byte, host string, projectID valuer.UUID, version int) string {
	key := implerrortracking.PublicKeyForVersion(secret, projectID.String(), version)
	return "https://" + key + "@" + host + "/v1/event/" + projectID.String()
}
