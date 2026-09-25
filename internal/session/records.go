package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Todo struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ToolTrace struct {
	ID         int64           `json:"id"`
	RequestID  string          `json:"request_id"`
	Step       int             `json:"step"`
	ToolName   string          `json:"tool_name"`
	Arguments  json.RawMessage `json:"arguments"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
	DurationMS int64           `json:"duration_ms"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (s *Store) AddTodo(ctx context.Context, userID, sessionID, text string) (Todo, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO todos(session_id, text, done, created_at)
		SELECT id, ?, 0, ? FROM sessions WHERE id = ? AND user_id = ?`,
		text, now.Format(time.RFC3339Nano), sessionID, userID,
	)
	if err != nil {
		return Todo{}, fmt.Errorf("add todo: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Todo{}, fmt.Errorf("check todo insertion: %w", err)
	}
	if affected == 0 {
		return Todo{}, ErrNotFound
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Todo{}, fmt.Errorf("read todo id: %w", err)
	}
	return Todo{ID: id, Text: text, CreatedAt: now}, nil
}

func (s *Store) ListTodos(ctx context.Context, userID, sessionID string) ([]Todo, error) {
	if _, err := s.Get(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.text, t.done, t.created_at FROM todos t
		WHERE t.session_id = ? ORDER BY t.id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()
	items := make([]Todo, 0)
	for rows.Next() {
		var item Todo
		var done int
		var created string
		if err := rows.Scan(&item.ID, &item.Text, &done, &created); err != nil {
			return nil, fmt.Errorf("scan todo: %w", err)
		}
		item.Done = done != 0
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse todo time: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AddMessage(ctx context.Context, userID, sessionID, role, content string) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO messages(session_id, role, content, created_at)
		SELECT id, ?, ?, ? FROM sessions WHERE id = ? AND user_id = ?`,
		role, content, time.Now().UTC().Format(time.RFC3339Nano), sessionID, userID)
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check message insertion: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RecentMessages(ctx context.Context, userID, sessionID string, limit int) ([]Message, error) {
	if _, err := s.Get(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, errors.New("message limit must be positive")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT role, content FROM messages WHERE session_id = ? AND role IN ('user', 'assistant')
		ORDER BY id DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("read messages: %w", err)
	}
	defer rows.Close()
	messages := make([]Message, 0)
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.Role, &message.Content); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (s *Store) AddTrace(ctx context.Context, userID, sessionID string, trace ToolTrace) error {
	_, err := s.Get(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tool_traces(session_id, request_id, step, tool_name, arguments_json,
		result_json, error_text, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, sessionID, trace.RequestID, trace.Step,
		trace.ToolName, string(trace.Arguments), nullableJSON(trace.Result),
		nullableText(trace.Error), trace.DurationMS, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save tool trace: %w", err)
	}
	return nil
}

func (s *Store) ListTraces(ctx context.Context, userID, sessionID string) ([]ToolTrace, error) {
	if _, err := s.Get(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, request_id, step, tool_name, arguments_json, result_json, error_text,
		duration_ms, created_at FROM tool_traces WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()
	traces := make([]ToolTrace, 0)
	for rows.Next() {
		var item ToolTrace
		var arguments, created string
		var result, errorText sql.NullString
		if err := rows.Scan(&item.ID, &item.RequestID, &item.Step, &item.ToolName,
			&arguments, &result, &errorText, &item.DurationMS, &created); err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}
		item.Arguments = json.RawMessage(arguments)
		if result.Valid {
			item.Result = json.RawMessage(result.String)
		}
		if errorText.Valid {
			item.Error = errorText.String
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse trace time: %w", err)
		}
		traces = append(traces, item)
	}
	return traces, rows.Err()
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
