package implerrortracking

import (
	"context"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// OccurrenceSink persists a normalized occurrence to the REUSED telemetry store
// (o11y_logs as an ERROR-severity record) so OTel-native drill-down and the
// unified plane see the same events. It is deliberately a seam:
//
//   - The issue list/detail never depends on it — o11y_issues carries the latest
//     sample, so error capture is fully viewable with the no-op sink.
//   - The write is best-effort at the call site (fail-soft): a telemetry-store
//     hiccup must never drop the authoritative issue upsert.
//
// The default is NoopSink. The ClickHouse logs sink is enabled explicitly once its
// insert is byte-verified against a live datastore (a raw logs_v2 INSERT couples
// to resource-fingerprint/ts-bucket schema that must be confirmed live, not
// reconstructed) — see chsink.go.
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
