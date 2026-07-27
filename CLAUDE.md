# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI Sandbox Gateway — a lightweight HTTP service that provides AI agents with command execution, file operations, GUI automation, and async task management. Pure Go implementation with no external dependencies beyond the standard library.

**Key Design Principle**: Keep the API minimal. Any operation achievable with a single shell command (git, DNS lookup, compression, code formatting) should go through `/exec` rather than get its own endpoint. Endpoints exist only for capabilities that shell can't handle well: binary file transfers, GUI automation, cross-request task state, and HTTP calls that need header parsing.

## Architecture

### Core Components

**Server Layer** (`internal/server/server.go`)
- HTTP server with custom `X-Gateway-Token` authentication (not `Authorization`, because ModelScope reverse proxy strips standard auth headers)
- 24 endpoints grouped by function: exec (2), filesystem (7), GUI (6), tasks (4), system/network (2), public (2)
- Auth middleware uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks

**Command Execution** (`internal/handlers/common.go`)
- `runArgv()`: Execute command via argv array (bypasses shell, prevents command injection)
- `runShell()`: Execute via `sh -lc` for operations requiring shell features (pipes, redirects, `&&`)
- `cappedBuffer`: Limits output capture to 4 MiB to prevent memory exhaustion
- All user input (paths, patterns, env vars, code) passes through argv arrays, never string concatenation

**Async Tasks** (`internal/handlers/tasks.go`)
- Background task execution with status tracking for commands that exceed HTTP gateway timeouts (ModelScope ~60s)
- Task table capped at 200 entries with 30-minute retention for completed tasks (prevents OOM in long-running deployments)
- Context-based cancellation (not channels) to avoid lost signals
- Tasks transition: pending → running → completed/failed/cancelled

**Config** (`internal/config/config.go`)
- All config from environment variables
- **Critical**: `GATEWAY_TOKEN` must be set explicitly for production. Random tokens are ephemeral and change on container restart, causing all callers to get 401s
- On startup, logs token source (env/generated) but never the token value or prefix (ModelScope runtime logs are visible to workspace visitors)

### Security Model

1. **Command Injection Prevention**: User input never concatenated into shell strings; always passed via argv arrays
2. **Timing Attack Prevention**: Token comparison uses constant-time compare
3. **Memory Boundaries**: Output 4 MiB, file read/download 32 MiB, HTTP fetch 8 MiB, task table 200 entries
4. **Path Safety**: Rejects deletion of `/` and `.`; error messages hide host absolute paths
5. **Log Safety**: Never logs token content, only source

## Common Commands

### Build
```bash
go build -o sandbox .
```

### Run Locally
```bash
export GATEWAY_TOKEN=your_secret_here
./sandbox
# Service starts on http://localhost:7860
# Web UI available at http://localhost:7860/ui
```

### Docker Build
```bash
docker build -t ai-sandbox-go .
# Note: Docker build includes Playwright/Chromium (~300MB additional size)
```

### Code Quality
```bash
go vet ./...
go fmt ./...
```

### Testing Endpoints
All endpoints except `/`, `/health`, `/favicon.ico`, `/ui*` require the `X-Gateway-Token` header:
```bash
curl -H "X-Gateway-Token: your_secret_here" \
  -X POST http://localhost:7860/exec \
  -d '{"command":"echo hello"}'
```

### Web Management UI
Access the web-based management interface at `http://localhost:7860/ui` and login with your `GATEWAY_TOKEN`. The UI provides:
- System status monitoring
- Task management with live log streaming
- Quick actions for common operations (exec, screenshot, file read/write)
- No separate authentication needed beyond the initial login

## Development Patterns

### Adding a New Endpoint

1. Add handler to `internal/handlers/` (choose appropriate file: exec, filesystem, gui, tasks, network, basic)
2. Register route in `server.registerRoutes()` with appropriate path
3. Use `requirePOST()` for non-GET endpoints
4. Use `respondJSON()` for all responses (never `json.Encode()` directly to ResponseWriter — if encoding fails mid-stream, status code is already sent)
5. Use `runArgv()` when user input is involved, `runShell()` only when shell features (pipes, redirects) are required

### Async vs Sync Execution

- **Sync** (`/exec`): Commands expected to complete within HTTP timeout (~30s)
- **Async** (`/task/create`): Commands that may exceed 30s or need status polling
  - Supports streaming logs: `/task/status?task_id=xxx&stream=true` returns partial output while task is running
- **Background** (`/exec` with `"background": true`): Fire-and-forget services (e.g., `python -m http.server`); output redirected to `/tmp/sandbox_bg.log`
- **Batch** (`/batch`): Execute multiple operations in a single request to reduce HTTP round-trips

### Browser Automation

Browser automation endpoints use Playwright with Chromium headless:
- Each operation launches an ephemeral browser instance (no state persistence between calls)
- Selectors use CSS syntax (e.g., `.login-button`, `input[name='username']`)
- JavaScript execution runs in page context via `/browser/evaluate`
- Screenshots support full page or specific element capture
- Default timeout: 30 seconds per operation

Check availability with `/browser/status` before attempting automation.

### Error Handling

- Use `statError()` for filesystem errors — translates OS errors to stable HTTP status codes and messages
- Never expose internal paths or syscall details in error messages
- Return structured errors via `failure()` helper with optional extra context

### GUI Automation

Requires Xvfb + xdotool + scrot stack (automatically started by `entrypoint.sh`):
- Resolution: 1280x800x24
- Display: `:99` (configurable via `DISPLAY` env var)
- Tools: xdotool (click/type/key), scrot (screenshot), imagemagick (fallback)
- Check tool availability via `/gui_status` before attempting automation

## File Organization

- `main.go` — Entry point: load config, start server
- `internal/config/` — Environment variable loading, ephemeral token generation
- `internal/server/` — HTTP server, routing, auth middleware
- `internal/handlers/` — Request handlers grouped by domain:
  - `common.go` — Shared utilities (runArgv, cappedBuffer, respondJSON)
  - `exec.go` — Command execution (/exec, /run_code)
  - `filesystem.go` — File operations (read/write/delete/list/search/upload/download, batch write)
  - `gui.go` — GUI automation (screenshot/click/type/key/mouse)
  - `tasks.go` — Async tasks (/task/*, /system/info) with streaming log support
  - `network.go` — HTTP proxy (/http_fetch)
  - `basic.go` — Public endpoints (/, /health)
  - `batch.go` — Batch operations (/batch)
  - `browser.go` — Browser automation via Playwright (/browser/*)
  - `ui.go` — Web management interface (/ui, /ui/auth, /ui/logout)
  - `ui_login.go` — Login page HTML
  - `ui_dashboard_*.go` — Dashboard HTML split across multiple files

## Deployment Notes

### ModelScope Cloud Studio

- Port **must** be 7860 (platform requirement)
- Set `GATEWAY_TOKEN` as environment variable in workspace settings (otherwise it regenerates on every restart)
- Reverse proxy strips `Authorization` headers, hence custom `X-Gateway-Token` header
- Runtime logs visible to workspace visitors — never log sensitive values

### Docker Container

Container includes full development toolchain:
- Languages: python3, nodejs, go, bash
- GUI: Xvfb, fluxbox, xdotool, scrot, imagemagick
- Tools: git, curl, wget, vim, zip/unzip, tar, gzip, net-tools, dnsutils
- Python packages: requests, numpy, pandas

Entrypoint script (`entrypoint.sh`) starts Xvfb → fluxbox → Go service in sequence.

## Language & Comments

Codebase uses Chinese comments internally. When adding new code, match the existing comment style of the file you're editing.
