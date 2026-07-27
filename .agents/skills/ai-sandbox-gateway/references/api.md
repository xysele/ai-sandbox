# AI Sandbox Gateway API

Use this reference for request fields, response semantics, limits, and endpoint-specific behavior. All bodies are JSON unless stated otherwise.

## Connection contract

- Join an endpoint to the configured base URL without discarding a deployment path prefix. For example, base `https://host/proxy/7860` plus `/exec` becomes `https://host/proxy/7860/exec`.
- On ModelScope, add the account's `studio_token` query parameter to every endpoint, including `/health`. The bundled helper reads it from the JSON credentials file and merges it with any existing query parameters.
- Send `X-Gateway-Token: <token>` on every endpoint except `GET /`, `GET /health`, and `/favicon.ico`.
- Send `Content-Type: application/json` with JSON requests.
- Expect JSON responses. Most use `{"success":true,...}` or `{"success":false,"error":"..."}`.
- Inspect JSON even on HTTP 200. Command failures and some operation failures use a 200 response with `success: false`.
- Expect `401` with `invalid or missing X-Gateway-Token` for missing, stale, or incorrect credentials.
- Treat a ModelScope-shaped `403` as outer Studio authentication failure, not a response from the gateway.
- Treat `404 no such endpoint` as a path/base-URL error rather than success.

## Service and execution

### `GET /`

Return `{"service":"ai-sandbox-go","ok":true}`. No authentication is required.

### `GET /health`

Return `ok`, `display`, `gui_ready`, and a `runtimes` object whose keys can include `python`, `bash`, `node`, and `go`. No authentication is required.

### `POST /exec`

Request:

```json
{
  "command": "pwd && find . -maxdepth 2 -type f",
  "cwd": "/tmp/job",
  "env": {"MODE": "test"},
  "timeout": 30,
  "background": false
}
```

Run `command` through `sh -lc`, so shell operators are active. `cwd`, `env`, `timeout`, and `background` are optional. The default timeout is 30 seconds.

Foreground response fields are `success`, `stdout`, `stderr`, and `exit_code`. Exit code 124 means timeout and 143 means cancellation. Stdout and stderr are independently capped at 4 MiB.

Background mode returns `pid` and `log_file` immediately. All background launches append to `/tmp/sandbox_bg.log`; use tasks instead when output attribution, final status, or cancellation matters.

### `POST /run_code`

Request fields:

- `language`: `python`, `bash`, `node`, or `go`; default `python`
- `code`: required multiline source string
- `timeout`: optional seconds; default 30

Return `success`, `stdout`, `stderr`, `exit_code`, and `language`. The code runs from a temporary file that is removed afterward.

## Asynchronous tasks

### `POST /task/create`

Request:

```json
{
  "command": "npm ci && npm test",
  "cwd": "/tmp/project",
  "env": {"CI": "true"},
  "timeout": 600
}
```

Run through `sh -lc` and return `task_id` plus initial `status`. The default timeout is 300 seconds.

### `GET|POST /task/status`

Use `GET /task/status?task_id=task_1&stream=true`, or POST `{"task_id":"task_1"}`. States are `pending`, `running`, `completed`, `failed`, and `cancelled`.

While active, `stream=true` adds `partial`, `current_stdout`, and `current_stderr`. When finished, the response adds `end_time`, `duration`, and `result` containing the command result fields.

### `POST /task/cancel`

Send `{"task_id":"task_1"}`. Return `status: "cancelling"`; poll status to observe the terminal `cancelled` state.

### `GET|POST /task/list`

Return summarized `tasks` and `count`. Finished tasks are retained for about 30 minutes. The task table holds at most 200 entries and returns 503 when full.

## Files

### `POST /read_file`

Send `path` and optional line-based `offset` and `limit`. Return `content`, byte `size`, and `more_lines`. `offset` is zero-based. Files larger than 32 MiB are rejected before line slicing; process or split them in the sandbox first.

### `POST /write_file`

Send `path` plus either UTF-8 `content` or base64 `content_b64`; optional `append` defaults to false. `content_b64` takes precedence when nonempty. Parent directories are created. Return `bytes_written`.

### `POST /write_files`

Send:

