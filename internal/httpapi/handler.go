package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"demoagent/internal/agent"
	"demoagent/internal/session"
)

type SessionStore interface {
	Ping(context.Context) error
	Create(context.Context, string) (session.Session, error)
	Get(context.Context, string, string) (session.Session, error)
	ListTraces(context.Context, string, string) ([]session.ToolTrace, error)
}

func NewHandler(store SessionStore, runner *agent.Runner) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_unavailable", "database is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /sessions", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			UserID string `json:"user_id"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if strings.TrimSpace(body.UserID) == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "user_id is required")
			return
		}
		created, err := store.Create(r.Context(), body.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", "could not create session")
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})
	mux.HandleFunc("GET /sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "user_id is required")
			return
		}
		item, err := store.Get(r.Context(), userID, r.PathValue("id"))
		if errors.Is(err, session.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", "could not read session")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("POST /sessions/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			UserID  string `json:"user_id"`
			Message string `json:"message"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if strings.TrimSpace(body.UserID) == "" || strings.TrimSpace(body.Message) == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "user_id and message are required")
			return
		}
		if runner == nil {
			writeError(w, http.StatusServiceUnavailable, "llm_not_configured", "Agent is not configured")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()
		result, err := runner.Run(ctx, body.UserID, r.PathValue("id"), body.Message)
		if err != nil {
			switch {
			case errors.Is(err, session.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "session not found")
			case errors.Is(err, agent.ErrNotConfigured):
				writeError(w, http.StatusServiceUnavailable, "llm_not_configured", err.Error())
			case errors.Is(err, agent.ErrEmptyMessage), errors.Is(err, agent.ErrMessageTooLong):
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			case errors.Is(err, agent.ErrLimit):
				writeError(w, http.StatusUnprocessableEntity, "agent_limit", err.Error())
			default:
				writeError(w, http.StatusBadGateway, "agent_error", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /sessions/{id}/trace", func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "user_id is required")
			return
		}
		traces, err := store.ListTraces(r.Context(), userID, r.PathValue("id"))
		if errors.Is(err, session.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", "could not read traces")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"traces": traces})
	})
	return mux
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
