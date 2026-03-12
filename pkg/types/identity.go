package types

import (
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Identifiable struct {
	ID valuer.UUID `json:"id" bun:"id,pk,type:text" required:"true"`
}
