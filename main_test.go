package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestAPITodoLifecycleViaRoutes(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	app := newApp(s)
	server := httptest.NewServer(app.routes())
	defer server.Close()

	createBody := `{"text":"Ship feature","tab":"private","deadline":"2031-01-01"}`
	createRes, err := http.Post(server.URL+"/api/todos", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createRes.Body.Close()
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on create, got %d", createRes.StatusCode)
	}
	var created Todo
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode created todo failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created todo ID")
	}

	listRes, err := http.Get(server.URL + "/api/todos?tab=private")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d", listRes.StatusCode)
	}
	var listed []Todo
	if err := json.NewDecoder(listRes.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed todos failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed todo, got %d", len(listed))
	}

	toggleReq, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/todos/"+created.ID+"/toggle", nil)
	toggleRes, err := http.DefaultClient.Do(toggleReq)
	if err != nil {
		t.Fatalf("toggle request failed: %v", err)
	}
	defer toggleRes.Body.Close()
	if toggleRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on toggle, got %d", toggleRes.StatusCode)
	}

	deleteReq, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/todos/"+created.ID, nil)
	deleteRes, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer deleteRes.Body.Close()
	if deleteRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", deleteRes.StatusCode)
	}
}

func TestLoadDotEnvAppliesFileValues(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	content := bytes.NewBufferString("")
	content.WriteString("# comment\n")
	content.WriteString("PORT=9091\n")
	content.WriteString("EXTRA_FLAG=yes\n")
	if err := os.WriteFile(envPath, content.Bytes(), 0644); err != nil {
		t.Fatalf("write temp env file failed: %v", err)
	}

	t.Setenv("PORT", "")
	t.Setenv("EXTRA_FLAG", "")
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("EXTRA_FLAG")

	loadDotEnv(envPath)

	if got := os.Getenv("PORT"); got != "9091" {
		t.Fatalf("expected PORT=9091, got %q", got)
	}
	if got := os.Getenv("EXTRA_FLAG"); got != "yes" {
		t.Fatalf("expected EXTRA_FLAG=yes, got %q", got)
	}
}

func TestListTodosReturnsETagAndSupportsNotModified(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	created := s.Add("ETag check", TabWork, "")
	if created.ID == "" {
		t.Fatal("expected created todo ID")
	}

	app := newApp(s)
	server := httptest.NewServer(app.routes())
	defer server.Close()

	firstRes, err := http.Get(server.URL + "/api/todos?tab=work")
	if err != nil {
		t.Fatalf("first list request failed: %v", err)
	}
	defer firstRes.Body.Close()
	if firstRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from first list request, got %d", firstRes.StatusCode)
	}
	etag := firstRes.Header.Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on list response")
	}

	secondReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/todos?tab=work", nil)
	secondReq.Header.Set("If-None-Match", etag)
	secondRes, err := http.DefaultClient.Do(secondReq)
	if err != nil {
		t.Fatalf("second list request failed: %v", err)
	}
	defer secondRes.Body.Close()
	if secondRes.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304 when ETag matches, got %d", secondRes.StatusCode)
	}
}

func TestAddTodoRejectsInvalidDeadline(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	app := newApp(s)
	server := httptest.NewServer(app.routes())
	defer server.Close()

	body := `{"text":"invalid date","tab":"work","deadline":"2026/31/12"}`
	res, err := http.Post(server.URL+"/api/todos", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid deadline, got %d", res.StatusCode)
	}
}

func TestAddTodoAcceptsValidDeadline(t *testing.T) {
	t.Parallel()

	s := &Store{todos: []Todo{}}
	app := newApp(s)
	server := httptest.NewServer(app.routes())
	defer server.Close()

	body := `{"text":"valid date","tab":"work","deadline":"2026-12-31"}`
	res, err := http.Post(server.URL+"/api/todos", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for valid deadline, got %d", res.StatusCode)
	}
}
