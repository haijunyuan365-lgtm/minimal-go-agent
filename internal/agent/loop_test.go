package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"demoagent/internal/llm"
	"demoagent/internal/session"
	"demoagent/internal/tools"
)

type scriptedClient struct {
	responses []llm.Response
	requests  []llm.Request
}

func (client *scriptedClient) CreateResponse(_ context.Context, request llm.Request) (llm.Response, error) {
	request.Input = append([]json.RawMessage(nil), request.Input...)
	client.requests = append(client.requests, request)
	if len(client.responses) == 0 {
		return llm.Response{}, errors.New("unexpected LLM call")
	}
	response := client.responses[0]
	client.responses = client.responses[1:]
	return response, nil
}

func testRunner(t *testing.T, client llm.Client) (*Runner, *session.Store, session.Session) {
	t.Helper()
	store, err := session.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	item, err := store.Create(context.Background(), "user-a")
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	for _, tool := range []tools.Tool{tools.Calculator{}, tools.Search{}, tools.Weather{}, tools.TodoTool{Store: store}} {
		if err := registry.Register(tool); err != nil {
			t.Fatal(err)
		}
	}
	return NewRunner(client, store, registry, "test-model"), store, item
}

func toolCall(id, name, args string) json.RawMessage {
	encoded, _ := json.Marshal(map[string]string{
		"type": "function_call", "call_id": id, "name": name, "arguments": args,
	})
	return encoded
}

func finalMessage(text string) json.RawMessage {
	encoded, _ := json.Marshal(map[string]any{
		"type": "message", "role": "assistant", "phase": "final_answer",
		"content": []map[string]string{{"type": "output_text", "text": text}},
	})
	return encoded
}

func TestRunMultipleToolsAndTrace(t *testing.T) {
	client := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{
			json.RawMessage(`{"type":"reasoning","summary":[{"type":"summary_text","text":"Use two tools."}]}`),
			toolCall("call_calc", "calculator", `{"expression":"(12+8)/4"}`),
			toolCall("call_weather", "weather", `{"city":"北京"}`),
		}},
		{Output: []json.RawMessage{finalMessage("计算结果是 5；天气是模拟数据。")}},
	}}
	runner, store, item := testRunner(t, client)
	result, err := runner.Run(context.Background(), "user-a", item.ID, "算一下，再查北京天气")
	if err != nil {
		t.Fatal(err)
	}
	if result.LLMCalls != 2 || result.ToolCalls != 2 || len(result.ReasoningSummaries) != 1 {
		t.Fatalf("wrong result: %+v", result)
	}
	if len(client.requests) != 2 || len(client.requests[1].Input) != 6 {
		t.Fatalf("tool outputs were not replayed: %+v", client.requests)
	}
	parts := make([]string, 0, len(client.requests[1].Input))
	for _, item := range client.requests[1].Input {
		parts = append(parts, string(item))
	}
	joined := strings.Join(parts, "\n")
	if !strings.Contains(joined, `function_call_output`) || !strings.Contains(joined, `call_calc`) || !strings.Contains(joined, `result`) {
		t.Fatalf("missing function output: %s", joined)
	}
	traces, err := store.ListTraces(context.Background(), "user-a", item.ID)
	if err != nil || len(traces) != 2 || traces[0].ToolName != "calculator" || traces[1].ToolName != "weather" {
		t.Fatalf("wrong trace: %+v, %v", traces, err)
	}
}

func TestRunDirectAnswerAndFollowupHistory(t *testing.T) {
	client := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{finalMessage("你好！")}},
		{Output: []json.RawMessage{finalMessage("你刚才打了招呼。")}},
	}}
	runner, _, item := testRunner(t, client)
	if _, err := runner.Run(context.Background(), "user-a", item.ID, "你好"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), "user-a", item.ID, "我刚才说了什么？"); err != nil {
		t.Fatal(err)
	}
	if len(client.requests[1].Input) != 3 {
		t.Fatalf("follow-up context missing: %+v", client.requests[1].Input)
	}
	if !strings.Contains(string(client.requests[1].Input[0]), "你好") || !strings.Contains(string(client.requests[1].Input[1]), "你好") {
		t.Fatalf("history is wrong: %+v", client.requests[1].Input)
	}
	if !strings.Contains(string(client.requests[1].Input[1]), `"phase":"final_answer"`) {
		t.Fatalf("assistant phase was lost: %s", client.requests[1].Input[1])
	}
}

func TestRunUnknownToolErrorAndLimit(t *testing.T) {
	client := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{toolCall("bad", "missing", `{}`)}},
		{Output: []json.RawMessage{finalMessage("这个工具不可用。")}},
	}}
	runner, store, item := testRunner(t, client)
	if _, err := runner.Run(context.Background(), "user-a", item.ID, "调用不存在的工具"); err != nil {
		t.Fatal(err)
	}
	traces, err := store.ListTraces(context.Background(), "user-a", item.ID)
	if err != nil || len(traces) != 1 || !strings.Contains(traces[0].Error, "unknown tool") {
		t.Fatalf("unknown tool was not traced: %+v, %v", traces, err)
	}

	limitClient := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{toolCall("first", "calculator", `{"expression":"2+2"}`)}},
		{Output: []json.RawMessage{toolCall("second", "calculator", `{"expression":"3+3"}`)}},
	}}
	limited, _, another := testRunner(t, limitClient)
	limited.MaxLLMCalls = 2
	result, err := limited.Run(context.Background(), "user-a", another.ID, "反复计算")
	if !errors.Is(err, ErrLimit) || result.ToolCalls != 1 || result.LLMCalls != 2 {
		t.Fatalf("limit result: %+v, %v", result, err)
	}
}

func TestToolFollowupReadsSessionTodo(t *testing.T) {
	client := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{toolCall("add", "todo", `{"action":"add","text":"带伞"}`)}},
		{Output: []json.RawMessage{finalMessage("已记下带伞。")}},
		{Output: []json.RawMessage{toolCall("list", "todo", `{"action":"list","text":""}`)}},
		{Output: []json.RawMessage{finalMessage("你记了带伞。")}},
	}}
	runner, store, conversation := testRunner(t, client)
	if _, err := runner.Run(context.Background(), "user-a", conversation.ID, "帮我记下带伞"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), "user-a", conversation.ID, "刚才记了什么待办？"); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 4 || len(client.requests[2].Input) != 3 {
		t.Fatalf("follow-up context was not recalled: %+v", client.requests)
	}
	lastInput := string(client.requests[3].Input[len(client.requests[3].Input)-1])
	if !strings.Contains(lastInput, "带伞") || !strings.Contains(lastInput, "function_call_output") {
		t.Fatalf("todo list was not returned to model: %s", lastInput)
	}
	todos, err := store.ListTodos(context.Background(), "user-a", conversation.ID)
	if err != nil || len(todos) != 1 || todos[0].Text != "带伞" {
		t.Fatalf("todo not persisted: %+v, %v", todos, err)
	}
}
