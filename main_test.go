package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStoreAddListToggleDelete(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	deadline := "2030-12-25"

	// Expected behavior: Add creates an unfinished todo with requested fields.
	added := s.Add("Write tests", TabWork, deadline)
	if added.Text != "Write tests" {
		t.Fatalf("expected text Write tests, got %q", added.Text)
	}
	if added.Tab != TabWork {
		t.Fatalf("expected tab %q, got %q", TabWork, added.Tab)
	}
	if added.Deadline != deadline {
		t.Fatalf("expected deadline %q, got %q", deadline, added.Deadline)
	}
	if added.Done {
		t.Fatal("expected new todo to be not done")
	}
	if added.ID == "" {
		t.Fatal("expected generated ID")
	}

	// Expected behavior: List(tab) only returns todos from that tab.
	workTodos := s.List(TabWork)
	if len(workTodos) != 1 {
		t.Fatalf("expected 1 work todo, got %d", len(workTodos))
	}
	privateTodos := s.List(TabPrivate)
	if len(privateTodos) != 0 {
		t.Fatalf("expected 0 private todos, got %d", len(privateTodos))
	}

	// Expected behavior: Toggle flips done status for existing todo.
	if ok := s.Toggle(added.ID); !ok {
		t.Fatal("expected toggle to succeed for existing ID")
	}
	if !s.List(TabWork)[0].Done {
		t.Fatal("expected todo to be done after toggle")
	}

	// Expected behavior: Delete removes existing todo and returns true.
	if ok := s.Delete(added.ID); !ok {
		t.Fatal("expected delete to succeed for existing ID")
	}
	if len(s.List(TabWork)) != 0 {
		t.Fatalf("expected empty list after delete, got %d item(s)", len(s.List(TabWork)))
	}
}

func TestStoreOperationsNotFound(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	if s.Toggle("missing-id") {
		t.Fatal("expected toggle to fail for missing ID")
	}
	if s.Delete("missing-id") {
		t.Fatal("expected delete to fail for missing ID")
	}
}

func TestJSONResponseSetsHeadersStatusAndBody(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	payload := map[string]any{"ok": true}

	// Expected behavior: helper writes JSON content type, status, and valid body.
	jsonResponse(rr, http.StatusCreated, payload)

	if got := rr.Code; got != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected content type application/json, got %q", got)
	}

	var body map[string]bool
	if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&body); err != nil {
		t.Fatalf("expected valid json body, decode error: %v", err)
	}
	if !body["ok"] {
		t.Fatal("expected body.ok to be true")
	}
}

func TestLocalIPv4ReturnsOnlyIPv4(t *testing.T) {
	t.Parallel()

	// Expected behavior: any returned address is a valid IPv4 string.
	ips := localIPv4()
	for _, ip := range ips {
		parsed := strings.Split(ip, ".")
		if len(parsed) != 4 {
			t.Fatalf("expected IPv4 format, got %q", ip)
		}
	}
}

func TestStoreCreatedAtSet(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	before := time.Now().Add(-1 * time.Second)
	item := s.Add("Check created time", TabPrivate, "")
	after := time.Now().Add(1 * time.Second)

	// Expected behavior: CreatedAt is set to current server time.
	if item.CreatedAt.Before(before) || item.CreatedAt.After(after) {
		t.Fatalf("expected CreatedAt around now, got %v", item.CreatedAt)
	}
}