```json
{
  "files": [
    {"path": "/tmp/job/a.txt", "content": "a\n"},
    {"path": "/tmp/job/blob.bin", "content_b64": "AAE="}
  ]
}
```

Accept at most 100 files and about 16 MiB decoded total content. Parent directories are created. On failure, the handler attempts to delete paths already written. This is not a transaction and can destroy an overwritten pre-existing file during rollback; use fresh target paths for important multi-file writes.

### `POST /delete_file`

Send `{"path":"/tmp/job"}`. Recursively remove the target. Only cleaned `/` and `.` are explicitly refused; resolve and verify every other target before calling.

### `POST /list_dir`

Send optional `path` (default `.`) and `show_hidden` (default false). Return entries with `name`, `type`, `size`, and file `modified` Unix time where available.

### `POST /search_files`

Send required `pattern`; optional `path` (default `.`), `glob`, `before_context`, `after_context`, and `max_results` (default 200). The search uses recursive grep and returns raw text in `matches` plus `match_lines`. No matches is a successful response with empty output.

### `POST /upload` and `POST /download`

- Upload: send `path` and required `content_b64`; return `size`.
- Download: send `path`; return `content_b64` and `size`.

Downloads and direct reads reject files over 32 MiB. Compress, split, or summarize them in the sandbox first.

## Network and resources

### `POST /http_fetch`

Send `url` plus optional `method` (default GET), string `headers`, string `body`, and `timeout` (default 30 seconds). Return upstream `status_code`, first-value `headers`, string `body`, byte `size`, and `truncated`. Bodies are capped at 8 MiB. An upstream 4xx/5xx still produces gateway `success: true`; inspect `status_code`.

### `GET|POST /system/info`

Return OS/architecture, Go/runtime statistics, active and total task counts, the 200-task limit, load average, memory summary, and root-filesystem disk summary.

### `POST /batch`

Request:

```json
{
  "operations": [
    {"endpoint": "/write_file", "body": {"path": "/tmp/job/a", "content": "x"}},
    {"endpoint": "/read_file", "body": {"path": "/tmp/job/a"}}
  ],
  "stop_on_error": true
}
```

Accept 1 to 50 operations. Each sub-operation is invoked as POST, receives no prior result interpolation, and gets `_status_code` in its captured result. `/batch` cannot call itself. Set `stop_on_error` explicitly because its omitted value is false. Inspect top-level `success`, `executed`, `failed`, and every item in `results`.

## Virtual desktop GUI

Use these only after `/health` reports `gui_ready: true`. The desktop is shared and stateful.

- `GET /gui_status`: return X server and tool readiness details.
- `POST /screenshot`: accept optional remote `path`; return PNG `base64`, `mime_type`, `size`, and the saved remote path. An empty body is allowed.
- `POST /click`: accept integer `x`, `y`; optional `button` (`left`, `middle`, `right`) and `double`.
- `POST /type_text`: accept required `text`; type into the focused window.
- `POST /press_key`: accept required `key`, such as `Return`, `Escape`, `ctrl+c`, or `alt+Tab`.
- `POST /move_mouse`: accept integer `x` and `y`; move without clicking.

Take a screenshot after navigation and after any action whose outcome matters. Coordinate assumptions can become stale whenever the UI changes.

## Stateless browser automation

Check `GET /browser/status` before use. Every operation launches a new headless Chromium instance, loads the supplied URL, performs one action, and closes the browser. There is no persistent page or session across calls.

- `POST /browser/navigate`: `url`, optional `timeout`; return final `url` and HTTP `status` after network idle.
- `POST /browser/screenshot`: `url`, optional `selector`, `full_page`, `timeout`; return PNG `base64` and `size`.
- `POST /browser/click`: required `url` and CSS `selector`; optional `timeout`. The post-click state is discarded.
- `POST /browser/type`: required `url`, CSS `selector`, and nonempty `text`; optional `timeout`. The post-type state is discarded.
- `POST /browser/evaluate`: required `url` and JavaScript expression/source string `script`; optional `timeout`; return JSON-serializable `result`.
- `POST /browser/wait_for`: required `url` and CSS `selector`; optional `timeout`; wait for the selector after page load.

For a multi-step browser flow, use one custom Playwright script in the sandbox or use the stateful virtual desktop instead of chaining these stateless endpoints.
