package preferencetypes

import "github.com/hanzoai/o11y/pkg/valuer"

var (
	ScopeOrg  = Scope{valuer.NewString("org")}
	ScopeUser = Scope{valuer.NewString("user")}
)

type Scope struct{ valuer.String }
