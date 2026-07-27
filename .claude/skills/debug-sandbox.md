---
skill: debug-sandbox
description: Debug issues with the AI Sandbox Gateway
---

# Debug Sandbox Skill

Use this skill to diagnose and fix common issues with the AI Sandbox Gateway.

## Quick Diagnostic Commands

```bash
# Check if server is running
pgrep -f "./sandbox" || echo "Server not running"

# Check port availability
lsof -i :7860 || echo "Port 7860 is available"

# Check environment variables
echo "GATEWAY_TOKEN: ${GATEWAY_TOKEN:+SET}"
echo "PORT: ${PORT:-7860}"
echo "DISPLAY: ${DISPLAY:-not set}"

# Test compilation
go build -o sandbox . && echo "Build successful" || echo "Build failed"

# Check for syntax errors
go vet ./... 2>&1 | head -20
```

## Common Issues

### Issue 1: 401 Unauthorized

**Symptoms:**
```json
{"error":"unauthorized","success":false}
```

**Diagnosis:**
```bash
# Check if GATEWAY_TOKEN is set
echo "Token set: ${GATEWAY_TOKEN:+YES}"

# Check server logs for token source
ps aux | grep sandbox

# Try with explicit token
curl -H "X-Gateway-Token: test-token-123" http://localhost:7860/health
```

**Solutions:**

1. **Set GATEWAY_TOKEN before starting**
   ```bash
   export GATEWAY_TOKEN="your-secret-token"
   ./sandbox
   ```

2. **Check token matches between client and server**
   ```bash
   # Server side
   echo $GATEWAY_TOKEN
   
   # Client request must match
   curl -H "X-Gateway-Token: $GATEWAY_TOKEN" ...
   ```

3. **Look for token in server startup logs**
   ```
   [main] Auth: GATEWAY_TOKEN loaded from environment
   ```

### Issue 2: Connection Refused

**Symptoms:**
```
curl: (7) Failed to connect to localhost port 7860: Connection refused
```

**Diagnosis:**
```bash
# Is server running?
pgrep -f "./sandbox"

# Is port open?
lsof -i :7860

# Check if binary exists
ls -lh sandbox

# Check for port conflicts
netstat -tuln | grep 7860
```

**Solutions:**

1. **Start the server**
   ```bash
   export GATEWAY_TOKEN="test-token"
   ./sandbox
   ```

2. **Use different port if 7860 is taken**
   ```bash
   export PORT=8080
   ./sandbox
   ```

3. **Rebuild if binary is missing**
   ```bash
   go build -o sandbox .
   ```

### Issue 3: Command Execution Fails

**Symptoms:**
```json
{"error":"command failed","success":false,"stderr":"..."}
```

**Diagnosis:**
```bash
# Test command locally first
echo "test" > /tmp/test.txt
cat /tmp/test.txt

# Check command syntax
curl -s -X POST http://localhost:7860/exec \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command":"pwd"}' | jq .
```

**Solutions:**

1. **Check command syntax**
   ```bash
   # Use shell features with /exec
   curl -X POST http://localhost:7860/exec \
     -H "X-Gateway-Token: $GATEWAY_TOKEN" \
     -d '{"command":"echo hello && pwd"}'
   ```

2. **Check file paths are absolute**
   ```bash
   # BAD - relative path
   {"command": "cat test.txt"}
   
   # GOOD - absolute path
   {"command": "cat /tmp/test.txt"}
   ```

3. **Check for permission issues**
   ```bash
   # Test file permissions
   ls -la /path/to/file
   
   # Use appropriate user
   whoami
   ```

### Issue 4: Build Errors

**Symptoms:**
```
undefined: base64.DecodeString
import cycle not allowed
```

**Diagnosis:**
```bash
# Check Go version
go version

# Check imports
grep -r "import" internal/handlers/*.go | head -20

# Run verbose build
go build -v -o sandbox .
```

**Solutions:**

1. **Fix import issues**
   ```bash
   # Run go mod tidy
   go mod tidy
   
   # Check for import cycles
   go build -v ./... 2>&1 | grep cycle
   ```

2. **Fix base64 usage**
   ```go
   // Use StdEncoding explicitly
   decoded, err := base64.StdEncoding.DecodeString(encoded)
   ```

3. **Clean build cache**
   ```bash
   go clean -cache
   go build -o sandbox .
   ```

### Issue 5: GUI Automation Fails

