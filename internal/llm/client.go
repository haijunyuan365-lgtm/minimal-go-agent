package llm

import (
	"context"
	"encoding/json"
)

// Client is the only LLM dependency visible to the Agent runtime.
// A real Responses API adapter and deterministic test doubles implement it.
type Client interface {
	CreateResponse(context.Context, Request) (Response, error)
}

type Request struct {
	Model            string
	Instructions     string
	Input            []json.RawMessage
	Tools            []json.RawMessage
	Store            bool
	ReasoningSummary string
	ReasoningEffort  string
}

type Response struct {
	ID                string
	Status            string
	Output            []json.RawMessage
	Usage             json.RawMessage
	IncompleteDetails json.RawMessage
}
