package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `server:
  listen_addr: ":9090"
  database_path: test.db
  read_header_timeout_seconds: 5
  read_timeout_seconds: 10
  write_timeout_seconds: 190
  idle_timeout_seconds: 60
  shutdown_timeout_seconds: 5
llm:
  provider: deepseek
  deepseek:
    api_key: "${DEEPSEEK_API_KEY}"
    model: deepseek-flash
    endpoint: https://api.deepseek.com/responses
    request_timeout_seconds: 45
    reasoning_effort: low
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: gpt-5.4
    endpoint: https://api.openai.com/v1/responses
    request_timeout_seconds: 45
agent:
  max_llm_calls: 6
  max_tool_calls: 8
  max_message_chars: 4000
  max_recent_turns: 8
  context_char_limit: 12000
  recent_char_budget: 8000
  max_summary_chars: 2000
  max_compaction_calls: 3
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileAndSelectProvider(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test-secret")
	path := writeConfig(t, validYAML)
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ListenAddr != ":9090" || cfg.Agent.MaxLLMCalls != 6 ||
		cfg.SelectedLLM().Model != "deepseek-flash" || ResolveAPIKey(cfg.SelectedLLM().APIKey) != "test-secret" {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
	t.Setenv("AGENT_CONFIG", path)
	if loaded, err := Load(); err != nil || loaded.LLM.Provider != "deepseek" {
		t.Fatalf("Load() = %+v, %v", loaded, err)
	}
	openAI := strings.Replace(validYAML, "provider: deepseek", "provider: openai", 1)
	if cfg, err := LoadFile(writeConfig(t, openAI)); err != nil || cfg.SelectedLLM().Model != "gpt-5.4" {
		t.Fatalf("OpenAI selection = %+v, %v", cfg, err)
	}
}

func TestInvalidConfigRejected(t *testing.T) {
	cases := map[string]string{
		"unknown field":       strings.Replace(validYAML, "  provider: deepseek", "  provider: deepseek\n  typo: true", 1),
		"invalid provider":    strings.Replace(validYAML, "provider: deepseek", "provider: other", 1),
		"bad endpoint":        strings.Replace(validYAML, "https://api.deepseek.com/responses", "https://api.deepseek.com/chat/completions", 1),
		"bad request timeout": strings.Replace(validYAML, "request_timeout_seconds: 45", "request_timeout_seconds: 0", 1),
		"bad server timeout":  strings.Replace(validYAML, "read_timeout_seconds: 10", "read_timeout_seconds: 0", 1),
		"zero limit":          strings.Replace(validYAML, "max_llm_calls: 6", "max_llm_calls: 0", 1),
		"extra document":      validYAML + "---\nserver: {}\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadFile(writeConfig(t, content)); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", " secret ")
	if got := ResolveAPIKey("${DEEPSEEK_API_KEY}"); got != "secret" {
		t.Fatalf("resolved key = %q", got)
	}
	if got := ResolveAPIKey("literal-key"); got != "literal-key" {
		t.Fatalf("literal key = %q", got)
	}
}

func TestRepositoryConfigLoads(t *testing.T) {
	cfg, err := LoadFile(filepath.Join("..", "..", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Provider != "deepseek" || cfg.SelectedLLM().Endpoint == "" {
		t.Fatalf("unexpected repository config: %+v", cfg.LLM)
	}
}