**Symptoms:**
```json
{"error":"GUI tools not available","success":false}
```

**Diagnosis:**
```bash
# Check GUI status
curl -s http://localhost:7860/gui_status \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .

# Check for required tools
which xdotool scrot Xvfb

# Check DISPLAY variable
echo $DISPLAY

# Check if Xvfb is running
pgrep Xvfb
```

**Solutions:**

1. **Install required tools**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install xvfb xdotool scrot imagemagick
   
   # macOS (limited support)
   brew install xdotool
   ```

2. **Start Xvfb**
   ```bash
   export DISPLAY=:99
   Xvfb :99 -screen 0 1280x800x24 &
   ```

3. **Use Docker image with GUI tools**
   ```bash
   docker run -p 7860:7860 -e GATEWAY_TOKEN="token" ai-sandbox-go
   ```

### Issue 6: Browser Automation Fails

**Symptoms:**
```json
{"error":"Cannot find module 'playwright-chromium'","success":false}
```

**Diagnosis:**
```bash
# Check browser status
curl -s http://localhost:7860/browser/status \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .

# Check if playwright is installed
which playwright

# Check npm global packages
npm list -g --depth=0 | grep playwright
```

**Solutions:**

1. **Install playwright-chromium**
   ```bash
   npm install -g playwright-chromium
   playwright install chromium --with-deps
   ```

2. **Check Node.js version**
   ```bash
   node --version  # Should be v14+
   ```

3. **Set NODE_PATH if needed**
   ```bash
   export NODE_PATH=$(npm root -g)
   ```

### Issue 7: Task Stuck in Pending

**Symptoms:**
```json
{"status":"pending","task_id":"task_1"}
```

**Diagnosis:**
```bash
# Check task status
curl -s "http://localhost:7860/task/status?task_id=task_1" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .

# List all tasks
curl -s "http://localhost:7860/task/list" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .

# Check server logs
tail -f /tmp/sandbox.log  # if logging enabled
```

**Solutions:**

1. **Wait longer** - task may be running
   ```bash
   sleep 5
   curl -s "http://localhost:7860/task/status?task_id=task_1" \
     -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .
   ```

2. **Cancel stuck task**
   ```bash
   curl -X POST "http://localhost:7860/task/cancel" \
     -H "X-Gateway-Token: $GATEWAY_TOKEN" \
     -d '{"task_id":"task_1"}'
   ```

3. **Restart server if task system hung**
   ```bash
   pkill -f "./sandbox"
   ./sandbox &
   ```

## Debug Logging

Enable verbose logging:

```bash
# Run server in foreground to see logs
./sandbox

# Or redirect to file
./sandbox > /tmp/sandbox.log 2>&1 &

# Monitor logs
tail -f /tmp/sandbox.log
```

## Performance Issues

### High Memory Usage

```bash
# Check memory usage
ps aux | grep sandbox

# Check number of active tasks
curl -s "http://localhost:7860/task/list" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq 'length'

# Restart if memory is high
pkill -f "./sandbox"
./sandbox &
```

### Slow Response Times

```bash
# Test response time
time curl -s http://localhost:7860/health

# Check system load
uptime

# Check for long-running tasks
curl -s "http://localhost:7860/task/list" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | \
  jq '.[] | select(.status=="running")'
```

## Testing Tools

### Quick Health Check Script

```bash
#!/bin/bash
TOKEN="${GATEWAY_TOKEN:-test-token-123}"
URL="http://localhost:7860"

echo "=== Health Check ==="
curl -s $URL/health | jq .

echo -e "\n=== Auth Test ==="
curl -s -X POST $URL/exec \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"command":"echo test"}' | jq .success

echo -e "\n=== GUI Status ==="
curl -s $URL/gui_status \
  -H "X-Gateway-Token: $TOKEN" | jq .available

echo -e "\n=== Browser Status ==="
curl -s $URL/browser/status \
  -H "X-Gateway-Token: $TOKEN" | jq .available
```

### Comprehensive Test

```bash
# Save as test_all.sh
bash .claude/skills/../test_new_features.sh
```

## Getting Help

If issue persists:

1. Check CLAUDE.md for architecture details
2. Read error messages carefully - they often include hints
3. Check logs for stack traces
4. Verify all prerequisites are installed
5. Try minimal reproduction case
6. Check if issue happens with fresh build: `go clean && go build`
