package llmobstypes

import (
	"context"

	"github.com/hanzoai/o11y/pkg/valuer"
)

// Store persists the two net-new tables (scores, annotations). The span views
// have no store — they are computed through the querier.
type Store interface {
	CreateScore(ctx context.Context, score *Score) error
	GetScore(ctx context.Context, orgID, id valuer.UUID) (*Score, error)
	ListScores(ctx context.Context, orgID valuer.UUID, q *ScoresQuery) ([]*Score, int, error)
	DeleteScore(ctx context.Context, orgID, id valuer.UUID) error

	CreateAnnotation(ctx context.Context, annotation *Annotation) error
	ListAnnotations(ctx context.Context, orgID valuer.UUID, q *AnnotationsQuery) ([]*Annotation, int, error)
}
