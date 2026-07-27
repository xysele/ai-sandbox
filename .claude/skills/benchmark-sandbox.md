---
skill: benchmark-sandbox
description: Benchmark and performance test the AI Sandbox Gateway
---

# Benchmark Sandbox Skill

Use this skill to measure performance and identify bottlenecks in the AI Sandbox Gateway.

## Quick Performance Test

```bash
# Ensure server is running
export GATEWAY_TOKEN="test-token-123"
./sandbox &
SERVER_PID=$!
sleep 2

# Basic latency test
echo "=== Latency Test ==="
for i in {1..10}; do
  time curl -s http://localhost:7860/health > /dev/null
done

# Throughput test
echo -e "\n=== Throughput Test ==="
time for i in {1..100}; do
  curl -s -X POST http://localhost:7860/exec \
    -H "X-Gateway-Token: $GATEWAY_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"command":"echo test"}' > /dev/null &
done
wait

kill $SERVER_PID
```

## Detailed Benchmarks

### 1. Endpoint Latency

Test individual endpoint response times:

```bash
#!/bin/bash
TOKEN="$GATEWAY_TOKEN"
URL="http://localhost:7860"

echo "Endpoint,Min(ms),Max(ms),Avg(ms)"

# Health endpoint
times=()
for i in {1..50}; do
  t=$(curl -o /dev/null -s -w '%{time_total}\n' $URL/health)
  times+=($t)
done
# Calculate stats and print
echo "health,$(echo ${times[@]} | tr ' ' '\n' | sort -n | awk '{sum+=$1} END {print sum/NR*1000}')"

# Exec endpoint
times=()
for i in {1..50}; do
  t=$(curl -o /dev/null -s -w '%{time_total}\n' \
    -X POST $URL/exec \
    -H "X-Gateway-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"command":"echo test"}')
  times+=($t)
done
echo "exec,$(echo ${times[@]} | tr ' ' '\n' | sort -n | awk '{sum+=$1} END {print sum/NR*1000}')"

# File read endpoint
echo "test content" > /tmp/bench_test.txt
times=()
for i in {1..50}; do
  t=$(curl -o /dev/null -s -w '%{time_total}\n' \
    -X POST $URL/read_file \
    -H "X-Gateway-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"path":"/tmp/bench_test.txt"}')
  times+=($t)
done
echo "read_file,$(echo ${times[@]} | tr ' ' '\n' | sort -n | awk '{sum+=$1} END {print sum/NR*1000}')"
```

### 2. Concurrent Requests

Test server performance under load:

```bash
#!/bin/bash
TOKEN="$GATEWAY_TOKEN"
URL="http://localhost:7860"

test_concurrency() {
  local concurrent=$1
  local total=$2
  
  echo "Testing $concurrent concurrent requests ($total total)..."
  
  start=$(date +%s.%N)
  for i in $(seq 1 $total); do
    curl -s -X POST $URL/exec \
      -H "X-Gateway-Token: $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"command":"echo test"}' > /dev/null &
    
    # Control concurrency
    if (( $(jobs -r | wc -l) >= concurrent )); then
      wait -n
    fi
  done
  wait
  end=$(date +%s.%N)
  
  duration=$(echo "$end - $start" | bc)
  rps=$(echo "$total / $duration" | bc -l)
  
  printf "Duration: %.2fs, RPS: %.2f\n" $duration $rps
}

# Test different concurrency levels
test_concurrency 1 50
test_concurrency 5 50
test_concurrency 10 50
test_concurrency 20 50
```

### 3. Memory Usage Over Time

Monitor memory consumption during operations:

```bash
#!/bin/bash

# Start monitoring
monitor_memory() {
  local pid=$1
  local interval=1
  
  echo "Time(s),RSS(KB),VSZ(KB)"
  
  for i in {1..60}; do
    if ! ps -p $pid > /dev/null 2>&1; then
      echo "Process died"
      break
    fi
    
    mem=$(ps -o rss=,vsz= -p $pid)
    echo "$i,$mem" | tr ' ' ','
    sleep $interval
  done
}

# Start server
export GATEWAY_TOKEN="test-token-123"
./sandbox > /dev/null 2>&1 &
SERVER_PID=$!
sleep 2

# Monitor in background
monitor_memory $SERVER_PID > /tmp/memory_profile.csv &
MONITOR_PID=$!

# Generate load
for i in {1..100}; do
  curl -s -X POST http://localhost:7860/exec \
    -H "X-Gateway-Token: $GATEWAY_TOKEN" \
    -d '{"command":"sleep 0.1"}' > /dev/null &
  sleep 0.5
done
wait

# Stop monitoring
kill $MONITOR_PID 2>/dev/null
kill $SERVER_PID

# Show results
echo "Memory profile saved to /tmp/memory_profile.csv"
cat /tmp/memory_profile.csv | tail -10
```

### 4. File Operation Performance

Benchmark file operations at different sizes:

