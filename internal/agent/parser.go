package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"demoagent/internal/llm"
)

type ToolCall struct {
	CallID    string
	Name      string
	Arguments json.RawMessage
}

type Decision struct {
	Calls              []ToolCall
	Final              string
	ReasoningSummaries []string
}

func ParseResponse(response llm.Response) (Decision, error) {
	var decision Decision
	seen := make(map[string]bool)
	for _, raw := range response.Output {
		var item struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			CallID    string `json:"call_id"`
			Arguments string `json:"arguments"`
			Role      string `json:"role"`
			Phase     string `json:"phase"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Summary []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return Decision{}, fmt.Errorf("decode LLM output item: %w", err)
		}
		switch item.Type {
		case "function_call":
			if item.CallID == "" || item.Name == "" || seen[item.CallID] {
				return Decision{}, errors.New("invalid or duplicate tool call id/name")
			}
			seen[item.CallID] = true
			decision.Calls = append(decision.Calls, ToolCall{item.CallID, item.Name, json.RawMessage(item.Arguments)})
		case "message":
			if item.Role != "assistant" || (item.Phase != "" && item.Phase != "final_answer") {
				continue
			}
			for _, content := range item.Content {
				if content.Type == "output_text" {
					decision.Final += content.Text
				}
			}
		case "reasoning":
			for _, summary := range item.Summary {
				if summary.Type == "summary_text" && strings.TrimSpace(summary.Text) != "" {
					decision.ReasoningSummaries = append(decision.ReasoningSummaries, summary.Text)
				}
			}
		}
	}
	if len(decision.Calls) == 0 && strings.TrimSpace(decision.Final) == "" {
		return Decision{}, errors.New("LLM response has neither tool calls nor a final answer")
	}
	return decision, nil
}
