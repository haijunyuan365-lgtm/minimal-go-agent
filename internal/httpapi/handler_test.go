package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"demoagent/internal/agent"
	"demoagent/internal/llm"
	"demoagent/internal/session"
	"demoagent/internal/tools"
)

type fakeLLM struct {
	responses []llm.Response
}

func (f *fakeLLM) CreateResponse(_ context.Context, _ llm.Request) (llm.Response, error) {
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

func TestSessionRoutes(t *testing.T) {
	store, err := session.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := NewHandler(store, nil)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d: %s", health.Code, health.Body.String())
	}

	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString(`{"user_id":"user-a"}`)))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}
	var item session.Session
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ID == "" {
		t.Fatal("empty session id")
	}

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/sessions/"+item.ID+"?user_id=user-a", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", read.Code, read.Body.String())
	}
	wrongUser := httptest.NewRecorder()
	handler.ServeHTTP(wrongUser, httptest.NewRequest(http.MethodGet, "/sessions/"+item.ID+"?user_id=user-b", nil))
	if wrongUser.Code != http.StatusNotFound {
		t.Fatalf("wrong user status = %d", wrongUser.Code)
	}

	chat := httptest.NewRecorder()
	handler.ServeHTTP(chat, httptest.NewRequest(http.MethodPost, "/sessions/"+item.ID+"/messages", bytes.NewBufferString(`{}`)))
	if chat.Code != http.StatusBadRequest {
		t.Fatalf("invalid chat request status = %d", chat.Code)
	}
}

func TestCreateSessionRejectsInvalidJSON(t *testing.T) {
	store, err := session.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, body := range []string{`{}`, `{"user_id":"a","unexpected":true}`, `{"user_id":"a"}{"user_id":"b"}`} {
		recorder := httptest.NewRecorder()
		NewHandler(store, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString(body)))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %q returned %d", body, recorder.Code)
		}
	}
}

func TestChatAndTraceRoutes(t *testing.T) {
	ctx := context.Background()
	store, err := session.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	item, err := store.Create(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Calculator{}); err != nil {
		t.Fatal(err)
	}
	client := &fakeLLM{responses: []llm.Response{
		{Output: []json.RawMessage{json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"calculator","arguments":"{\"expression\":\"2+2\"}"}`)}},
		{Output: []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"4"}]}`)}},
	}}
	handler := NewHandler(store, agent.NewRunner(client, store, registry, "test-model", agent.Limits{
		MaxLLMCalls: 6, MaxToolCalls: 8, MaxMessageChars: 4000,
		MaxRecentTurns: 8, ContextCharLimit: 12000, RecentCharBudget: 8000,
		MaxSummaryChars: 2000, MaxCompactionCalls: 3,
	}))
	chat := httptest.NewRecorder()
	handler.ServeHTTP(chat, httptest.NewRequest(http.MethodPost, "/sessions/"+item.ID+"/messages",
		bytes.NewBufferString(`{"user_id":"user-a","message":"2+2=?"}`)))
	if chat.Code != http.StatusOK {
		t.Fatalf("chat status = %d: %s", chat.Code, chat.Body.String())
	}
	var answer struct {
		Answer    string `json:"answer"`
		ToolCalls int    `json:"tool_calls"`
	}
	if err := json.Unmarshal(chat.Body.Bytes(), &answer); err != nil || answer.Answer != "4" || answer.ToolCalls != 1 {
		t.Fatalf("chat answer = %+v, %v", answer, err)
	}
	trace := httptest.NewRecorder()
	handler.ServeHTTP(trace, httptest.NewRequest(http.MethodGet, "/sessions/"+item.ID+"/trace?user_id=user-a", nil))
	if trace.Code != http.StatusOK || !bytes.Contains(trace.Body.Bytes(), []byte(`"tool_name":"calculator"`)) {
		t.Fatalf("trace status = %d: %s", trace.Code, trace.Body.String())
	}
	other := httptest.NewRecorder()
	handler.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/sessions/"+item.ID+"/trace?user_id=user-b", nil))
	if other.Code != http.StatusNotFound {
		t.Fatalf("other user saw traces: %d", other.Code)
	}
}
