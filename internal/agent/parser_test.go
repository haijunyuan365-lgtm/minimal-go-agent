package agent

import (
	"encoding/json"
	"testing"

	"demoagent/internal/llm"
)

func TestParseResponse(t *testing.T) {
	response := llm.Response{Output: []json.RawMessage{
		json.RawMessage(`{"type":"reasoning","summary":[{"type":"summary_text","text":"Need arithmetic."}]}`),
		json.RawMessage(`{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"I will calculate."}]}`),
		json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"calculator","arguments":"{\"expression\":\"2+2\"}"}`),
	}}
	decision, err := ParseResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.Calls) != 1 || decision.Calls[0].Name != "calculator" || len(decision.ReasoningSummaries) != 1 || decision.Final != "" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	final, err := ParseResponse(llm.Response{Output: []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"Done."}]}`),
	}})
	if err != nil || final.Final != "Done." {
		t.Fatalf("final = %+v, %v", final, err)
	}
}

func TestParseResponseRejectsDuplicateCallsAndEmptyOutput(t *testing.T) {
	call := json.RawMessage(`{"type":"function_call","call_id":"same","name":"calculator","arguments":"{}"}`)
	if _, err := ParseResponse(llm.Response{Output: []json.RawMessage{call, call}}); err == nil {
		t.Fatal("duplicate call id accepted")
	}
	if _, err := ParseResponse(llm.Response{}); err == nil {
		t.Fatal("empty output accepted")
	}
}
