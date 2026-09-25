package config

import "testing"

func TestLoadDefaultsAndValidation(t *testing.T) {
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
