---
skill: monitor-sandbox
description: Monitor the AI Sandbox Gateway in production with health checks and alerts
---

# Monitor Sandbox Skill

Use this skill to set up monitoring, health checks, and alerting for the AI Sandbox Gateway in production.

## Quick Health Check

```bash
# Basic health check
curl -s http://localhost:7860/health | jq .

# Expected response
{
  "status": "ok",
  "uptime_seconds": 12345
}
```

## Monitoring Components

### 1. HTTP Health Endpoint

The gateway provides a `/health` endpoint that returns:

```bash
curl -s http://localhost:7860/health | jq .
```

Response fields:
- `status`: "ok" if server is responding
- `uptime_seconds`: How long the server has been running

**Use for:**
- Load balancer health checks
- Kubernetes liveness/readiness probes
- Uptime monitoring services

### 2. System Information

```bash
curl -s http://localhost:7860/system_info \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .
```

Response includes:
- OS and architecture
- CPU count
- Memory stats (total, available, used)
- Disk usage
- Process info

**Use for:**
- Capacity planning
- Resource utilization tracking
- Performance debugging

### 3. Task List Monitoring

```bash
curl -s http://localhost:7860/task/list \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .
```

Monitor for:
- Number of running tasks
- Number of pending tasks
- Tasks stuck in running state
- Task failure rate

## Health Check Scripts

### Simple Uptime Monitor

```bash
#!/bin/bash
# save as check_health.sh

URL="http://localhost:7860"
TIMEOUT=5

response=$(curl -s -m $TIMEOUT "$URL/health")
status=$(echo "$response" | jq -r '.status' 2>/dev/null)

if [ "$status" = "ok" ]; then
  echo "OK - Gateway is healthy"
  exit 0
else
  echo "CRITICAL - Gateway is down or unhealthy"
  exit 2
fi
```

### Comprehensive Health Check

```bash
#!/bin/bash
# save as comprehensive_check.sh

TOKEN="${GATEWAY_TOKEN}"
URL="http://localhost:7860"
EXIT_CODE=0

echo "=== AI Sandbox Gateway Health Check ==="
echo "Time: $(date)"
echo ""

# 1. Basic health
echo "1. Health Endpoint..."
health=$(curl -s -m 5 "$URL/health" 2>/dev/null)
if [ $? -eq 0 ] && echo "$health" | jq -e '.status == "ok"' >/dev/null 2>&1; then
  uptime=$(echo "$health" | jq -r '.uptime_seconds')
  echo "   OK - Healthy (uptime: ${uptime}s)"
else
  echo "   FAILED - Cannot reach health endpoint"
  EXIT_CODE=2
fi

# 2. Authentication
echo "2. Authentication..."
auth_test=$(curl -s -m 5 -X POST "$URL/exec" \
  -H "X-Gateway-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command":"echo test"}' 2>/dev/null)
if echo "$auth_test" | jq -e '.success == true' >/dev/null 2>&1; then
  echo "   OK - Authentication working"
else
  echo "   FAILED - Authentication error"
  EXIT_CODE=2
fi

# 3. System resources
echo "3. System Resources..."
sysinfo=$(curl -s -m 5 "$URL/system_info" \
  -H "X-Gateway-Token: $TOKEN" 2>/dev/null)
if [ $? -eq 0 ]; then
  mem_used=$(echo "$sysinfo" | jq -r '.memory.used_percent // 0')
  disk_used=$(echo "$sysinfo" | jq -r '.disk.used_percent // 0')
  
  echo "   Memory: ${mem_used}% used"
  echo "   Disk: ${disk_used}% used"
  
  if (( $(echo "$mem_used > 90" | bc -l 2>/dev/null || echo 0) )); then
    echo "   WARNING - Memory usage above 90%"
    EXIT_CODE=1
  fi
  
  if (( $(echo "$disk_used > 90" | bc -l 2>/dev/null || echo 0) )); then
    echo "   WARNING - Disk usage above 90%"
    EXIT_CODE=1
  fi
else
  echo "   FAILED - Cannot get system info"
  EXIT_CODE=2
fi

# 4. Active tasks
echo "4. Active Tasks..."
tasks=$(curl -s -m 5 "$URL/task/list" \
  -H "X-Gateway-Token: $TOKEN" 2>/dev/null)
if [ $? -eq 0 ]; then
  running=$(echo "$tasks" | jq '[.[] | select(.status=="running")] | length')
  pending=$(echo "$tasks" | jq '[.[] | select(.status=="pending")] | length')
  
  echo "   Running: $running"
  echo "   Pending: $pending"
  
  if [ "$running" -gt 50 ]; then
    echo "   WARNING - High number of running tasks"
    EXIT_CODE=1
  fi
else
  echo "   FAILED - Cannot get task list"
  EXIT_CODE=2
fi

echo ""
if [ $EXIT_CODE -eq 0 ]; then
  echo "Status: ALL CHECKS PASSED"
elif [ $EXIT_CODE -eq 1 ]; then
  echo "Status: WARNING - Some issues detected"
else
  echo "Status: CRITICAL - Health check failed"
fi

exit $EXIT_CODE
```

## Integration with Monitoring Systems

