//go:build !grpc

// UpsertControlProcessors is the OPAmp ingestion-rules update path,
// used to push processor config to remote agents. The real
// implementation pulls go.opentelemetry.io/collector/confmap which
// transitively requires google.golang.org/grpc — that path is gated
// behind -tags grpc.
//
// Default builds expose a no-op that returns "" with no error so
// callers in pkg/query-service/agentConf compile without dragging the
// collector framework into the dep graph.

package opamp

import (
	"context"
	"errors"

	"github.com/hanzoai/o11y/pkg/query-service/app/opamp/model"
)

// errOpAmpDisabled signals that OPAmp remote-agent control is not
// available in this build. Wired in pkg/query-service/agentConf via
// the OnChangeCallback contract.
var errOpAmpDisabled = errors.New("opamp: ingestion-rules push disabled (rebuild with -tags grpc to enable)")

// UpsertControlProcessors is a no-op for grpc-untagged builds.
// Returns an empty config hash and the disabled-sentinel error so the
// caller can decide whether to surface it.
func UpsertControlProcessors(_ context.Context, _ string, _ map[string]interface{}, _ model.OnChangeCallback) (string, error) {
	return "", errOpAmpDisabled
}
