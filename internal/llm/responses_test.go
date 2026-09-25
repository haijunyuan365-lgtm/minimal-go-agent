package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestResponsesClientWireFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request method or auth header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["model"] != "test-model" || payload["store"] != false || payload["tool_choice"] != "auto" {
			t.Errorf("unexpected payload: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer server.Close()
	client := &ResponsesClient{APIKey: "test-key", Endpoint: server.URL, HTTP: server.Client()}
	response, err := client.CreateResponse(context.Background(), Request{
		Model: "test-model", Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		Tools: []json.RawMessage{json.RawMessage(`{"type":"function","name":"calculator"}`)}, Store: false,
	})
	if err != nil || response.ID != "resp_1" || len(response.Output) != 1 {
		t.Fatalf("response = %+v, %v", response, err)
	}
}

func TestResponsesClientHandlesAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid API key","type":"invalid_request_error"}}`))
	}))
	defer server.Close()
	client := &ResponsesClient{APIKey: "bad", Endpoint: server.URL, HTTP: server.Client()}
	if _, err := client.CreateResponse(context.Background(), Request{Model: "test", Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}}); err == nil {
		t.Fatal("expected provider error")
	}
}

func TestLiveResponsesAPI(t *testing.T) {
	if os.Getenv("AGENT_LIVE_TEST") != "1" {
		t.Skip("set AGENT_LIVE_TEST=1 to opt in to a real API call")
	}
	apiKey, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL")
	if apiKey == "" || model == "" {
		t.Fatal("OPENAI_API_KEY and OPENAI_MODEL are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := NewResponsesClient(apiKey).CreateResponse(ctx, Request{
		Model: model, Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"Reply with the word ready."}`)},
	})
	if err != nil || len(response.Output) == 0 {
		t.Fatalf("live API response failed: %v", err)
	}
}