```bash
#!/bin/bash
TOKEN="$GATEWAY_TOKEN"
URL="http://localhost:7860"

echo "Size,Write(ms),Read(ms)"

for size in 1K 10K 100K 1M 10M; do
  # Generate test file
  dd if=/dev/urandom of=/tmp/bench_$size.dat bs=$size count=1 2>/dev/null
  content=$(base64 < /tmp/bench_$size.dat)
  
  # Measure write
  write_start=$(date +%s.%N)
  curl -s -X POST $URL/write_file \
    -H "X-Gateway-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"path\":\"/tmp/test_$size.dat\",\"content\":\"$content\",\"base64\":true}" > /dev/null
  write_end=$(date +%s.%N)
  write_time=$(echo "($write_end - $write_start) * 1000" | bc)
  
  # Measure read
  read_start=$(date +%s.%N)
  curl -s -X POST $URL/read_file \
    -H "X-Gateway-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"path\":\"/tmp/test_$size.dat\"}" > /dev/null
  read_end=$(date +%s.%N)
  read_time=$(echo "($read_end - $read_start) * 1000" | bc)
  
  printf "%s,%.2f,%.2f\n" $size $write_time $read_time
  
  # Cleanup
  rm -f /tmp/bench_$size.dat /tmp/test_$size.dat
done
```

### 5. Task System Performance

Benchmark async task creation and polling:

```bash
#!/bin/bash
TOKEN="$GATEWAY_TOKEN"
URL="http://localhost:7860"

echo "=== Task System Benchmark ==="

# Create multiple tasks
echo "Creating 50 tasks..."
start=$(date +%s.%N)
task_ids=()
for i in {1..50}; do
  response=$(curl -s -X POST $URL/task/create \
    -H "X-Gateway-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"command\":\"sleep 1 && echo task $i done\"}")
  task_id=$(echo $response | jq -r '.task_id')
  task_ids+=($task_id)
done
create_end=$(date +%s.%N)
create_time=$(echo "$create_end - $start" | bc)
printf "Task creation: %.2fs (%.2f tasks/sec)\n" $create_time $(echo "50 / $create_time" | bc -l)

# Poll tasks until complete
echo "Polling tasks..."
poll_count=0
while true; do
  all_done=true
  for task_id in "${task_ids[@]}"; do
    status=$(curl -s "$URL/task/status?task_id=$task_id" \
      -H "X-Gateway-Token: $TOKEN" | jq -r '.status')
    poll_count=$((poll_count + 1))
    if [[ "$status" != "completed" && "$status" != "failed" ]]; then
      all_done=false
    fi
  done
  
  if $all_done; then
    break
  fi
  sleep 0.5
done
poll_end=$(date +%s.%N)
poll_time=$(echo "$poll_end - $create_end" | bc)
printf "Task completion: %.2fs\n" $poll_time
printf "Total polls: %d\n" $poll_count
```

## Load Testing with Apache Bench (ab)

If you have `ab` installed:

```bash
# Simple GET request load test
ab -n 1000 -c 10 http://localhost:7860/health

# POST request load test (requires file with JSON payload)
echo '{"command":"echo test"}' > /tmp/payload.json
ab -n 1000 -c 10 -p /tmp/payload.json \
  -T 'application/json' \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  http://localhost:7860/exec
```

## Load Testing with wrk

If you have `wrk` installed:

```bash
# Install wrk (if needed)
# macOS: brew install wrk
# Linux: git clone https://github.com/wg/wrk && cd wrk && make

# Basic load test
wrk -t4 -c100 -d30s http://localhost:7860/health

# POST request with Lua script
cat > post.lua << 'EOF'
wrk.method = "POST"
wrk.body   = '{"command":"echo test"}'
wrk.headers["Content-Type"] = "application/json"
wrk.headers["X-Gateway-Token"] = os.getenv("GATEWAY_TOKEN")
EOF

wrk -t4 -c100 -d30s -s post.lua http://localhost:7860/exec
```

## Performance Profiling

### CPU Profiling

```bash
# Install pprof (if needed)
go install github.com/google/pprof@latest

# Add to main.go (for development only):
# import _ "net/http/pprof"
# go func() { http.ListenAndServe("localhost:6060", nil) }()

# Capture 30s CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# Analyze
go tool pprof cpu.prof
# Commands: top, list, web
```

### Memory Profiling

```bash
# Capture heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof

# Analyze
go tool pprof heap.prof
```

## Performance Expectations

Based on typical hardware (4 CPU cores, 8GB RAM):

| Endpoint | Expected Latency | Expected RPS |
|----------|-----------------|--------------|
| /health | 1-5ms | 1000+ |
| /exec (echo) | 10-50ms | 100-200 |
| /read_file (1KB) | 5-20ms | 200-500 |
| /write_file (1KB) | 10-30ms | 100-200 |
| /batch (2 ops) | 20-100ms | 50-100 |
| /task/create | 5-15ms | 200-500 |

Memory usage should remain under 100MB for typical workloads.

## Optimization Tips

If performance is below expectations:

1. **Check system resources**
   ```bash
   top
   iostat
   netstat -s
   ```

2. **Reduce logging** - Remove debug logs in hot paths

3. **Tune task limits** - Adjust maxTasks in tasks.go if needed

4. **Use batch endpoint** - Reduce HTTP overhead for multiple operations

5. **Enable HTTP keep-alive** - Client should reuse connections

6. **Profile and optimize** - Use pprof to find bottlenecks
