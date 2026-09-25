package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	ListenAddr              string
	DatabasePath            string
	LLMProvider             string
	OpenAIAPIKey            string
	OpenAIModel             string
	OpenAIReasoningSummary  string
	DeepSeekAPIKey          string
	DeepSeekModel           string
	DeepSeekEndpoint        string
	DeepSeekReasoningEffort string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:              valueOrDefault("AGENT_LISTEN_ADDR", ":8080"),
		DatabasePath:            valueOrDefault("AGENT_DB_PATH", "data/agent.db"),
		LLMProvider:             valueOrDefault("LLM_PROVIDER", "openai"),
		OpenAIAPIKey:            strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIModel:             strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		OpenAIReasoningSummary:  strings.TrimSpace(os.Getenv("OPENAI_REASONING_SUMMARY")),
		DeepSeekAPIKey:          strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")),
		DeepSeekModel:           valueOrDefault("DEEPSEEK_MODEL", "deepseek-flash"),
		DeepSeekEndpoint:        strings.TrimSpace(os.Getenv("DEEPSEEK_ENDPOINT")),
		DeepSeekReasoningEffort: strings.TrimSpace(os.Getenv("DEEPSEEK_REASONING_EFFORT")),
	}
	if cfg.ListenAddr == "" {
		return Config{}, errors.New("AGENT_LISTEN_ADDR must not be empty")
	}
	if cfg.DatabasePath == "" {
		return Config{}, errors.New("AGENT_DB_PATH must not be empty")
	}
	if cfg.LLMProvider != "openai" && cfg.LLMProvider != "deepseek" {
		return Config{}, errors.New("LLM_PROVIDER must be openai or deepseek")
	}
	if cfg.LLMProvider == "openai" {
		if cfg.OpenAIReasoningSummary != "" && cfg.OpenAIReasoningSummary != "auto" {
			return Config{}, errors.New("OPENAI_REASONING_SUMMARY must be empty or auto")
		}
		return cfg, nil
	}
	if cfg.DeepSeekReasoningEffort != "" && cfg.DeepSeekReasoningEffort != "none" && cfg.DeepSeekReasoningEffort != "low" && cfg.DeepSeekReasoningEffort != "high" && cfg.DeepSeekReasoningEffort != "max" {
		return Config{}, errors.New("DEEPSEEK_REASONING_EFFORT must be none, low, high, or max")
	}
	if cfg.DeepSeekEndpoint != "" {
		parsed, err := url.Parse(cfg.DeepSeekEndpoint)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, errors.New("DEEPSEEK_ENDPOINT must be an absolute HTTP(S) URL ending in /responses")
		}
		if !strings.HasSuffix(parsed.Path, "/responses") {
			return Config{}, errors.New("DEEPSEEK_ENDPOINT must end in /responses")
		}
	}
	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return strings.TrimSpace(value)
}
