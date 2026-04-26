package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_ "embed"
)

type Tab string

const (
	TabWork    Tab = "work"
	TabPrivate Tab = "private"
)

type Todo struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	Tab       Tab       `json:"tab"`
	Deadline  string    `json:"deadline"` // "YYYY-MM-DD" or ""
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	mu    sync.RWMutex
	todos []Todo
}

var store = &Store{todos: []Todo{
	{ID: "1", Text: "Finish Q3 report", Done: false, Tab: TabWork, Deadline: time.Now().AddDate(0, 0, 2).Format("2006-01-02"), CreatedAt: time.Now()},
	{ID: "2", Text: "Team standup prep", Done: true, Tab: TabWork, Deadline: "", CreatedAt: time.Now()},
	{ID: "3", Text: "Buy groceries", Done: false, Tab: TabPrivate, Deadline: time.Now().AddDate(0, 0, 1).Format("2006-01-02"), CreatedAt: time.Now()},
	{ID: "4", Text: "Call mom", Done: false, Tab: TabPrivate, Deadline: "", CreatedAt: time.Now()},
}}

func (s *Store) List(tab Tab) []Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Todo
	for _, t := range s.todos {
		if t.Tab == tab {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) Add(text string, tab Tab, deadline string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := Todo{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Text:      text,
		Done:      false,
		Tab:       tab,
		Deadline:  deadline,
		CreatedAt: time.Now(),
	}
	s.todos = append(s.todos, t)
	return t
}

func (s *Store) Toggle(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.todos {
		if t.ID == id {
			s.todos[i].Done = !s.todos[i].Done
			return true
		}
	}
	return false
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.todos {
		if t.ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return true
		}
	}
	return false
}

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type App struct {
	store *Store
}

func newApp(s *Store) *App {
	return &App{store: s}
}

func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", a.handleIndex)
	mux.HandleFunc("GET /manifest.webmanifest", a.handleManifest)
	mux.HandleFunc("GET /sw.js", a.handleServiceWorker)
	mux.HandleFunc("GET /api/todos", a.handleListTodos)
	mux.HandleFunc("POST /api/todos", a.handleAddTodo)
	mux.HandleFunc("PATCH /api/todos/{id}/toggle", a.handleToggleTodo)
	mux.HandleFunc("DELETE /api/todos/{id}", a.handleDeleteTodo)
	return mux
}

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlPage))
}

func (a *App) handleManifest(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Write([]byte(manifestJSON))
}

func (a *App) handleServiceWorker(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(serviceWorkerJS))
}

func (a *App) handleListTodos(w http.ResponseWriter, r *http.Request) {
	tab := normalizeTab(Tab(r.URL.Query().Get("tab")))
	todos := a.store.List(tab)
	if todos == nil {
		todos = []Todo{}
	}
	etag := makeTodosETag(todos)
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	jsonResponse(w, http.StatusOK, todos)
}

