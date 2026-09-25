package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"demoagent/internal/llm"
	"demoagent/internal/session"
	"demoagent/internal/tools"
)

// Opt-in because this test uses a real, billable API call.
func TestLiveAgentToolFlow(t *testing.T) {
	if os.Getenv("AGENT_LIVE_TEST") != "1" {
		t.Skip("set AGENT_LIVE_TEST=1 to opt in to real API calls")
	}
	apiKey, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL")
	if apiKey == "" || model == "" {
		t.Fatal("OPENAI_API_KEY and OPENAI_MODEL are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store, err := session.Open(ctx, filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conversation, err := store.Create(ctx, "live-test")
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	for _, tool := range []tools.Tool{tools.Calculator{}, tools.Search{}, tools.Weather{}, tools.TodoTool{Store: store}} {
		if err := registry.Register(tool); err != nil {
			t.Fatal(err)
		}
	}
	runner := NewRunner(llm.NewResponsesClient(apiKey), store, registry, model)
	result, err := runner.Run(ctx, "live-test", conversation.ID,
		"请调用 calculator 计算 (12+8)/4，然后调用 weather 查看北京的演示天气。最后用中文简短回答，并说明天气是模拟数据。")
	if err != nil {
		t.Fatal(err)
	}
	traces, err := store.ListTraces(ctx, "live-test", conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	called := map[string]bool{}
	for _, trace := range traces {
		called[trace.ToolName] = true
	}
	if result.Answer == "" || !called["calculator"] || !called["weather"] {
		t.Fatalf("live flow incomplete: answer=%q, called=%v", result.Answer, called)
	}
	t.Logf("live flow passed: llm_calls=%d tool_calls=%d", result.LLMCalls, result.ToolCalls)
}
