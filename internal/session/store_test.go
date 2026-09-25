package session

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
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
