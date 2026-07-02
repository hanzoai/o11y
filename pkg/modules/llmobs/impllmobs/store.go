package impllmobs

import (
	"context"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type store struct {
	sqlstore sqlstore.SQLStore
}

func NewStore(sqlstore sqlstore.SQLStore) llmobstypes.Store {
	return &store{sqlstore: sqlstore}
}

func (s *store) CreateScore(ctx context.Context, score *llmobstypes.Score) error {
	_, err := s.sqlstore.BunDBCtx(ctx).NewInsert().Model(score).Exec(ctx)
	return err
}

func (s *store) GetScore(ctx context.Context, orgID, id valuer.UUID) (*llmobstypes.Score, error) {
	score := new(llmobstypes.Score)
	err := s.sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(score).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, s.sqlstore.WrapNotFoundErrf(err, llmobstypes.ErrCodeLLMObsNotFound, "score %s not found in the org", id)
	}
	return score, nil
}

func (s *store) ListScores(ctx context.Context, orgID valuer.UUID, q *llmobstypes.ScoresQuery) ([]*llmobstypes.Score, int, error) {
	scores := make([]*llmobstypes.Score, 0)

	query := s.sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(&scores).
		Where("org_id = ?", orgID)

	if q.TraceID != "" {
		query = query.Where("trace_id = ?", q.TraceID)
	}
	if q.ObservationID != "" {
		query = query.Where("observation_id = ?", q.ObservationID)
	}
	if q.Name != "" {
		query = query.Where("name = ?", q.Name)
	}
	if q.Source != "" {
		query = query.Where("source = ?", q.Source)
	}

	count, err := query.
		Order("timestamp DESC").
		Offset(clampOffset(q.Offset)).
		Limit(clampLimit(q.Limit)).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	return scores, count, nil
}

func (s *store) DeleteScore(ctx context.Context, orgID, id valuer.UUID) error {
	res, err := s.sqlstore.
		BunDBCtx(ctx).
		NewDelete().
		Model((*llmobstypes.Score)(nil)).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.Newf(errors.TypeNotFound, llmobstypes.ErrCodeLLMObsNotFound, "score %s not found in the org", id)
	}
	return nil
}

func (s *store) CreateAnnotation(ctx context.Context, annotation *llmobstypes.Annotation) error {
	_, err := s.sqlstore.BunDBCtx(ctx).NewInsert().Model(annotation).Exec(ctx)
	return err
}

func (s *store) ListAnnotations(ctx context.Context, orgID valuer.UUID, q *llmobstypes.AnnotationsQuery) ([]*llmobstypes.Annotation, int, error) {
	annotations := make([]*llmobstypes.Annotation, 0)

	query := s.sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(&annotations).
		Where("org_id = ?", orgID)

	if q.TraceID != "" {
		query = query.Where("trace_id = ?", q.TraceID)
	}
	if q.Queue != "" {
		query = query.Where("queue = ?", q.Queue)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}

	count, err := query.
		Order("created_at DESC").
		Offset(clampOffset(q.Offset)).
		Limit(clampLimit(q.Limit)).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	return annotations, count, nil
}
