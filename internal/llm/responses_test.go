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
	client := NewDeepSeekClient("deepseek-key")
	client.Endpoint = server.URL + "/responses"
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

func TestLiveResponsesAPI(t *testing.T) {
	if os.Getenv("AGENT_LIVE_TEST") != "1" {
		t.Skip("set AGENT_LIVE_TEST=1 to opt in to a real API call")
	}
	provider := os.Getenv("LLM_PROVIDER")
	var client Client
	apiKey, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL")
	if provider == "deepseek" {
		apiKey, model = os.Getenv("DEEPSEEK_API_KEY"), os.Getenv("DEEPSEEK_MODEL")
		deepSeek := NewDeepSeekClient(apiKey)
		if endpoint := os.Getenv("DEEPSEEK_ENDPOINT"); endpoint != "" {
			deepSeek.Endpoint = endpoint
		}
		client = deepSeek
	} else {
		client = NewResponsesClient(apiKey)
	}
	if apiKey == "" || model == "" {
		t.Fatal("selected LLM provider API key and model are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := client.CreateResponse(ctx, Request{
		Model: model, Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"Reply with the word ready."}`)},
	})
	if err != nil || len(response.Output) == 0 {
		t.Fatalf("live API response failed: %v", err)
	}
}
