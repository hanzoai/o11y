package implerrortracking

import (
	"context"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// OccurrenceSink is the OPTIONAL bridge that would also persist each occurrence to
// the shared telemetry store (o11y_logs as an ERROR-severity record) for OTel-native
// drill-down. It is deliberately a seam, and the default is NoopSink:
//
//   - Authoritative storage is o11y_issues — the grouped issue plus the latest
//     occurrence sample. Error capture is fully viewable with the no-op sink; the
//     sink is enrichment, not the source of truth.
//   - When a real sink is wired, its write is fail-soft at the call site: a
//     telemetry-store hiccup must never drop the durable issue upsert.
//
// NOTE (honest status): the ClickHouse logs sink is NOT implemented in this build —
// only NoopSink exists. A raw logs_v2 INSERT couples to resource-fingerprint /
// ts-bucket schema that must be byte-verified against a LIVE datastore, not
// reconstructed, so it is a deliberate fast-follow. Today occurrences are NOT
// written to the telemetry store; the count and the latest sample live on the
// issue row.
type OccurrenceSink interface {
	Write(ctx context.Context, orgID valuer.UUID, occ *errortrackingtypes.Occurrence) error
}

// NoopSink discards occurrences. It is a complete, correct sink for the MVP: the
// authoritative issue (with its latest sample) is persisted independently.
type NoopSink struct{}

func NewNoopSink() OccurrenceSink { return NoopSink{} }

func (NoopSink) Write(context.Context, valuer.UUID, *errortrackingtypes.Occurrence) error {
	return nil
}