### Kubernetes Probes

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ai-sandbox
spec:
  containers:
  - name: sandbox
    image: ai-sandbox-go:latest
    ports:
    - containerPort: 7860
    env:
    - name: GATEWAY_TOKEN
      valueFrom:
        secretKeyRef:
          name: sandbox-secrets
          key: token
    
    # Liveness probe - restart if failing
    livenessProbe:
      httpGet:
        path: /health
        port: 7860
      initialDelaySeconds: 10
      periodSeconds: 30
      timeoutSeconds: 5
      failureThreshold: 3
    
    # Readiness probe - remove from service if failing
    readinessProbe:
      httpGet:
        path: /health
        port: 7860
      initialDelaySeconds: 5
      periodSeconds: 10
      timeoutSeconds: 3
      failureThreshold: 2
    
    # Resource limits
    resources:
      requests:
        memory: "256Mi"
        cpu: "250m"
      limits:
        memory: "1Gi"
        cpu: "1000m"
```

### Docker Health Check

```dockerfile
FROM golang:1.21-alpine
COPY . /app
WORKDIR /app
RUN go build -o sandbox .

EXPOSE 7860

# Built-in health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://localhost:7860/health || exit 1

CMD ["./sandbox"]
```

Or in docker-compose.yml:

```yaml
version: '3.8'
services:
  sandbox:
    build: .
    ports:
      - "7860:7860"
    environment:
      GATEWAY_TOKEN: "${GATEWAY_TOKEN}"
    healthcheck:
      test: ["CMD", "wget", "-q", "-O-", "http://localhost:7860/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

### Nagios/Icinga Check

```bash
#!/bin/bash
# save as check_sandbox_nagios.sh

URL="$1"
TOKEN="$2"

if [ -z "$URL" ] || [ -z "$TOKEN" ]; then
  echo "UNKNOWN - Usage: $0 <url> <token>"
  exit 3
fi

health=$(curl -s -m 10 "$URL/health" 2>/dev/null)

if [ $? -ne 0 ]; then
  echo "CRITICAL - Cannot connect to $URL"
  exit 2
fi

status=$(echo "$health" | jq -r '.status' 2>/dev/null)

if [ "$status" = "ok" ]; then
  uptime=$(echo "$health" | jq -r '.uptime_seconds')
  echo "OK - Gateway healthy (uptime: ${uptime}s)"
  exit 0
else
  echo "CRITICAL - Health check failed"
  exit 2
fi
```

## Alert Configuration

### Slack Webhook

```bash
#!/bin/bash
# send_slack_alert.sh

WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
MESSAGE="$1"

curl -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "{\"text\":\"$MESSAGE\"}"
```

### Email Alerts

```bash
#!/bin/bash
# send_alert.sh

RECIPIENT="admin@example.com"
SUBJECT="AI Sandbox Gateway Alert"
MESSAGE="$1"

echo "$MESSAGE" | mail -s "$SUBJECT" "$RECIPIENT"
```

## Performance Metrics to Monitor

| Metric | Threshold | Action |
|--------|-----------|--------|
| Response time (p95) | > 1000ms | Investigate slow endpoints |
| Memory usage | > 80% | Check for memory leaks |
| CPU usage | > 80% | Check for CPU-intensive tasks |
| Active tasks | > 100 | Implement task limits |
| Error rate | > 1% | Check logs for issues |
| Disk usage | > 90% | Clean up temporary files |
| Uptime | < 99% | Investigate crashes |

## Automated Monitoring Script

```bash
#!/bin/bash
# continuous_monitor.sh - Run in background

URL="http://localhost:7860"
TOKEN="$GATEWAY_TOKEN"
CHECK_INTERVAL=300  # 5 minutes
LOG_FILE="/var/log/sandbox_monitor.log"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

while true; do
  # Health check
  health=$(curl -s -m 10 "$URL/health" 2>/dev/null)
  
  if [ $? -eq 0 ]; then
    status=$(echo "$health" | jq -r '.status')
    if [ "$status" = "ok" ]; then
      log "Health check passed"
    else
      log "Health check failed: $health"
    fi
  else
    log "Cannot connect to gateway"
  fi
  
  sleep "$CHECK_INTERVAL"
done
```

Run in background:
```bash
nohup ./continuous_monitor.sh &
```

## Best Practices

1. **Multiple check types** - Use both health endpoint and functional tests
2. **Appropriate intervals** - 30-60s for critical services, 5-10min for non-critical
3. **Redundant monitoring** - Use multiple monitoring systems
4. **Alert fatigue** - Set reasonable thresholds to avoid false positives
5. **Escalation** - Critical alerts go to on-call, warnings to team channel
6. **Dashboards** - Visual dashboards for at-a-glance status
7. **Log retention** - Keep logs for at least 30 days
8. **Regular testing** - Test alert mechanisms monthly

## Troubleshooting Monitors

If monitors are failing but service seems fine:

1. **Check network** - Can monitoring system reach gateway?
2. **Check timeouts** - Are timeouts too aggressive?
3. **Check credentials** - Is GATEWAY_TOKEN correct?
4. **Check thresholds** - Are thresholds too strict?
5. **Check monitor itself** - Is monitoring script/service working?
