//go:build !grpc

// OPAmp config-push default path.
//
// OPAmp (Open Agent Management Protocol) is fundamentally an OTLP/gRPC
// remote-agent control protocol — pushing processor config to remote
// collectors. The full implementation lives in configure_ingestionRules.go
// (//go:build grpc) and pulls go.opentelemetry.io/collector/confmap, which
// transitively requires google.golang.org/grpc.
//
// Default builds use ZAP-native ingestion via pkg/zapreceiver instead — no
// OPAmp needed because Hanzo services ship spans directly over the ZAP
// envelope wire to the o11y receiver. Operators who run an external OTLP
// fleet and need OPAmp remote-agent control rebuild with -tags grpc.

package opamp

import (
	"context"
	"errors"

	"github.com/hanzoai/o11y/pkg/query-service/app/opamp/model"
)

// errOpAmpUnavailable signals that OPAmp remote-agent control is not
// part of this build. Surfaced to callers in agentConf so the UI/API
// can render a clear "rebuild with -tags grpc" message.
var errOpAmpUnavailable = errors.New("opamp: ZAP-native ingestion is the default; rebuild with -tags grpc to enable OTLP/OPAmp remote-agent control")

// UpsertControlProcessors is the default no-op for ZAP-native builds.
// Returns an empty config hash and errOpAmpUnavailable. The grpc-tagged
// build replaces this with the real collector/confmap-backed
// implementation in configure_ingestionRules.go.
func UpsertControlProcessors(_ context.Context, _ string, _ map[string]interface{}, _ model.OnChangeCallback) (string, error) {
	return "", errOpAmpUnavailable
}
