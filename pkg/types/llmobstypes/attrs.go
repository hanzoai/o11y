package llmobstypes

import "github.com/hanzoai/o11y/pkg/errors"

// OTel GenAI semantic-convention attribute keys used to project O11y spans
// into LLM observations. The pricing-owned keys
// (gen_ai.request.model, gen_ai.usage.*, _o11y.gen_ai.total_cost) live in
// llmpricingruletypes and are referenced from there to stay DRY; only the
// keys unique to the observability views are declared here.
const (
	// GenAISystem is the canonical marker that a span is an LLM call.
	GenAISystem = "gen_ai.system"
	// GenAIOperationName distinguishes chat / embeddings / tool observations.
	GenAIOperationName = "gen_ai.operation.name"
	// GenAIResponseModel is the model the provider actually served.
	GenAIResponseModel = "gen_ai.response.model"

	// SessionID and UserID are the standard OTel keys observations group on.
	SessionID = "session.id"
	UserID    = "user.id"

	// ServiceName is the resource attribute identifying the emitting app.
	ServiceName = "service.name"

	// GenAIHanzoOrgID is the TENANT slug the ai emit path stamps on every gen_ai
	// span (= the JWT `owner` / X-Org-Id slug). It is the ONLY org discriminator on
	// the span-view telemetry, so every observations/traces/sessions/users query
	// MUST AND an equality on it against the caller's validated org — otherwise a
	// tenant reads every other tenant's spans.
	GenAIHanzoOrgID = "gen_ai.hanzo.org_id"
)

var (
	ErrCodeLLMObsInvalidInput = errors.MustNewCode("llmobs_invalid_input")
	ErrCodeLLMObsNotFound     = errors.MustNewCode("llmobs_not_found")
)
