package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	ListenAddr             string
	DatabasePath           string
	OpenAIAPIKey           string
	OpenAIModel            string
	OpenAIReasoningSummary string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:             valueOrDefault("AGENT_LISTEN_ADDR", ":8080"),
		DatabasePath:           valueOrDefault("AGENT_DB_PATH", "data/agent.db"),
		OpenAIAPIKey:           strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIModel:            strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		OpenAIReasoningSummary: strings.TrimSpace(os.Getenv("OPENAI_REASONING_SUMMARY")),
	}
	if cfg.ListenAddr == "" {
		return Config{}, errors.New("AGENT_LISTEN_ADDR must not be empty")
	}
	if cfg.DatabasePath == "" {
		return Config{}, errors.New("AGENT_DB_PATH must not be empty")
	}
	if cfg.OpenAIReasoningSummary != "" && cfg.OpenAIReasoningSummary != "auto" {
		return Config{}, errors.New("OPENAI_REASONING_SUMMARY must be empty or auto")
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
