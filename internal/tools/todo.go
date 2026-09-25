package tools

import (
	"context"
	"errors"
	"strings"

	"demoagent/internal/session"
)

type TodoStore interface {
	AddTodo(context.Context, string, string, string) (session.Todo, error)
	ListTodos(context.Context, string, string) ([]session.Todo, error)
}

type TodoTool struct{ Store TodoStore }

func (TodoTool) Spec() Spec {
	return Spec{
		Name:        "todo",
		Description: "Add or list todo items for the current conversation session. Use action=add with nonempty text, or action=list with text as an empty string.",
		Parameters: Schema{
			Type: "object", Properties: map[string]Property{
				"action": {Type: "string", Description: "add or list", Enum: []string{"add", "list"}},
				"text":   {Type: "string", Description: "Todo text for add; empty string for list"},
			}, Required: []string{"action", "text"},
		},
	}
}

func (tool TodoTool) Execute(ctx context.Context, invocation Invocation, args map[string]any) (any, error) {
	if tool.Store == nil {
		return nil, errors.New("todo storage is unavailable")
	}
	text := strings.TrimSpace(args["text"].(string))
	switch args["action"].(string) {
	case "add":
		if text == "" || len(text) > 500 {
			return nil, errors.New("todo text must contain 1 to 500 characters")
		}
		item, err := tool.Store.AddTodo(ctx, invocation.UserID, invocation.SessionID, text)
		if err != nil {
			return nil, err
		}
		return map[string]any{"added": item}, nil
	case "list":
		items, err := tool.Store.ListTodos(ctx, invocation.UserID, invocation.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	default:
		return nil, errors.New("unsupported todo action")
	}
}
