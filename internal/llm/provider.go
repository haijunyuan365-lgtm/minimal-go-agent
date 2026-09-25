package llm

import (
	"time"

	"demoagent/internal/config"
)

// NewConfiguredClient creates the selected provider adapter from config.yml.
// A missing API key leaves chat unavailable while the HTTP service stays usable.
func NewConfiguredClient(cfg config.Config) Client {
	selected := cfg.SelectedLLM()
	apiKey := config.ResolveAPIKey(selected.APIKey)
	if apiKey == "" {
		return nil
	}
	if cfg.LLM.Provider == "deepseek" {
		return NewDeepSeekClient(apiKey, selected.Endpoint, time.Duration(selected.RequestTimeoutSeconds)*time.Second)
	}
	return NewResponsesClient(apiKey, selected.Endpoint, time.Duration(selected.RequestTimeoutSeconds)*time.Second)
}
