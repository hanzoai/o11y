package llmobstypes

import (
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

// Score is an eval score or human-feedback signal attached to a trace or a
// single observation. It is the one net-new table backing /v1/o11y/scores.
type Score struct {
	bun.BaseModel `bun:"table:llm_scores,alias:llm_scores" json:"-"`

	types.Identifiable
	types.TimeAuditable

	OrgID         valuer.UUID `bun:"org_id,type:text,notnull" json:"-"`
	TraceID       string      `bun:"trace_id,type:text,notnull" json:"traceId" required:"true"`
	ObservationID string      `bun:"observation_id,type:text" json:"observationId,omitempty"`
	Name          string      `bun:"name,type:text,notnull" json:"name" required:"true"`
	Value         float64     `bun:"value,type:numeric,notnull,default:0" json:"value"`
	StringValue   string      `bun:"string_value,type:text" json:"stringValue,omitempty"`
	DataType      string      `bun:"data_type,type:text,notnull,default:'NUMERIC'" json:"dataType"`
	Comment       string      `bun:"comment,type:text" json:"comment,omitempty"`
	Source        string      `bun:"source,type:text,notnull,default:'API'" json:"source"`
	Timestamp     time.Time   `bun:"timestamp,notnull" json:"timestamp"`
	CreatedBy     string      `bun:"created_by,type:text" json:"createdBy,omitempty"`
}

// IngestScore is the create payload for POST /v1/o11y/scores.
type IngestScore struct {
	TraceID       string  `json:"traceId" required:"true"`
	ObservationID string  `json:"observationId,omitempty"`
	Name          string  `json:"name" required:"true"`
	Value         float64 `json:"value"`
	StringValue   string  `json:"stringValue,omitempty"`
	DataType      string  `json:"dataType,omitempty"`
	Comment       string  `json:"comment,omitempty"`
	Source        string  `json:"source,omitempty"`
}

// ScoresQuery is the filter for GET /v1/o11y/scores.
type ScoresQuery struct {
	TraceID       string `query:"traceId" json:"traceId"`
	ObservationID string `query:"observationId" json:"observationId"`
	Name          string `query:"name" json:"name"`
	Source        string `query:"source" json:"source"`
	Offset        int    `query:"offset" json:"offset"`
	Limit         int    `query:"limit" json:"limit"`
}

type GettableScores struct {
	Items  []*Score `json:"items" required:"true"`
	Total  int      `json:"total" required:"true"`
	Offset int      `json:"offset" required:"true"`
	Limit  int      `json:"limit" required:"true"`
}

// Validate enforces the minimal invariants of a score ingest.
func (i *IngestScore) Validate() error {
	if i == nil {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeLLMObsInvalidInput, "score payload is null")
	}
	if i.TraceID == "" {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeLLMObsInvalidInput, "traceId is required")
	}
	if i.Name == "" {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeLLMObsInvalidInput, "name is required")
	}
	return nil
}

// NewScoreFromIngest builds a persistable Score, defaulting dataType and source.
func NewScoreFromIngest(i *IngestScore, orgID valuer.UUID, author string, now time.Time) *Score {
	dataType := i.DataType
	if dataType == "" {
		if i.StringValue != "" {
			dataType = "CATEGORICAL"
		} else {
			dataType = "NUMERIC"
		}
	}
	source := i.Source
	if source == "" {
		source = "API"
	}
	return &Score{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		TraceID:       i.TraceID,
		ObservationID: i.ObservationID,
		Name:          i.Name,
		Value:         i.Value,
		StringValue:   i.StringValue,
		DataType:      dataType,
		Comment:       i.Comment,
		Source:        source,
		Timestamp:     now,
		CreatedBy:     author,
	}
}
