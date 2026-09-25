package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"demoagent/internal/llm"
	"demoagent/internal/session"
	"demoagent/internal/tools"
)

func TestDeepSeekToolLoopAndFollowUp(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		var payload struct {
			Model string            `json:"model"`
			Input []json.RawMessage `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload.Model != "deepseek-flash" {
			t.Errorf("unexpected model: %q", payload.Model)
		}
		switch step {
		case 1:
			_, _ = w.Write([]byte(`{"id":"ds_1","status":"completed","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"Use calculator"}]},{"type":"function_call","call_id":"fc_1","name":"calculator","arguments":"{\"expression\":\"(12+8)/4\"}"}]}`))
		case 2:
			foundCall, foundOutput := false, false
			for _, item := range payload.Input {
				foundCall = foundCall || strings.Contains(string(item), `"call_id":"fc_1","name":"calculator"`)
				foundOutput = foundOutput || (strings.Contains(string(item), `"type":"function_call_output"`) && strings.Contains(string(item), `"call_id":"fc_1"`) && strings.Contains(string(item), `\"result\":5`))
			}
			if !foundCall || !foundOutput {
				t.Errorf("missing tool call or result in DeepSeek replay: %s", payload.Input)
			}
			_, _ = w.Write([]byte(`{"id":"ds_2","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"结果是 5。"}]}]}`))
		case 3:
			foundPrior := false
			for _, item := range payload.Input {
				foundPrior = foundPrior || strings.Contains(string(item), "结果是 5")
			}
			if !foundPrior {
				t.Errorf("follow-up lost session history: %s", payload.Input)
			}
			_, _ = w.Write([]byte(`{"id":"ds_3","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"刚才算出 5。"}]}]}`))
		default:
			t.Errorf("unexpected request %d", step)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	store, err := session.Open(ctx, filepath.Join(t.TempDir(), "deepseek.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conversation, err := store.Create(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Calculator{}); err != nil {
		t.Fatal(err)
	}
	client := llm.NewDeepSeekClient("test-key")
	client.Endpoint = server.URL + "/responses"
	client.HTTP = server.Client()
	runner := NewRunner(client, store, registry, "deepseek-flash")
	first, err := runner.Run(ctx, "user-a", conversation.ID, "计算 (12+8)/4")
	if err != nil || first.Answer != "结果是 5。" || first.ToolCalls != 1 || first.LLMCalls != 2 {
		t.Fatalf("first turn = %+v, %v", first, err)
	}
	second, err := runner.Run(ctx, "user-a", conversation.ID, "刚才结果是多少？")
	if err != nil || second.Answer != "刚才算出 5。" || step != 3 {
		t.Fatalf("follow-up = %+v, step=%d, %v", second, step, err)
	}
	traces, err := store.ListTraces(ctx, "user-a", conversation.ID)
	if err != nil || len(traces) != 1 || traces[0].ToolName != "calculator" {
		t.Fatalf("trace = %+v, %v", traces, err)
	}
}
