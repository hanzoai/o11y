package ruletypes

import "github.com/hanzoai/o11y/pkg/valuer"

type RuleHealth struct {
	valuer.String
}

var (
	HealthUnknown = RuleHealth{valuer.NewString("unknown")}
	HealthGood    = RuleHealth{valuer.NewString("ok")}
	HealthBad     = RuleHealth{valuer.NewString("err")}
)
