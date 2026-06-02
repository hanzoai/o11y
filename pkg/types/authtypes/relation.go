package authtypes

import (
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Relation struct {
	coretypes.Verb
}

func (Relation) Enum() []any {
	return coretypes.Verb{}.Enum()
}

func (rel *Relation) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}

	alias, err := coretypes.NewVerb(str)
	if err != nil {
		return err
	}

	*rel = Relation{alias}
	return nil
}
