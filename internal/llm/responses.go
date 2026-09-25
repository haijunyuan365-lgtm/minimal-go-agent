package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultEndpoint = "https://api.openai.com/v1/responses"
const DefaultDeepSeekEndpoint = "https://api.deepseek.com/responses"

type ResponsesClient struct {
	APIKey   string
	Endpoint string
	HTTP     *http.Client
	deepSeek bool
}

// NewDeepSeekClient uses DeepSeek's native, stateless Responses API.
func NewDeepSeekClient(apiKey string) *ResponsesClient {
	return &ResponsesClient{
		APIKey: apiKey, Endpoint: DefaultDeepSeekEndpoint,
		HTTP: &http.Client{Timeout: 45 * time.Second}, deepSeek: true,
	}
}

func NewResponsesClient(apiKey string) *ResponsesClient {
	return &ResponsesClient{
		APIKey: apiKey, Endpoint: DefaultEndpoint,
		HTTP: &http.Client{Timeout: 45 * time.Second},
	}
}

func (client *ResponsesClient) CreateResponse(ctx context.Context, request Request) (Response, error) {
	if strings.TrimSpace(client.APIKey) == "" || strings.TrimSpace(request.Model) == "" {
		if client.deepSeek {
			return Response{}, errors.New("DEEPSEEK_API_KEY and DEEPSEEK_MODEL are required for chat")
		}
		return Response{}, errors.New("OPENAI_API_KEY and OPENAI_MODEL are required for chat")
	}
	endpoint := client.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
		if client.deepSeek {
			endpoint = DefaultDeepSeekEndpoint
		}
	}
	httpClient := client.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	payload := map[string]any{
		"model": request.Model, "instructions": request.Instructions,
		"input": request.Input, "tools": request.Tools,
	}
	if client.deepSeek {
		// DeepSeek accepts Responses items but has no server-side storage or
		// strict function mode. Omit the OpenAI-only request fields.
		if len(request.Tools) == 0 {
			delete(payload, "tools")
		} else {
			definitions := make([]map[string]any, 0, len(request.Tools))
			for _, raw := range request.Tools {
				var definition map[string]any
				if err := json.Unmarshal(raw, &definition); err != nil {
					return Response{}, fmt.Errorf("decode DeepSeek tool definition: %w", err)
				}
				delete(definition, "strict")
				definitions = append(definitions, definition)
			}
			payload["tools"] = definitions
		}
		if request.ReasoningEffort != "" {
			payload["reasoning"] = map[string]string{"effort": request.ReasoningEffort}
		}
	} else {
		payload["store"] = request.Store
		if request.ReasoningSummary != "" {
			payload["reasoning"] = map[string]string{"summary": request.ReasoningSummary}
		}
	}
	if len(request.Tools) > 0 {
		payload["tool_choice"] = "auto"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("encode LLM request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return Response{}, fmt.Errorf("create LLM request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+client.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return Response{}, fmt.Errorf("send LLM request: %w", err)
	}
	defer httpResponse.Body.Close()
	const maxBody = 8 << 20
	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxBody+1))
	if err != nil {
		return Response{}, fmt.Errorf("read LLM response: %w", err)
	}
	if len(body) > maxBody {
		return Response{}, errors.New("LLM response exceeds 8 MiB")
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		var apiError struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &apiError) == nil && apiError.Error.Message != "" {
			return Response{}, fmt.Errorf("LLM API HTTP %d: %s", httpResponse.StatusCode, apiError.Error.Message)
		}
		return Response{}, fmt.Errorf("LLM API HTTP %d", httpResponse.StatusCode)
	}
	var parsed struct {
		ID                string            `json:"id"`
		Status            string            `json:"status"`
		Output            []json.RawMessage `json:"output"`
		Usage             json.RawMessage   `json:"usage"`
		IncompleteDetails json.RawMessage   `json:"incomplete_details"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Response{}, fmt.Errorf("decode LLM response: %w", err)
	}
	if parsed.ID == "" || parsed.Output == nil {
		return Response{}, errors.New("LLM response is missing id or output")
	}
	if parsed.Status != "" && parsed.Status != "completed" {
		return Response{}, fmt.Errorf("LLM response status %q: %s", parsed.Status, string(parsed.IncompleteDetails))
	}
	return Response{
		ID: parsed.ID, Status: parsed.Status, Output: parsed.Output,
		Usage: parsed.Usage, IncompleteDetails: parsed.IncompleteDetails,
	}, nil
}
