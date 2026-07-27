---
skill: deploy-sandbox
description: Build and deploy the AI Sandbox Gateway to production
---

# Deploy Sandbox Skill

Use this skill to build, test, and deploy the AI Sandbox Gateway.

## Pre-deployment Checklist

1. **Verify code compiles**
   ```bash
   go build -o sandbox .
   ```

2. **Run basic tests**
   ```bash
   # Test compilation of all packages
   go vet ./...
   
   # Check formatting
   go fmt ./...
   ```

3. **Test critical endpoints**
   ```bash
   export GATEWAY_TOKEN="test-deployment-token"
   ./sandbox &
   SERVER_PID=$!
   sleep 2
   
   # Test health
   curl -s http://localhost:7860/health
   
   # Test exec
   curl -s -X POST http://localhost:7860/exec \
     -H "X-Gateway-Token: $GATEWAY_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"command":"echo test"}' | jq .
   
   kill $SERVER_PID
   ```

## Deployment Methods

### Method 1: Docker Deployment

1. **Build Docker image**
   ```bash
   docker build -t ai-sandbox-go:latest .
   ```

2. **Test locally**
   ```bash
   docker run -d --name sandbox-test \
     -p 7860:7860 \
     -e GATEWAY_TOKEN="your-secret-token" \
     ai-sandbox-go:latest
   
   # Test
   curl http://localhost:7860/health
   
   # Clean up
   docker stop sandbox-test && docker rm sandbox-test
   ```

3. **Push to registry** (if using container registry)
   ```bash
   docker tag ai-sandbox-go:latest your-registry/ai-sandbox-go:latest
   docker push your-registry/ai-sandbox-go:latest
   ```

### Method 2: ModelSpace Deployment

1. **Ensure deploy.py is executable**
   ```bash
   chmod +x deploy.py
   ```

2. **Run deployment script**
   ```bash
   python3 deploy.py
   ```

3. **Set environment variables in ModelSpace console**
   - Go to workspace settings
   - Add `GATEWAY_TOKEN` with a strong random value
   - Restart the workspace

### Method 3: Direct Binary Deployment

1. **Build for target platform**
   ```bash
   # For Linux (most common server target)
   GOOS=linux GOARCH=amd64 go build -o sandbox .
   
   # For macOS
   GOOS=darwin GOARCH=amd64 go build -o sandbox .
   ```

2. **Copy to server**
   ```bash
   scp sandbox user@server:/opt/sandbox/
   scp entrypoint.sh user@server:/opt/sandbox/
   ```

3. **Set up systemd service** (on server)
   ```bash
   sudo tee /etc/systemd/system/sandbox.service << 'EOF'
   [Unit]
   Description=AI Sandbox Gateway
   After=network.target
   
   [Service]
   Type=simple
   User=sandbox
   WorkingDirectory=/opt/sandbox
   Environment="GATEWAY_TOKEN=your-secret-token-here"
   Environment="PORT=7860"
   ExecStart=/opt/sandbox/sandbox
   Restart=on-failure
   
   [Install]
   WantedBy=multi-user.target
   EOF
   
   sudo systemctl daemon-reload
   sudo systemctl enable sandbox
   sudo systemctl start sandbox
   ```

## Post-deployment Verification

1. **Check service is running**
   ```bash
   # Docker
   docker ps | grep sandbox
   
   # Systemd
   systemctl status sandbox
   
   # Process
   pgrep -f sandbox
   ```

2. **Test endpoints**
   ```bash
   TOKEN="your-production-token"
   URL="http://your-server:7860"
   
   # Health check
   curl -s $URL/health
   
   # Auth test
   curl -s -X POST $URL/exec \
     -H "X-Gateway-Token: $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"command":"echo production test"}' | jq .
   ```

3. **Check logs**
   ```bash
   # Docker
   docker logs -f sandbox-container
   
   # Systemd
   journalctl -u sandbox -f
   
   # Direct
   tail -f /var/log/sandbox.log
   ```

## Rollback Procedure

If deployment fails:

1. **Docker rollback**
   ```bash
   docker stop sandbox-current
   docker run -d --name sandbox-current \
     -p 7860:7860 \
     -e GATEWAY_TOKEN="$TOKEN" \
     ai-sandbox-go:previous-tag
   ```

2. **Binary rollback**
   ```bash
   sudo systemctl stop sandbox
   sudo cp /opt/sandbox/sandbox.backup /opt/sandbox/sandbox
   sudo systemctl start sandbox
   ```

## Security Checklist

- [ ] GATEWAY_TOKEN is strong (32+ characters, random)
- [ ] GATEWAY_TOKEN is set via environment variable (not hardcoded)
- [ ] Server logs are not publicly accessible
- [ ] Firewall rules allow only necessary ports
- [ ] Service runs as non-root user (if using systemd)
- [ ] HTTPS/TLS configured (if exposing to internet)

## Monitoring

Monitor these metrics post-deployment:

- Service uptime: `curl http://localhost:7860/health`
- Memory usage: `ps aux | grep sandbox`
- Active tasks: `curl -H "X-Gateway-Token: $TOKEN" http://localhost:7860/task/list`
- Error rate in logs: `journalctl -u sandbox | grep -i error`
