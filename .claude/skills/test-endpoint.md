---
skill: test-endpoint
description: Test an AI Sandbox Gateway endpoint with proper authentication
---

# Test Endpoint Skill

Use this skill to quickly test any endpoint of the AI Sandbox Gateway.

## Usage

When the user asks to test an endpoint, follow these steps:

1. **Check if server is running**
   ```bash
   pgrep -f "./sandbox" || echo "Server not running"
   ```

2. **Get the GATEWAY_TOKEN**
   ```bash
   echo ${GATEWAY_TOKEN:-"test-token-123"}
   ```

3. **Determine the endpoint and method**
   - Most endpoints use POST
   - Public endpoints: GET /, /health
   - UI endpoints: GET /ui, /ui/auth (POST), /ui/logout

4. **Build the curl command**
   - Always include `-H "X-Gateway-Token: $GATEWAY_TOKEN"` except for public endpoints
   - Use `-X POST` for non-GET requests
   - Use `-H "Content-Type: application/json"` for JSON payloads
   - Pretty print with `| jq .` if output is JSON

5. **Test the endpoint**

## Examples

### Test exec endpoint
```bash
curl -s -X POST "http://localhost:7860/exec" \
  -H "X-Gateway-Token: ${GATEWAY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"command":"echo hello world"}' | jq .
```

### Test batch endpoint
```bash
curl -s -X POST "http://localhost:7860/batch" \
  -H "X-Gateway-Token: ${GATEWAY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "operations": [
      {"type": "write_file", "params": {"path": "/tmp/test.txt", "content": "hello"}},
      {"type": "read_file", "params": {"path": "/tmp/test.txt"}}
    ]
  }' | jq .
```

### Test task creation and status
```bash
TASK_ID=$(curl -s -X POST "http://localhost:7860/task/create" \
  -H "X-Gateway-Token: ${GATEWAY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"command": "sleep 2 && echo done"}' | jq -r '.task_id')

echo "Task ID: $TASK_ID"
sleep 3

curl -s "http://localhost:7860/task/status?task_id=$TASK_ID" \
  -H "X-Gateway-Token: ${GATEWAY_TOKEN}" | jq .
```

### Test browser status
```bash
curl -s "http://localhost:7860/browser/status" \
  -H "X-Gateway-Token: ${GATEWAY_TOKEN}" | jq .
```

## Common Issues

- **401 Unauthorized**: Check GATEWAY_TOKEN is set and matches server configuration
- **Connection refused**: Server not running, start with `./sandbox`
- **Empty response**: Check if endpoint requires POST method
- **Port conflict**: Default port 7860, check with `lsof -i :7860`
