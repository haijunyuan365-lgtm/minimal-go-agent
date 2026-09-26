package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"demoagent/internal/config"
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

func TestDeepSeekResponsesWireFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer deepseek-key" {
			t.Errorf("unexpected DeepSeek request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, exists := payload["store"]; exists {
			t.Error("DeepSeek request should omit unsupported store")
		}
		if string(payload["tool_choice"]) != `"auto"` || string(payload["reasoning"]) != `{"effort":"low"}` {
			t.Errorf("unexpected DeepSeek options: %s", payload)
		}
		var definitions []map[string]any
		if err := json.Unmarshal(payload["tools"], &definitions); err != nil || len(definitions) != 1 {
			t.Fatalf("unexpected tools: %s, %v", payload["tools"], err)
		}
		if _, exists := definitions[0]["strict"]; exists {
			t.Error("DeepSeek tool definition should omit unsupported strict")
		}
		_, _ = w.Write([]byte(`{"id":"resp_ds","status":"completed","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"plan"}]},{"type":"function_call","call_id":"call_1","name":"calculator","arguments":"{\"expression\":\"2+2\"}"}]}`))
	}))
	defer server.Close()
	client := NewDeepSeekClient("deepseek-key", server.URL+"/responses", 45*time.Second)
	client.HTTP = server.Client()
	response, err := client.CreateResponse(context.Background(), Request{
		Model: "deepseek-flash", Instructions: "Use tools", ReasoningEffort: "low",
		Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"2+2?"}`)},
		Tools: []json.RawMessage{json.RawMessage(`{"type":"function","name":"calculator","strict":true,"parameters":{"type":"object"}}`)},
	})
	if err != nil || response.ID != "resp_ds" || len(response.Output) != 2 {
		t.Fatalf("DeepSeek response = %+v, %v", response, err)
	}
}

type waitForCancellation struct{}

func (waitForCancellation) RoundTrip(r *http.Request) (*http.Response, error) {
	<-r.Context().Done()
	return nil, r.Context().Err()
}

func TestResponsesClientTimeout(t *testing.T) {
	client := NewDeepSeekClient("test-key", "https://example.com/responses", 20*time.Millisecond)
	client.HTTP.Transport = waitForCancellation{}
	_, err := client.CreateResponse(context.Background(), Request{
		Model: "deepseek-flash", Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want request deadline error, got %v", err)
	}
}

func TestResponsesClientRejectsInvalidResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
	}{
		{"malformed JSON", `not-json`},
		{"missing output", `{"id":"resp_1","status":"completed"}`},
		{"incomplete", `{"id":"resp_1","status":"incomplete","output":[],"incomplete_details":{"reason":"max_output_tokens"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := NewResponsesClient("test-key", server.URL+"/responses", time.Second)
			if _, err := client.CreateResponse(context.Background(), Request{
				Model: "test-model", Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
			}); err == nil {
				t.Fatal("expected malformed or incomplete response to fail")
			}
		})
	}
}

func TestLiveResponsesAPI(t *testing.T) {
	if os.Getenv("AGENT_LIVE_TEST") != "1" {
		t.Skip("set AGENT_LIVE_TEST=1 to opt in to a real API call")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	client := NewConfiguredClient(cfg)
	if client == nil {
		t.Fatal("selected LLM provider API key is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := client.CreateResponse(ctx, Request{
		Model: cfg.SelectedLLM().Model, Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"Reply with the word ready."}`)},
	})
	if err != nil || len(response.Output) == 0 {
		t.Fatalf("live API response failed: %v", err)
	}
}
