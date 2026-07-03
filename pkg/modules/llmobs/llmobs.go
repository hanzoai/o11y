package llmobs

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Module is the native LLM-observability surface that absorbs the upstream product:
// observations/traces/sessions/users are views over gen_ai spans, while
// scores and annotations are CRUD over two net-new tables.
type Module interface {
	ListObservations(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.Observation, error)
	ListTraces(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.Trace, error)
	ListSessions(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.Session, error)
	ListUsers(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.User, error)

	ListScores(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ScoresQuery) ([]*llmobstypes.Score, int, error)
	CreateScore(ctx context.Context, orgID valuer.UUID, author string, in *llmobstypes.IngestScore) (*llmobstypes.Score, error)
	GetScore(ctx context.Context, orgID, id valuer.UUID) (*llmobstypes.Score, error)
	DeleteScore(ctx context.Context, orgID, id valuer.UUID) error

	ListAnnotations(ctx context.Context, orgID valuer.UUID, q *llmobstypes.AnnotationsQuery) ([]*llmobstypes.Annotation, int, error)
	CreateAnnotation(ctx context.Context, orgID valuer.UUID, author string, in *llmobstypes.IngestAnnotation) (*llmobstypes.Annotation, error)
}

// Handler is the HTTP surface for the routes under /v1/o11y.
type Handler interface {
	Observations(rw http.ResponseWriter, r *http.Request)
	Traces(rw http.ResponseWriter, r *http.Request)
	Sessions(rw http.ResponseWriter, r *http.Request)
	Users(rw http.ResponseWriter, r *http.Request)

	ListScores(rw http.ResponseWriter, r *http.Request)
	CreateScore(rw http.ResponseWriter, r *http.Request)
	GetScore(rw http.ResponseWriter, r *http.Request)
	DeleteScore(rw http.ResponseWriter, r *http.Request)

	Annotations(rw http.ResponseWriter, r *http.Request)
	CreateAnnotation(rw http.ResponseWriter, r *http.Request)
}
