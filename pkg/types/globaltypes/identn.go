package globaltypes

// IdentNConfig is the part of GET /v1/o11y/global/config the console reads to
// decide how identity reaches it. The `tokenizer` member is gone with the
// tokenizer: o11y minted its own access and refresh tokens, and it does not.
type IdentNConfig struct {
	APIKey        APIKeyConfig        `json:"apikey"`
	Impersonation ImpersonationConfig `json:"impersonation"`
}

type APIKeyConfig struct {
	Enabled bool `json:"enabled"`
}

type ImpersonationConfig struct {
	Enabled bool `json:"enabled"`
}

func NewIdentNConfig(apiKey APIKeyConfig, impersonation ImpersonationConfig) IdentNConfig {
	return IdentNConfig{
		APIKey:        apiKey,
		Impersonation: impersonation,
	}
}
