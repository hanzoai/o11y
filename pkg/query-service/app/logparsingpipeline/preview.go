// Log pipeline preview is a signoz-inherited feature backed by the
// signoz-otel-collector simulator, which pulls google.golang.org/grpc
// via the OTel collector framework. Default builds ship a noop that
// returns the input logs unchanged + a warning that preview requires
// -tags signoz.
//
// O11yLogsToPLogs and PLogsToO11yLogs (plain pdata conversions, no grpc
// pull) stay in preview.go and are excluded from this noop build.

package logparsingpipeline

import (
	"context"

	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/hanzoai/o11y/pkg/types/pipelinetypes"
)

// SimulatePipelinesProcessing is a no-op in default builds. Returns the
// input logs untouched, plus a single warning string explaining why no
// transformation happened. Build with -tags signoz to enable the real
// collector-based simulator.
func SimulatePipelinesProcessing(_ context.Context, _ []pipelinetypes.GettablePipeline, logs []model.O11yLog) ([]model.O11yLog, []string, error) {
	return logs, []string{
		"log pipeline preview disabled (signoz-otel-collector pulls google.golang.org/grpc — rebuild with -tags signoz to enable)",
	}, nil
}
