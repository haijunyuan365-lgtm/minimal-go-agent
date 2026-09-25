package session

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsAndIsolatesSessions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "agent.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.Create(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("different windows received the same session id")
	}
	if _, err := store.Get(ctx, "user-b", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user access should be hidden, got %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Get(ctx, "user-a", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != first.ID || got.UserID != "user-a" || got.CreatedAt.IsZero() {
		t.Fatalf("persisted session changed: %+v", got)
	}
}

func TestOpenMigratesExistingSessionsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec("INSERT INTO sessions(id,user_id,summary,created_at,updated_at) VALUES(?,?,?,?,?)",
		"legacy-session", "user-a", "old memory", now, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	item, err := store.Get(context.Background(), "user-a", "legacy-session")
	if err != nil || item.Summary != "old memory" || item.SummaryThroughMessageID != 0 {
		t.Fatalf("migration lost session: %+v, %v", item, err)
	}
}

func TestMemoryBoundaryAndRawMessagesPersist(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Create(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddTurn(ctx, "user-a", item.ID, "我叫小海", "记住了"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddTurn(ctx, "user-a", item.ID, "我喜欢 Go", "好的"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddTodo(ctx, "user-a", item.ID, "写周报"); err != nil {
		t.Fatal(err)
	}
	messages, err := store.MessagesAfter(ctx, "user-a", item.ID, 0)
	if err != nil || len(messages) != 4 {
		t.Fatalf("messages: %+v, %v", messages, err)
	}
	if err := store.UpdateSummary(ctx, "user-a", item.ID, 0, messages[1].ID, "用户叫小海"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSummary(ctx, "user-a", item.ID, 0, messages[3].ID, "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected optimistic conflict, got %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Get(ctx, "user-a", item.ID)
	if err != nil || got.Summary != "用户叫小海" || got.SummaryThroughMessageID != messages[1].ID {
		t.Fatalf("memory did not persist: %+v, %v", got, err)
	}
	active, err := reopened.MessagesAfter(ctx, "user-a", item.ID, got.SummaryThroughMessageID)
	if err != nil || len(active) != 2 || active[0].Content != "我喜欢 Go" {
		t.Fatalf("active messages: %+v, %v", active, err)
	}
	all, err := reopened.MessagesAfter(ctx, "user-a", item.ID, 0)
	if err != nil || len(all) != 4 {
		t.Fatalf("raw messages were deleted: %+v, %v", all, err)
	}
	todos, err := reopened.ListTodos(ctx, "user-a", item.ID)
	if err != nil || len(todos) != 1 || todos[0].Text != "写周报" {
		t.Fatalf("todo did not survive restart: %+v, %v", todos, err)
	}
}

func TestSchemaContainsRequiredTables(t *testing.T) {
	store, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, name := range []string{"sessions", "messages", "todos", "tool_traces"} {
		var found string
		err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&found)
		if err != nil || found != name {
			t.Fatalf("table %q missing: %v", name, err)
		}
	}
}
