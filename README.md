# TODO Web App (Go)

Simple todo web app built with Go (`main.go`) and an embedded HTML/CSS/JS frontend.

## Prerequisites

- Go installed (1.22+ recommended)
- macOS, Linux, or Windows terminal

Check Go:

```bash
go version
```

## Project Setup

1. Clone or open this project folder.
2. Create `.env` in project root (already included in this repo):

```env
PORT=8080
```

3. Make sure `.env` is not committed publicly (already covered by `.gitignore`).

## Run the Project

From project root:

```bash
go run main.go
```

You should see logs like:

- `Todo app running at http://localhost:8080`
- `On your phone (same Wi-Fi): http://<your-local-ip>:8080`

Open in browser:

- Desktop: `http://localhost:8080`
- Phone (same Wi-Fi): `http://<your-local-ip>:8080`

## Run Tests

```bash
go test main.go main_test.go
```

## How to Use

- Switch between `Work` and `Private` tabs.
- Add task text and optional deadline.
- Toggle a task complete using the round check button.
- Delete a task with `✕`.

## Install on Phone (PWA-style)

The app includes `manifest.webmanifest` and `sw.js`, so you can add it to home screen.

- iPhone (Safari): Share -> Add to Home Screen
- Android (Chrome): Menu -> Install app / Add to Home screen

## Common Troubleshooting

- **Port already in use**
  - Change `PORT` in `.env`, then restart:
  - Example: `PORT=9090`
- **Phone cannot open app**
  - Ensure phone and computer are on the same Wi-Fi.
  - Use the exact LAN URL printed by the app.
  - Check firewall allows incoming connections for the Go process.
- **Go command not found**
  - Install Go and reopen terminal.

## File Overview

- `main.go` - server, API routes, and embedded frontend
- `main_test.go` - core behavior tests
- `EXPECTED_BEHAVIOR.md` - expected behavior specification
- `.env` - local runtime configuration
- `.gitignore` - ignored files