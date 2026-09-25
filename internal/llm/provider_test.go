package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"demoagent/internal/config"
)

func TestConfiguredClientUsesSelectedProvider(t *testing.T) {
	deepSeekCalls, openAICalls := 0, 0
	deepSeek := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deepSeekCalls++
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer deepseek-key" {
			t.Errorf("unexpected DeepSeek request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"ds","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer deepSeek.Close()
	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openAICalls++
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer openai-key" {
			t.Errorf("unexpected OpenAI request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"oa","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer openAI.Close()
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	cfg := config.Config{LLM: config.LLMConfig{
		Provider: "deepseek",
		DeepSeek: config.ProviderConfig{APIKey: "${DEEPSEEK_API_KEY}", Model: "deepseek-flash", Endpoint: deepSeek.URL + "/responses", RequestTimeoutSeconds: 45},
		OpenAI:   config.ProviderConfig{APIKey: "${OPENAI_API_KEY}", Model: "test-model", Endpoint: openAI.URL + "/responses", RequestTimeoutSeconds: 45},
	}}
	request := Request{Model: cfg.SelectedLLM().Model, Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}}
	if _, err := NewConfiguredClient(cfg).CreateResponse(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	cfg.LLM.Provider = "openai"
	request.Model = cfg.SelectedLLM().Model
	if _, err := NewConfiguredClient(cfg).CreateResponse(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if deepSeekCalls != 1 || openAICalls != 1 {
		t.Fatalf("provider calls: DeepSeek=%d OpenAI=%d", deepSeekCalls, openAICalls)
	}
}
