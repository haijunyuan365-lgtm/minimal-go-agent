package config

import "testing"

func TestLoadDefaultsAndValidation(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("AGENT_LISTEN_ADDR", ":9090")
	t.Setenv("AGENT_DB_PATH", "test.db")
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("OPENAI_MODEL", "example-model")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":9090" || cfg.DatabasePath != "test.db" || cfg.OpenAIAPIKey != "secret" || cfg.OpenAIModel != "example-model" {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	t.Setenv("AGENT_DB_PATH", " ")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an empty database path")
	}
}

func TestLoadDeepSeek(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "deepseek")
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-secret")
	t.Setenv("DEEPSEEK_MODEL", "deepseek-v4-pro")
	t.Setenv("DEEPSEEK_ENDPOINT", "https://api.deepseek.com/responses")
	t.Setenv("DEEPSEEK_REASONING_EFFORT", "low")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMProvider != "deepseek" || cfg.DeepSeekAPIKey != "deepseek-secret" || cfg.DeepSeekModel != "deepseek-v4-pro" || cfg.DeepSeekReasoningEffort != "low" {
		t.Fatalf("unexpected DeepSeek config: %+v", cfg)
	}
	t.Setenv("DEEPSEEK_ENDPOINT", "https://api.deepseek.com/chat/completions")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid DeepSeek endpoint")
	}
	t.Setenv("DEEPSEEK_ENDPOINT", "")
	t.Setenv("DEEPSEEK_REASONING_EFFORT", "auto")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid reasoning effort")
	}
	t.Setenv("DEEPSEEK_REASONING_EFFORT", "")
	t.Setenv("LLM_PROVIDER", "other")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid provider")
	}
}
