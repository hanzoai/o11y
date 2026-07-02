package llmobstypes

import (
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/valuer"
)

func TestNewScoreFromIngest(t *testing.T) {
	org := valuer.GenerateUUID()
	now := time.Unix(1700000000, 0)

	// numeric score: dataType and source default
	s := NewScoreFromIngest(&IngestScore{TraceID: "t1", Name: "quality", Value: 0.9}, org, "z@hanzo.ai", now)
	if s.DataType != "NUMERIC" {
		t.Errorf("dataType = %q, want NUMERIC", s.DataType)
	}
	if s.Source != "API" {
		t.Errorf("source = %q, want API", s.Source)
	}
	if s.OrgID != org || s.CreatedBy != "z@hanzo.ai" || !s.Timestamp.Equal(now) || s.ID.IsZero() {
		t.Errorf("score envelope wrong: %+v", s)
	}

	// string value without explicit dataType is inferred CATEGORICAL
	s = NewScoreFromIngest(&IngestScore{TraceID: "t1", Name: "label", StringValue: "good"}, org, "z@hanzo.ai", now)
	if s.DataType != "CATEGORICAL" {
		t.Errorf("dataType = %q, want CATEGORICAL", s.DataType)
	}

	// explicit dataType/source are preserved
	s = NewScoreFromIngest(&IngestScore{TraceID: "t1", Name: "ok", DataType: "BOOLEAN", Source: "EVAL"}, org, "z@hanzo.ai", now)
	if s.DataType != "BOOLEAN" || s.Source != "EVAL" {
		t.Errorf("explicit dataType/source not preserved: %+v", s)
	}
}

func TestIngestScoreValidate(t *testing.T) {
	if err := (&IngestScore{Name: "x"}).Validate(); err == nil {
		t.Error("missing traceId should fail")
	}
	if err := (&IngestScore{TraceID: "t1"}).Validate(); err == nil {
		t.Error("missing name should fail")
	}
	if err := (&IngestScore{TraceID: "t1", Name: "x"}).Validate(); err != nil {
		t.Errorf("valid score rejected: %v", err)
	}
	if err := (*IngestScore)(nil).Validate(); err == nil {
		t.Error("nil score should fail")
	}
}

func TestNewAnnotationFromIngest(t *testing.T) {
	org := valuer.GenerateUUID()
	now := time.Unix(1700000000, 0)

	a := NewAnnotationFromIngest(&IngestAnnotation{TraceID: "t1", Content: "looks off"}, org, "z@hanzo.ai", now)
	if a.Status != "PENDING" {
		t.Errorf("status = %q, want PENDING", a.Status)
	}
	if a.OrgID != org || a.Author != "z@hanzo.ai" || a.ID.IsZero() {
		t.Errorf("annotation envelope wrong: %+v", a)
	}

	a = NewAnnotationFromIngest(&IngestAnnotation{TraceID: "t1", Content: "done", Status: "COMPLETED", Queue: "review"}, org, "z@hanzo.ai", now)
	if a.Status != "COMPLETED" || a.Queue != "review" {
		t.Errorf("explicit status/queue not preserved: %+v", a)
	}
}

func TestIngestAnnotationValidate(t *testing.T) {
	if err := (&IngestAnnotation{Content: "x"}).Validate(); err == nil {
		t.Error("missing traceId should fail")
	}
	if err := (&IngestAnnotation{TraceID: "t1"}).Validate(); err == nil {
		t.Error("missing content should fail")
	}
	if err := (&IngestAnnotation{TraceID: "t1", Content: "x"}).Validate(); err != nil {
		t.Errorf("valid annotation rejected: %v", err)
	}
}
