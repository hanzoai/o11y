package llmobstypes

import (
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

// Annotation is a human note (optionally part of a review queue) attached to a
// trace or observation. It is the second net-new table, backing
// /v1/o11y/annotation. Langfuse-parity minimal: a note plus an optional queue
// and status.
type Annotation struct {
	bun.BaseModel `bun:"table:llm_annotations,alias:llm_annotations" json:"-"`

	types.Identifiable
	types.TimeAuditable

	OrgID         valuer.UUID `bun:"org_id,type:text,notnull" json:"-"`
	TraceID       string      `bun:"trace_id,type:text,notnull" json:"traceId" required:"true"`
	ObservationID string      `bun:"observation_id,type:text" json:"observationId,omitempty"`
	Queue         string      `bun:"queue,type:text" json:"queue,omitempty"`
	Content       string      `bun:"content,type:text,notnull" json:"content" required:"true"`
	Status        string      `bun:"status,type:text,notnull,default:'PENDING'" json:"status"`
	Author        string      `bun:"author,type:text" json:"author,omitempty"`
}

// IngestAnnotation is the create payload for POST /v1/o11y/annotation.
type IngestAnnotation struct {
	TraceID       string `json:"traceId" required:"true"`
	ObservationID string `json:"observationId,omitempty"`
	Queue         string `json:"queue,omitempty"`
	Content       string `json:"content" required:"true"`
	Status        string `json:"status,omitempty"`
}

// AnnotationsQuery is the filter for GET /v1/o11y/annotation.
type AnnotationsQuery struct {
	TraceID string `query:"traceId" json:"traceId"`
	Queue   string `query:"queue" json:"queue"`
	Status  string `query:"status" json:"status"`
	Offset  int    `query:"offset" json:"offset"`
	Limit   int    `query:"limit" json:"limit"`
}

type GettableAnnotations struct {
	Items  []*Annotation `json:"items" required:"true"`
	Total  int           `json:"total" required:"true"`
	Offset int           `json:"offset" required:"true"`
	Limit  int           `json:"limit" required:"true"`
}

func (i *IngestAnnotation) Validate() error {
	if i == nil {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeLLMObsInvalidInput, "annotation payload is null")
	}
	if i.TraceID == "" {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeLLMObsInvalidInput, "traceId is required")
	}
	if i.Content == "" {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeLLMObsInvalidInput, "content is required")
	}
	return nil
}

// NewAnnotationFromIngest builds a persistable Annotation, defaulting status.
func NewAnnotationFromIngest(i *IngestAnnotation, orgID valuer.UUID, author string, now time.Time) *Annotation {
	status := i.Status
	if status == "" {
		status = "PENDING"
	}
	return &Annotation{
		Identifiable:  types.Identifiable{ID: valuer.GenerateUUID()},
		TimeAuditable: types.TimeAuditable{CreatedAt: now, UpdatedAt: now},
		OrgID:         orgID,
		TraceID:       i.TraceID,
		ObservationID: i.ObservationID,
		Queue:         i.Queue,
		Content:       i.Content,
		Status:        status,
		Author:        author,
	}
}
