package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("session not found")
var ErrConflict = errors.New("session memory changed concurrently")

//go:embed schema.sql
var schema embed.FS

type Session struct {
	ID                      string    `json:"session_id"`
	UserID                  string    `json:"user_id"`
	Summary                 string    `json:"summary"`
	SummaryThroughMessageID int64     `json:"summary_through_message_id"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path must not be empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection keeps SQLite PRAGMAs consistent and avoids write contention.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if path != ":memory:" {
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("enable sqlite WAL: %w", err)
		}
	}
	ddl, err := schema.ReadFile("schema.sql")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("read schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(ddl)); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := migrateSessionMemory(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrateSessionMemory(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(sessions)")
	if err != nil {
		return fmt.Errorf("inspect sessions schema: %w", err)
	}
	var hasBoundary bool
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan sessions schema: %w", err)
		}
		if name == "summary_through_message_id" {
			hasBoundary = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read sessions schema: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close sessions schema query: %w", err)
	}
	if !hasBoundary {
		if _, err := db.ExecContext(ctx, "ALTER TABLE sessions ADD COLUMN summary_through_message_id INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add session summary boundary: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) Create(ctx context.Context, userID string) (Session, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Session{}, errors.New("user_id must not be empty")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Session{}, fmt.Errorf("generate session id: %w", err)
	}
	now := time.Now().UTC()
	item := Session{
		ID:        hex.EncodeToString(random[:]),
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions(id, user_id, summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		item.ID, item.UserID, item.Summary, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	return item, nil
}

func (s *Store) Get(ctx context.Context, userID, sessionID string) (Session, error) {
	var item Session
	var created, updated string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, summary, summary_through_message_id, created_at, updated_at FROM sessions WHERE id = ? AND user_id = ?",
		sessionID, userID,
	).Scan(&item.ID, &item.UserID, &item.Summary, &item.SummaryThroughMessageID, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("query session: %w", err)
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Session{}, fmt.Errorf("parse session created_at: %w", err)
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return Session{}, fmt.Errorf("parse session updated_at: %w", err)
	}
	return item, nil
}
