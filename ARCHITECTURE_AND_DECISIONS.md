# Architecture and Solution Decisions

## Purpose

This document captures the current architecture of the TODO web app and the main solution decisions made so far, including why each decision was selected.

## System Architecture

## High-level components

- Go HTTP server in `main.go`
- In-memory domain store (`Store`) with mutex-based concurrency safety
- Application layer (`App`) that owns route wiring and request handlers
- Embedded frontend (HTML/CSS/JS) served by the Go backend
- Lightweight environment configuration via `.env`
- CI/CD pipelines via GitHub Actions

## Runtime flow

1. Process starts and loads `.env` using `loadDotEnv(".env")`.
2. `newApp(store)` creates the app with injected store dependency.
3. `app.routes()` registers all HTTP routes on `http.ServeMux`.
4. Server listens on `PORT` (default `8080`).
5. Browser frontend calls REST endpoints:
   - `GET /api/todos`
   - `POST /api/todos`
   - `PATCH /api/todos/{id}/toggle`
   - `DELETE /api/todos/{id}`

## Data model

`Todo` fields:

- `ID` (string)
- `Text` (string)
- `Done` (bool)
- `Tab` (`work` or `private`)
- `Deadline` (string date, optional)
- `CreatedAt` (time)

State is process-local only (not persisted across restarts).

## Key Solution Decisions

## 1) Single-binary backend + embedded frontend

- Decision: keep frontend embedded in `main.go` and serve from Go routes.
- Why: simplest deployment and fastest local iteration.
- Trade-off: frontend code is less modular than a separate SPA project.

## 2) In-memory store with `sync.RWMutex`

- Decision: use in-memory slice with mutex locking.
- Why: minimal complexity for current scope (YAGNI), good enough for demo/local use.
- Trade-off: data is lost on restart and does not scale horizontally.

## 3) App layer and handler separation

- Decision: introduce `App` struct and handler methods (`handleAddTodo`, etc.).
- Why: improves SRP/SOLID, testability, and readability.
- Trade-off: slight increase in boilerplate.

## 4) Tab normalization helper

- Decision: centralize tab validation in `normalizeTab`.
- Why: removes duplicated validation logic (DRY).
- Trade-off: none significant.

## 5) Local `.env` support without third-party dependency

- Decision: implement custom `loadDotEnv` instead of adding dotenv library.
- Why: keeps dependency footprint near zero and meets current needs.
- Trade-off: parser is intentionally basic (simple `KEY=VALUE` lines).

## 6) Mobile-ready web app via PWA basics

- Decision: add manifest, service worker, viewport/meta, responsive CSS.
- Why: supports phone usage and home-screen install quickly.
- Trade-off: offline behavior is basic cache fallback, not full offline-first sync.

## 7) CI/CD with GitHub Actions

- Decision:
  - CI on push/PR for formatting + tests
  - CD on version tags to produce platform binaries
- Why: automation for quality gates and reproducible delivery artifacts.
- Trade-off: release artifacts are workflow artifacts unless extended to GitHub Releases.

## Testing Strategy

- Unit tests for store behavior and helper behavior in `main_test.go`
- HTTP lifecycle test using `httptest` and isolated store instance
- `.env` loading test for configuration correctness

Current scope intentionally avoids E2E browser automation and database integration tests.

## Constraints and Assumptions

- Single-process runtime
- No external DB
- No authentication/authorization
- Optimized for clarity and maintainability over feature depth

## Next recommended decisions

- Persistence: choose JSON file vs SQLite for todo durability
- Release process: attach CD binaries to GitHub Releases
- Observability: add structured request logging and basic health endpoint
