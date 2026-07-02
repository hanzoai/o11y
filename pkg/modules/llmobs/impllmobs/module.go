package impllmobs

import (
	"context"
	"time"

	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/types/ctxtypes"
	"github.com/hanzoai/o11y/pkg/types/instrumentationtypes"
	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type module struct {
	querier querier.Querier
	store   llmobstypes.Store
}

// NewModule wires the span-view querier and the scores/annotations store into
// the single LLM-observability module.
func NewModule(q querier.Querier, store llmobstypes.Store) llmobs.Module {
	return &module{querier: q, store: store}
}

func (m *module) ListObservations(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.Observation, error) {
	resp, err := m.query(ctx, orgID, "ListObservations", buildObservationsQuery(q))
	if err != nil {
		return nil, err
	}
	return mapObservations(resp), nil
}

func (m *module) ListTraces(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.Trace, error) {
	resp, err := m.query(ctx, orgID, "ListTraces", buildTracesQuery(q))
	if err != nil {
		return nil, err
	}
	return mapTraces(resp), nil
}

func (m *module) ListSessions(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.Session, error) {
	resp, err := m.query(ctx, orgID, "ListSessions", buildSessionsQuery(q))
	if err != nil {
		return nil, err
	}
	return mapSessions(resp), nil
}

func (m *module) ListUsers(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ViewQuery) ([]*llmobstypes.User, error) {
	resp, err := m.query(ctx, orgID, "ListUsers", buildUsersQuery(q))
	if err != nil {
		return nil, err
	}
	return mapUsers(resp), nil
}

func (m *module) ListScores(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ScoresQuery) ([]*llmobstypes.Score, int, error) {
	return m.store.ListScores(ctx, orgID, q)
}

func (m *module) CreateScore(ctx context.Context, orgID valuer.UUID, author string, in *llmobstypes.IngestScore) (*llmobstypes.Score, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	score := llmobstypes.NewScoreFromIngest(in, orgID, author, time.Now())
	if err := m.store.CreateScore(ctx, score); err != nil {
		return nil, err
	}
	return score, nil
}

func (m *module) GetScore(ctx context.Context, orgID, id valuer.UUID) (*llmobstypes.Score, error) {
	return m.store.GetScore(ctx, orgID, id)
}

func (m *module) DeleteScore(ctx context.Context, orgID, id valuer.UUID) error {
	return m.store.DeleteScore(ctx, orgID, id)
}

func (m *module) ListAnnotations(ctx context.Context, orgID valuer.UUID, q *llmobstypes.AnnotationsQuery) ([]*llmobstypes.Annotation, int, error) {
	return m.store.ListAnnotations(ctx, orgID, q)
}

func (m *module) CreateAnnotation(ctx context.Context, orgID valuer.UUID, author string, in *llmobstypes.IngestAnnotation) (*llmobstypes.Annotation, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	annotation := llmobstypes.NewAnnotationFromIngest(in, orgID, author, time.Now())
	if err := m.store.CreateAnnotation(ctx, annotation); err != nil {
		return nil, err
	}
	return annotation, nil
}

// query tags the context for instrumentation and runs the composed span-view
// request through the shared querier.
func (m *module) query(ctx context.Context, orgID valuer.UUID, fn string, req *qbtypes.QueryRangeRequest) (*qbtypes.QueryRangeResponse, error) {
	ctx = ctxtypes.NewContextWithCommentVals(ctx, map[string]string{
		instrumentationtypes.TelemetrySignal:  telemetrytypes.SignalTraces.StringValue(),
		instrumentationtypes.CodeNamespace:    "llmobs",
		instrumentationtypes.CodeFunctionName: fn,
	})
	return m.querier.QueryRange(ctx, orgID, req)
}