func (a *App) handleAddTodo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text     string `json:"text"`
		Tab      Tab    `json:"tab"`
		Deadline string `json:"deadline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if !isValidDeadline(body.Deadline) {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid deadline"})
		return
	}
	todo := a.store.Add(body.Text, normalizeTab(body.Tab), body.Deadline)
	jsonResponse(w, http.StatusCreated, todo)
}

func (a *App) handleToggleTodo(w http.ResponseWriter, r *http.Request) {
	if !a.store.Toggle(r.PathValue("id")) {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	if !a.store.Delete(r.PathValue("id")) {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func normalizeTab(tab Tab) Tab {
	if tab == TabWork || tab == TabPrivate {
		return tab
	}
	return TabWork
}

func makeTodosETag(todos []Todo) string {
	payload, _ := json.Marshal(todos)
	sum := sha256.Sum256(payload)
	return "\"" + hex.EncodeToString(sum[:]) + "\""
}

func isValidDeadline(deadline string) bool {
	if deadline == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", deadline)
	return err == nil
}

func main() {
	loadDotEnv(".env")

	app := newApp(store)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Todo app running at http://localhost%s", addr)
	for _, ip := range localIPv4() {
		log.Printf("On your phone (same Wi-Fi): http://%s:%s", ip, port)
	}
	log.Fatal(http.ListenAndServe(addr, app.routes()))
}

const htmlPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="theme-color" content="#0e0e0f">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
<link rel="manifest" href="/manifest.webmanifest">
<title>TŌDŌ</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=DM+Serif+Display:ital@0;1&family=DM+Mono:wght@300;400;500&display=swap" rel="stylesheet">
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  :root {
    --bg: #0e0e0f;
    --surface: #161618;
    --border: #2a2a2e;
    --text: #e8e6e0;
    --muted: #5a5a62;
    --accent-work: #c8a96e;
    --accent-priv: #7eb8c9;
    --danger: #d97060;
    --done: #3d3d42;
    --done-text: #555560;
    --radius: 12px;
    --radius-sm: 6px;
    --tab-h: 48px;
  }

  body {
    background: var(--bg);
    color: var(--text);
    font-family: 'DM Mono', monospace;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 48px 20px 80px;
  }

  /* Grain overlay */
  body::before {
    content: '';
    position: fixed;
    inset: 0;
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.04'/%3E%3C/svg%3E");
    pointer-events: none;
    z-index: 0;
  }

  .app { width: 100%; max-width: 560px; position: relative; z-index: 1; }

  /* Header */
  header {
    display: flex;
    align-items: baseline;
    gap: 16px;
    margin-bottom: 36px;
  }

  h1 {
    font-family: 'DM Serif Display', serif;
    font-size: 3rem;
    letter-spacing: -1px;
    color: var(--text);
    line-height: 1;
  }

  .header-tag {
    font-size: 0.65rem;
    letter-spacing: 0.15em;
    text-transform: uppercase;
    color: var(--muted);
    padding: 3px 8px;
    border: 1px solid var(--border);
    border-radius: 100px;
  }

  /* Tabs */
  .tabs {
    display: flex;
    gap: 2px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 4px;
    margin-bottom: 24px;
  }

  .tab {
    flex: 1;
    height: var(--tab-h);
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--muted);
    font-family: 'DM Mono', monospace;
    font-size: 0.75rem;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .tab:hover { color: var(--text); }

  .tab.active[data-tab="work"] {
    background: color-mix(in srgb, var(--accent-work) 12%, transparent);
    color: var(--accent-work);
    border: 1px solid color-mix(in srgb, var(--accent-work) 30%, transparent);
  }

  .tab.active[data-tab="private"] {
    background: color-mix(in srgb, var(--accent-priv) 12%, transparent);
    color: var(--accent-priv);
    border: 1px solid color-mix(in srgb, var(--accent-priv) 30%, transparent);
  }

  .tab-dot {
    width: 7px; height: 7px;
    border-radius: 50%;
    background: currentColor;
    opacity: 0.7;
  }

  /* Stats bar */
  .stats {
    display: flex;
    gap: 20px;
    margin-bottom: 20px;
    font-size: 0.7rem;
    color: var(--muted);
    letter-spacing: 0.05em;
  }

  .stats span { display: flex; align-items: center; gap: 5px; }
  .stats strong { color: var(--text); }

  /* Add form */
  .add-form {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px;
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .add-row { display: flex; gap: 8px; }

  .add-form input[type="text"] {
    flex: 1;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 10px 14px;
    color: var(--text);
    font-family: 'DM Mono', monospace;
    font-size: 0.82rem;
    outline: none;
    transition: border-color 0.2s;
  }

  .add-form input[type="text"]:focus { border-color: var(--muted); }

  .add-form input[type="date"] {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 10px 12px;
    color: var(--muted);
    font-family: 'DM Mono', monospace;
    font-size: 0.78rem;
    outline: none;
    cursor: pointer;
    transition: border-color 0.2s;
    color-scheme: dark;
  }

  .add-form input[type="date"]:focus { border-color: var(--muted); }

  .btn-add {
    background: var(--text);
    border: none;
    border-radius: var(--radius-sm);
    padding: 10px 18px;
    color: var(--bg);
    font-family: 'DM Mono', monospace;
    font-size: 0.78rem;
    font-weight: 500;
    letter-spacing: 0.05em;
    cursor: pointer;
    transition: opacity 0.15s, transform 0.1s;
    white-space: nowrap;
  }

  .btn-add:hover { opacity: 0.85; }
  .btn-add:active { transform: scale(0.97); }

  /* Todo list */
  .list { display: flex; flex-direction: column; gap: 6px; }

  .todo-item {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 14px 16px;
    display: flex;
    align-items: center;
    gap: 14px;
    transition: border-color 0.2s, opacity 0.2s;
    animation: slideIn 0.2s ease;
  }

  @keyframes slideIn {
    from { opacity: 0; transform: translateY(-6px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .todo-item:hover { border-color: var(--muted); }
  .todo-item.done { opacity: 0.45; }

  /* Checkbox */
  .check {
    width: 20px; height: 20px;
    border-radius: 50%;
    border: 2px solid var(--border);
    background: transparent;
    cursor: pointer;
    flex-shrink: 0;
    transition: all 0.15s;
    display: flex; align-items: center; justify-content: center;
    position: relative;
  }

  .check:hover { border-color: var(--muted); }

  .todo-item[data-tab="work"] .check.checked {
    background: var(--accent-work);
    border-color: var(--accent-work);
  }
  .todo-item[data-tab="private"] .check.checked {
    background: var(--accent-priv);
    border-color: var(--accent-priv);
  }

  .check.checked::after {
    content: '';
    width: 5px; height: 9px;
    border-right: 2px solid var(--bg);
    border-bottom: 2px solid var(--bg);
    transform: rotate(45deg) translate(-1px, -1px);
  }

  /* Todo text */
  .todo-body { flex: 1; min-width: 0; }

  .todo-text {
    font-size: 0.85rem;
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .done .todo-text { text-decoration: line-through; color: var(--done-text); }

  .deadline {
    font-size: 0.65rem;
    color: var(--muted);
    letter-spacing: 0.04em;
    margin-top: 3px;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .deadline.overdue { color: var(--danger); }
  .deadline.soon { color: var(--accent-work); }

  /* Delete btn */
  .btn-del {
    background: transparent;
    border: none;
    color: var(--muted);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    opacity: 0;
    transition: opacity 0.15s, color 0.15s;
    font-size: 1rem;
    line-height: 1;
    flex-shrink: 0;
  }

  .todo-item:hover .btn-del { opacity: 1; }
  .btn-del:hover { color: var(--danger); }

  /* Empty state */
  .empty {
    text-align: center;
    padding: 48px 20px;
    color: var(--muted);
    font-size: 0.78rem;
    letter-spacing: 0.05em;
    border: 1px dashed var(--border);
    border-radius: var(--radius);
  }

  .empty-icon { font-size: 2rem; margin-bottom: 12px; opacity: 0.4; }

  /* Loading */
  .loading {
    text-align: center;
    padding: 32px;
    color: var(--muted);
    font-size: 0.75rem;
    letter-spacing: 0.1em;
  }

  @media (max-width: 640px) {
    body { padding: 20px 12px 28px; }
    h1 { font-size: 2.25rem; }
    .header-tag { font-size: 0.58rem; }
    .stats { gap: 12px; flex-wrap: wrap; }
    .todo-item { padding: 12px; }
    .btn-del { opacity: 1; }
  }
</style>
</head>
<body>
<div class="app">
  <header>
    <h1>TŌDŌ</h1>
    <span class="header-tag">v1.0</span>
  </header>

  <div class="tabs">
    <button class="tab active" data-tab="work" onclick="switchTab('work')">
      <span class="tab-dot"></span>Work
    </button>
    <button class="tab" data-tab="private" onclick="switchTab('private')">
      <span class="tab-dot"></span>Private
    </button>
  </div>

  <div class="stats" id="stats"></div>

  <div class="add-form">
    <div class="add-row">
      <input type="text" id="new-text" placeholder="New task…" onkeydown="if(event.key==='Enter')addTodo()">
      <input type="date" id="new-deadline">
    </div>
    <div class="add-row">
      <button class="btn-add" onclick="addTodo()" style="width:100%">+ Add task</button>
    </div>
  </div>

  <div class="list" id="list"><div class="loading">loading…</div></div>
</div>

<script>
let currentTab = 'work';
let liveRefreshTimer = null;
let isLoading = false;
let lastListSignature = '';
let lastStatsSignature = '';
const tabETags = { work: '', private: '' };

function todosSignature(todos) {
  return todos.map(t => [
    t.id,
    t.done ? 1 : 0,
    t.tab || '',
    t.deadline || '',
    t.text || ''
  ].join('|')).join('~');
}

function switchTab(tab) {
  currentTab = tab;
  lastListSignature = '';
  lastStatsSignature = '';
  document.querySelectorAll('.tab').forEach(el => {
    el.classList.toggle('active', el.dataset.tab === tab);
  });
  load(true);
}

async function load(force) {
  if (isLoading) return;
  isLoading = true;
  try {
    const headers = {};
    if (!force && tabETags[currentTab]) {
      headers['If-None-Match'] = tabETags[currentTab];
    }
    const res = await fetch('/api/todos?tab=' + currentTab, { cache: 'no-store', headers });
    if (res.status === 304) return;
    if (!res.ok) return;
    tabETags[currentTab] = res.headers.get('ETag') || '';
    const todos = await res.json();
    const sig = todosSignature(todos);
    if (force || sig !== lastListSignature) {
      renderList(todos);
      lastListSignature = sig;
    }
    if (force || sig !== lastStatsSignature) {
      renderStats(todos);
      lastStatsSignature = sig;
    }
  } catch (e) {
    // Keep current UI state if a transient network issue occurs.
  } finally {
    isLoading = false;
  }
}

function renderStats(todos) {
  const done = todos.filter(t => t.done).length;
  const total = todos.length;
  const overdue = todos.filter(t => !t.done && t.deadline && isOverdue(t.deadline)).length;
  document.getElementById('stats').innerHTML =
    '<span>' + done + ' / <strong>' + total + '</strong> done</span>' +
    (overdue ? '<span style="color:var(--danger)">⚠ <strong>' + overdue + '</strong> overdue</span>' : '');
}

function isOverdue(d) {
  return d && new Date(d) < new Date(new Date().toDateString());
}

function isSoon(d) {
  if (!d) return false;
  const diff = (new Date(d) - new Date(new Date().toDateString())) / 86400000;
  return diff >= 0 && diff <= 2;
}

function fmtDate(d) {
  if (!d) return '';
  const date = new Date(d + 'T00:00:00');
  const today = new Date(new Date().toDateString());
  const diff = Math.round((date - today) / 86400000);
  if (diff === 0) return 'today';
  if (diff === 1) return 'tomorrow';
  if (diff === -1) return 'yesterday';
  if (diff < 0) return diff + ' days ago';
  return '+' + diff + 'd';
}

function renderList(todos) {
  const list = document.getElementById('list');
  if (!todos.length) {
    list.innerHTML = '<div class="empty"><div class="empty-icon">◎</div>Nothing here yet.<br>Add your first task above.</div>';
    return;
  }
  const sorted = [...todos].sort((a,b) => {
    if (a.done !== b.done) return a.done ? 1 : -1;
    if (a.deadline && b.deadline) return a.deadline.localeCompare(b.deadline);
    if (a.deadline) return -1;
    if (b.deadline) return 1;
    return 0;
  });
  list.innerHTML = sorted.map(t => {
    const od = !t.done && isOverdue(t.deadline);
    const soon = !t.done && !od && isSoon(t.deadline);
    const dlClass = od ? 'deadline overdue' : soon ? 'deadline soon' : 'deadline';
    const dlText = t.deadline ? fmtDate(t.deadline) : '';
    const dlIcon = od ? '⚠' : soon ? '◷' : '◈';
    return '<div class="todo-item' + (t.done ? ' done' : '') + '" data-tab="' + t.tab + '" data-id="' + t.id + '">' +
      '<div class="check ' + (t.done ? 'checked' : '') + '" onclick="toggle(\'' + t.id + '\')"></div>' +
      '<div class="todo-body">' +
        '<div class="todo-text">' + esc(t.text) + '</div>' +
        (dlText ? '<div class="' + dlClass + '">' + dlIcon + ' ' + dlText + '</div>' : '') +
      '</div>' +
      '<button class="btn-del" onclick="del(\'' + t.id + '\')" title="Delete">✕</button>' +
    '</div>';
  }).join('');
}

function esc(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

async function addTodo() {
  const text = document.getElementById('new-text').value.trim();
  if (!text) { document.getElementById('new-text').focus(); return; }
  const deadline = document.getElementById('new-deadline').value;
  await fetch('/api/todos', {
    method: 'POST',
    headers: {'Content-Type':'application/json'},
    body: JSON.stringify({ text, tab: currentTab, deadline })
  });
  document.getElementById('new-text').value = '';
  document.getElementById('new-deadline').value = '';
  load(true);
}

async function toggle(id) {
  await fetch('/api/todos/' + id + '/toggle', { method: 'PATCH' });
  load(true);
}

async function del(id) {
  const el = document.querySelector('[data-id="' + id + '"]');
  if (el) { el.style.opacity = '0'; el.style.transform = 'translateX(20px)'; el.style.transition = 'all 0.2s'; }
  setTimeout(async () => {
    await fetch('/api/todos/' + id, { method: 'DELETE' });
    load(true);
  }, 180);
}

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {});
  });
}

function startLiveRefresh() {
  if (liveRefreshTimer) return;
  liveRefreshTimer = setInterval(() => {
    if (!document.hidden) load();
  }, 2000);
}

load(true);
startLiveRefresh();
</script>
</body>
</html>`

const manifestJSON = `{
  "name": "TODO",
  "short_name": "TODO",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#0e0e0f",
  "theme_color": "#0e0e0f"
}`

const serviceWorkerJS = `const CACHE = 'todo-pwa-v1';
const ASSETS = ['/', '/manifest.webmanifest'];

self.addEventListener('install', event => {
  event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(ASSETS)));
  self.skipWaiting();
});

self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', event => {
  const req = event.request;
  if (req.method !== 'GET') return;
  event.respondWith(
    fetch(req).catch(() => caches.match(req).then(r => r || caches.match('/')))
  );
});`

func localIPv4() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
		}
	}
	return ips
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		_ = os.Setenv(key, value)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("warning: failed reading %s: %v", path, err)
	}
}
