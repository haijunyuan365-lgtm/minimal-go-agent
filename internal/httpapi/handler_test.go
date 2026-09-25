package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"demoagent/internal/session"
)

func TestSessionRoutes(t *testing.T) {
	store, err := session.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := NewHandler(store)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d: %s", health.Code, health.Body.String())
	}

	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString(`{"user_id":"user-a"}`)))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}
	var item session.Session
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ID == "" {
		t.Fatal("empty session id")
	}

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/sessions/"+item.ID+"?user_id=user-a", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", read.Code, read.Body.String())
	}
	wrongUser := httptest.NewRecorder()
	handler.ServeHTTP(wrongUser, httptest.NewRequest(http.MethodGet, "/sessions/"+item.ID+"?user_id=user-b", nil))
	if wrongUser.Code != http.StatusNotFound {
		t.Fatalf("wrong user status = %d", wrongUser.Code)
	}

	chat := httptest.NewRecorder()
	handler.ServeHTTP(chat, httptest.NewRequest(http.MethodPost, "/sessions/"+item.ID+"/messages", bytes.NewBufferString(`{}`)))
	if chat.Code != http.StatusNotImplemented {
		t.Fatalf("phase-one chat status = %d", chat.Code)
	}
}

func TestCreateSessionRejectsInvalidJSON(t *testing.T) {
	store, err := session.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, body := range []string{`{}`, `{"user_id":"a","unexpected":true}`, `{"user_id":"a"}{"user_id":"b"}`} {
		recorder := httptest.NewRecorder()
		NewHandler(store).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewBufferString(body)))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %q returned %d", body, recorder.Code)
		}
	}
}
