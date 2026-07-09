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
)

var (
	ErrCodeLLMObsInvalidInput = errors.MustNewCode("llmobs_invalid_input")
	ErrCodeLLMObsNotFound     = errors.MustNewCode("llmobs_not_found")
)
