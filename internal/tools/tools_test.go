package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"demoagent/internal/session"
)

func TestRegistryAndArgumentValidation(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Calculator{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(Calculator{}); err == nil {
		t.Fatal("duplicate registration should fail")
	}
	definitions, err := registry.Definitions()
	if err != nil || len(definitions) != 1 {
		t.Fatalf("definitions: %v, %v", definitions, err)
	}
	var definition struct {
		Name       string `json:"name"`
		Strict     bool   `json:"strict"`
		Parameters Schema `json:"parameters"`
	}
	if err := json.Unmarshal(definitions[0], &definition); err != nil {
		t.Fatal(err)
	}
	if definition.Name != "calculator" || !definition.Strict || definition.Parameters.AdditionalProperties {
		t.Fatalf("invalid tool definition: %+v", definition)
	}
	for _, invalid := range []string{`{}`, `{"expression":2}`, `{"expression":"1+1","extra":true}`, `null`} {
		if _, err := ValidateArguments(Calculator{}.Spec().Parameters, json.RawMessage(invalid)); err == nil {
			t.Fatalf("accepted bad arguments: %s", invalid)
		}
	}
}

func TestCalculatorAllowsArithmeticOnly(t *testing.T) {
	calculator := Calculator{}
	for _, test := range []struct {
		expression string
		want       float64
	}{
		{"(12+8)/4", 5}, {"-3*2+7", 1}, {"2.5*4", 10},
	} {
		result, err := calculator.Execute(context.Background(), Invocation{}, map[string]any{"expression": test.expression})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.(map[string]any)["result"].(float64); got != test.want {
			t.Fatalf("%s = %v, want %v", test.expression, got, test.want)
		}
	}
	for _, expression := range []string{"1/0", "1<<2", "foo(1)", "1;2"} {
		if _, err := calculator.Execute(context.Background(), Invocation{}, map[string]any{"expression": expression}); err == nil {
			t.Fatalf("accepted unsafe expression %q", expression)
		}
	}
}

func TestMockToolsAndSessionTodos(t *testing.T) {
	ctx := context.Background()
	searchResult, err := (Search{}).Execute(ctx, Invocation{}, map[string]any{"query": "项目说明"})
	if err != nil || searchResult.(map[string]any)["mock"] != true {
		t.Fatalf("search result: %v, %v", searchResult, err)
	}
	weatherResult, err := (Weather{}).Execute(ctx, Invocation{}, map[string]any{"city": "北京"})
	if err != nil || weatherResult.(map[string]any)["mock"] != true {
		t.Fatalf("weather result: %v, %v", weatherResult, err)
	}

	store, err := session.Open(ctx, filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, _ := store.Create(ctx, "user-a")
	second, _ := store.Create(ctx, "user-a")
	tool := TodoTool{Store: store}
	if _, err := tool.Execute(ctx, Invocation{UserID: "user-a", SessionID: first.ID}, map[string]any{"action": "add", "text": "带伞"}); err != nil {
		t.Fatal(err)
	}
	firstList, err := tool.Execute(ctx, Invocation{UserID: "user-a", SessionID: first.ID}, map[string]any{"action": "list", "text": ""})
	if err != nil || len(firstList.(map[string]any)["items"].([]session.Todo)) != 1 {
		t.Fatalf("first list: %v, %v", firstList, err)
	}
	secondList, err := tool.Execute(ctx, Invocation{UserID: "user-a", SessionID: second.ID}, map[string]any{"action": "list", "text": ""})
	if err != nil || len(secondList.(map[string]any)["items"].([]session.Todo)) != 0 {
		t.Fatalf("second session leaked todos: %v, %v", secondList, err)
	}
}
