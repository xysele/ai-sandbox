---
skill: add-endpoint
description: Add a new endpoint to the AI Sandbox Gateway
---

# Add Endpoint Skill

Use this skill when adding a new HTTP endpoint to the AI Sandbox Gateway.

## Step-by-Step Process

### 1. Determine Endpoint Category

Choose the appropriate handler file based on functionality:

- **exec.go** — Command execution, code running
- **filesystem.go** — File operations (read/write/delete/list/search)
- **gui.go** — GUI automation (screenshot/click/type)
- **tasks.go** — Async task management, system info
- **network.go** — HTTP operations, network requests
- **browser.go** — Browser automation via Playwright
- **batch.go** — Multi-operation batch processing
- **ui.go** — Web UI pages and API
- **basic.go** — Public endpoints (health, root)

### 2. Add Handler Function

Add to the appropriate file in `internal/handlers/`:

```go
func HandleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // 1. Require POST if modifying state
    if !requirePOST(w, r) {
        return
    }
    
    // 2. Parse request body
    var req struct {
        Param1 string `json:"param1"`
        Param2 int    `json:"param2"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondJSON(w, http.StatusBadRequest, failure("invalid JSON: " + err.Error()))
        return
    }
    
    // 3. Validate input
    if req.Param1 == "" {
        respondJSON(w, http.StatusBadRequest, failure("param1 is required"))
        return
    }
    
    // 4. Execute operation
    // Use runArgv() for user input to prevent command injection
    stdout, stderr, err := runArgv(r.Context(), []string{"command", req.Param1})
    if err != nil {
        respondJSON(w, http.StatusInternalServerError, failure("operation failed", map[string]any{
            "error": err.Error(),
            "stderr": stderr,
        }))
        return
    }
    
    // 5. Return structured response
    respondJSON(w, http.StatusOK, map[string]any{
        "success": true,
        "output":  stdout,
    })
}
```

### 3. Register Route

Add to `registerRoutes()` in `internal/server/server.go`:

```go
// In the appropriate section (exec, filesystem, gui, etc.)
http.HandleFunc("/your_endpoint", s.authMiddleware(handlers.HandleNewEndpoint))
```

### 4. Update Documentation

Add endpoint documentation to appropriate section in CLAUDE.md if it introduces new patterns.

### 5. Test the Endpoint

```bash
# Start server
export GATEWAY_TOKEN="test-token"
go build -o sandbox . && ./sandbox &
SERVER_PID=$!
sleep 2

# Test endpoint
curl -s -X POST "http://localhost:7860/your_endpoint" \
  -H "X-Gateway-Token: test-token" \
  -H "Content-Type: application/json" \
  -d '{"param1":"value","param2":123}' | jq .

# Clean up
kill $SERVER_PID
```

## Best Practices

### Security

1. **Always validate input**
   ```go
   if strings.Contains(path, "..") {
       respondJSON(w, http.StatusBadRequest, failure("path traversal not allowed"))
       return
   }
   ```

2. **Use runArgv() for user input**
   ```go
   // GOOD - prevents command injection
   stdout, stderr, err := runArgv(ctx, []string{"git", "clone", userURL})
   
   // BAD - vulnerable to injection
   cmd := exec.Command("sh", "-c", "git clone " + userURL)
   ```

3. **Use constant-time comparison for secrets**
   ```go
   if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
       return false
   }
   ```

### Error Handling

1. **Use statError() for filesystem operations**
   ```go
   data, err := os.ReadFile(path)
   if err != nil {
       respondJSON(w, statError(err))
       return
   }
   ```

2. **Provide structured error context**
   ```go
   respondJSON(w, http.StatusInternalServerError, failure("operation failed", map[string]any{
       "error":  err.Error(),
       "path":   path,
       "reason": "file not found",
   }))
   ```

3. **Never expose internal paths**
   ```go
   // BAD - exposes internal structure
   failure("cannot read /home/user/.secret/config")
   
   // GOOD - sanitized error
   failure("cannot read configuration file")
   ```

### Response Format

Always use `respondJSON()` with structured responses:

```go
// Success response
respondJSON(w, http.StatusOK, map[string]any{
    "success": true,
    "data":    result,
})

// Error response
respondJSON(w, http.StatusBadRequest, failure("error message"))

// Error with context
respondJSON(w, http.StatusInternalServerError, failure("error message", map[string]any{
    "detail": "additional context",
}))
```

### Memory Limits

Apply appropriate limits based on operation type:

```go
// Command output: 4 MiB
buf := cappedBuffer{max: 4 << 20}

// File read/download: 32 MiB
if stat.Size() > 32<<20 {
    respondJSON(w, http.StatusRequestEntityTooLarge, 
        failure("file too large (max 32 MiB)"))
    return
}

// HTTP fetch: 8 MiB
resp.Body = http.MaxBytesReader(w, resp.Body, 8<<20)
```

## Common Patterns

### Pattern 1: Simple Command Execution

```go
func HandleSimpleExec(w http.ResponseWriter, r *http.Request) {
    if !requirePOST(w, r) {
        return
    }
    
    var req struct {
        Command string `json:"command"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondJSON(w, http.StatusBadRequest, failure("invalid JSON"))
        return
    }
    
    stdout, stderr, err := runShell(r.Context(), req.Command)
    respondJSON(w, http.StatusOK, map[string]any{
        "success": err == nil,
        "stdout":  stdout,
        "stderr":  stderr,
    })
}
```

### Pattern 2: File Operation

```go
func HandleFileOp(w http.ResponseWriter, r *http.Request) {
    if !requirePOST(w, r) {
        return
    }
    
    var req struct {
        Path string `json:"path"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondJSON(w, http.StatusBadRequest, failure("invalid JSON"))
        return
    }
    
    data, err := os.ReadFile(req.Path)
    if err != nil {
        respondJSON(w, statError(err))
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]any{
        "success": true,
        "content": string(data),
    })
}
```

### Pattern 3: Async Task

```go
func HandleAsyncOp(w http.ResponseWriter, r *http.Request) {
    if !requirePOST(w, r) {
        return
    }
    
    var req struct {
        Command string `json:"command"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondJSON(w, http.StatusBadRequest, failure("invalid JSON"))
        return
    }
    
    taskID := createTask(req.Command)
    
    respondJSON(w, http.StatusOK, map[string]any{
        "success": true,
        "task_id": taskID,
        "status":  "pending",
    })
}
```

## Testing Checklist

After adding an endpoint:

- [ ] Builds without errors: `go build -o sandbox .`
- [ ] Passes vet check: `go vet ./...`
- [ ] Formatted correctly: `go fmt ./...`
- [ ] Returns proper status codes (200, 400, 401, 500)
- [ ] Returns structured JSON with `success` field
- [ ] Handles missing parameters gracefully
- [ ] Handles invalid JSON input
- [ ] Requires authentication (unless public endpoint)
- [ ] Uses runArgv() for user input (prevents injection)
- [ ] Applies appropriate memory limits
- [ ] Error messages don't expose internal paths
- [ ] Works with curl/Postman/httpie
