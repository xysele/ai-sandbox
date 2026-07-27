---
name: ai-sandbox-gateway
description: Operate a deployed AI Sandbox Gateway over HTTP. Use when an agent needs to execute shell commands or code in a remote sandbox, read or write sandbox files, run and monitor long tasks, inspect sandbox resources, make network requests from the sandbox, transfer binary files, or automate a browser or virtual desktop through this gateway. This skill is for consuming the gateway API, not maintaining its Go implementation.
---

# AI Sandbox Gateway

Use the gateway as a privileged remote execution environment. Keep every action within the user's requested sandbox and inspect every structured response before continuing.

## Configure access

Obtain the deployed base URL and token from the user, task context, or environment. Prefer these variables:

```bash
export AI_SANDBOX_URL="https://host.example/proxy/7860"
export GATEWAY_TOKEN="..."
```

Never print, commit, or place the token in a URL. Prefer `GATEWAY_TOKEN` over the helper's `--token` option so the secret does not appear in process arguments. Preserve any path prefix in the base URL.

Set the skill location when invoking its helper from outside this directory:

```bash
SKILL_DIR=".agents/skills/ai-sandbox-gateway"
python3 "$SKILL_DIR/scripts/gateway_call.py" health
```

`/health` is public and reports runtime and GUI readiness. Treat a successful health check as connectivity evidence only; make one harmless authenticated call such as `/system/info` when token validity also needs checking.

## Choose the operation

- Use `/exec` for a short shell command that needs pipes, redirection, `&&`, a working directory, or environment variables.
- Use `/run_code` for multiline Python, Bash, Node.js, or Go code. Check `/health` first because an advertised language can be absent from a customized image.
- Use `/task/create` for work likely to exceed 30 seconds or whose output must be polled or cancellation must remain possible. Do not use `/exec` background mode when a final result is needed.
- Use file endpoints for structured reads, writes, searches, and binary transfer. Prefer `/write_files` for a generated multi-file tree, but do not treat its rollback as a transaction.
- Use `/http_fetch` when the request must originate inside the sandbox and structured status, headers, and body matter.
- Use `/browser/*` for a single stateless Playwright action against a URL. Each call opens a fresh browser, so state, cookies, clicks, and typed text do not persist into another call.
- Use GUI endpoints for stateful coordinate-based interaction with the shared virtual desktop. Check `gui_ready` before using them and take a screenshot before choosing coordinates.
- Use `/batch` only for independent or fixed-path POST operations. Batch results cannot be interpolated into later request bodies.

Read [references/api.md](references/api.md) before using an unfamiliar endpoint or constructing a request beyond the examples below.

## Call the gateway

Use the bundled standard-library helper to avoid hand-written authentication headers and JSON escaping:

```bash
python3 "$SKILL_DIR/scripts/gateway_call.py" call /exec \
  --data '{"command":"pwd && ls -la","timeout":30}'

python3 "$SKILL_DIR/scripts/gateway_call.py" task \
  --timeout 600 --interval 2 'npm ci && npm test'

python3 "$SKILL_DIR/scripts/gateway_call.py" upload ./input.bin /tmp/job/input.bin
python3 "$SKILL_DIR/scripts/gateway_call.py" download /tmp/job/result.bin ./result.bin
```

For large or quote-heavy JSON, use `--data-file`:

```bash
python3 "$SKILL_DIR/scripts/gateway_call.py" call /run_code --data-file request.json
```

The helper prints response JSON to stdout, diagnostics to stderr, and exits nonzero for HTTP errors, transport errors, `success: false`, or failed/cancelled tasks. It accepts `SANDBOX_BASE_URL` as a fallback alias for `AI_SANDBOX_URL`.

Use direct HTTP only when the helper is unavailable:

```bash
curl -sS "$AI_SANDBOX_URL/exec" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"command":"uname -a"}'
```

## Handle results

Inspect both the HTTP outcome and the JSON body. Most endpoints return `success`; `/health` returns `ok`. A `200` response does not guarantee the requested operation succeeded: `/exec` reports nonzero commands with `success: false`, and `/http_fetch` reports the upstream status separately as `status_code`.

For asynchronous work, poll until `completed`, `failed`, or `cancelled`. Use `stream=true` only when intermediate output is useful. Finished tasks remain available for about 30 minutes and the table holds at most 200 tasks.

Verify important side effects with a read, listing, status query, or screenshot. Retry only transient connection and 5xx failures. Do not blindly retry writes, appends, batch operations, clicks, task creation, or command execution.

## Protect the sandbox

- Treat `/exec`, `/run_code`, file writes, and browser evaluation as arbitrary remote code execution.
- Resolve exact paths before overwrite, recursive deletion, process termination, package installation, or broad shell operations. Ask before destructive work not explicitly authorized.
- Use a dedicated working directory for generated artifacts. Avoid modifying system paths or unrelated user files.
- Keep output bounded. Command stdout and stderr are each capped at 4 MiB; process large data inside the sandbox and retrieve only the result.
- Never assume filesystem isolation inside the container. The file API accepts absolute paths, and `/delete_file` recursively removes its target.
- Cancel tasks that the user explicitly no longer needs. Do not abandon repeated polling without reporting the task ID and current state.
