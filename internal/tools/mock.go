package tools

import (
	"context"
	"errors"
	"strings"
)

type Search struct{}

func (Search) Spec() Spec {
	return Spec{
		Name:        "search",
		Description: "Search a small built-in demo knowledge base. Results are mock examples, not live web search. Use for questions about the demo project or Go agent concepts.",
		Parameters: Schema{
			Type: "object", Properties: map[string]Property{
				"query": {Type: "string", Description: "Search terms"},
			}, Required: []string{"query"},
		},
	}
}

func (Search) Execute(_ context.Context, _ Invocation, args map[string]any) (any, error) {
	query := strings.TrimSpace(args["query"].(string))
	if query == "" || len(query) > 200 {
		return nil, errors.New("query must contain 1 to 200 characters")
	}
	type hit struct {
		Title   string `json:"title"`
		Snippet string `json:"snippet"`
	}
	var results []hit
	lower := strings.ToLower(query)
	if strings.Contains(lower, "agent") || strings.Contains(lower, "项目") || strings.Contains(lower, "说明") {
		results = append(results, hit{"DemoAgent project", "A Go learning project that implements its own Agent loop, tool registry, session storage, and trace."})
	}
	if strings.Contains(lower, "go") || strings.Contains(lower, "工具") || strings.Contains(lower, "tool") {
		results = append(results, hit{"Tool registration", "Each local tool has a name, description, JSON Schema parameters, and an Execute method."})
	}
	if results == nil {
		results = []hit{}
	}
	return map[string]any{"mock": true, "query": query, "results": results}, nil
}

type Weather struct{}

func (Weather) Spec() Spec {
	return Spec{
		Name:        "weather",
		Description: "Return illustrative weather fixture data for Beijing, Shanghai, or Shenzhen. This is mock data, never a live forecast.",
		Parameters: Schema{
			Type: "object", Properties: map[string]Property{
				"city": {Type: "string", Description: "City name, in Chinese or English"},
			}, Required: []string{"city"},
		},
	}
}

func (Weather) Execute(_ context.Context, _ Invocation, args map[string]any) (any, error) {
	city := strings.ToLower(strings.TrimSpace(args["city"].(string)))
	fixtures := map[string]struct {
		Name      string
		Condition string
		Celsius   int
	}{
		"北京": {"北京", "多云", 22}, "beijing": {"北京", "多云", 22},
		"上海": {"上海", "小雨", 24}, "shanghai": {"上海", "小雨", 24},
		"深圳": {"深圳", "晴", 29}, "shenzhen": {"深圳", "晴", 29},
	}
	fixture, found := fixtures[city]
	if !found {
		return map[string]any{"mock": true, "city": args["city"], "available": false, "message": "no fixture for this city"}, nil
	}
	return map[string]any{
		"mock": true, "city": fixture.Name, "available": true,
		"condition": fixture.Condition, "temperature_c": fixture.Celsius,
		"note": "Illustrative fixture only; not current weather.",
	}, nil
}
