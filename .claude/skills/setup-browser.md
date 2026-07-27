---
skill: setup-browser
description: Setup and configure browser automation (Playwright) for the sandbox
---

# Setup Browser Skill

Use this skill to install and configure browser automation capabilities for the AI Sandbox Gateway.

## Quick Setup

### Check Current Status

```bash
# Start server if needed
export GATEWAY_TOKEN="test-token-123"
./sandbox &
sleep 2

# Check browser status
curl -s http://localhost:7860/browser/status \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" | jq .
```

Expected response if not installed:
```json
{
  "available": false,
  "error": "Cannot find module 'playwright-chromium'",
  "hint": "Install with: npm install -g playwright-chromium && playwright install chromium --with-deps"
}
```

## Installation Methods

### Method 1: NPM Global Install (Recommended)

```bash
# Install Node.js if needed
# macOS
brew install node

# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y nodejs npm

# Check version (need v14+)
node --version

# Install playwright-chromium globally
npm install -g playwright-chromium

# Install browser binaries and dependencies
playwright install chromium --with-deps

# Verify installation
which playwright
npm list -g playwright-chromium
```

### Method 2: Local Project Install

```bash
# In project directory
npm init -y
npm install playwright-chromium

# Install browser binaries
npx playwright install chromium --with-deps

# Set NODE_PATH to local node_modules
export NODE_PATH="$PWD/node_modules"
echo 'export NODE_PATH="$PWD/node_modules"' >> ~/.bashrc
```

### Method 3: Docker (Pre-configured)

```bash
# Use Dockerfile that already includes browser setup
docker build -t ai-sandbox-browser .

# Run with browser support
docker run -p 7860:7860 \
  -e GATEWAY_TOKEN="your-token" \
  --shm-size=2g \
  ai-sandbox-browser
```

## Verify Installation

```bash
# Test browser launch
node -e "require('playwright-chromium').chromium.launch().then(b => b.close())"

# Should print nothing if successful

# Test via gateway
curl -s -X POST http://localhost:7860/browser/navigate \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}' | jq .

# Take screenshot
curl -s -X POST http://localhost:7860/browser/screenshot \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"full_page":true}' | jq -r '.screenshot' | base64 -d > /tmp/test.png

# Verify screenshot created
file /tmp/test.png
```

## System Dependencies

### Ubuntu/Debian

```bash
sudo apt-get update
sudo apt-get install -y \
  libnss3 \
  libnspr4 \
  libatk1.0-0 \
  libatk-bridge2.0-0 \
  libcups2 \
  libdrm2 \
  libdbus-1-3 \
  libxkbcommon0 \
  libxcomposite1 \
  libxdamage1 \
  libxfixes3 \
  libxrandr2 \
  libgbm1 \
  libpango-1.0-0 \
  libcairo2 \
  libasound2
```

### CentOS/RHEL

```bash
sudo yum install -y \
  nss \
  nspr \
  atk \
  at-spi2-atk \
  cups-libs \
  libdrm \
  dbus-libs \
  libxkbcommon \
  libXcomposite \
  libXdamage \
  libXfixes \
  libXrandr \
  mesa-libgbm \
  pango \
  cairo \
  alsa-lib
```

### macOS

```bash
# Node.js via Homebrew
brew install node

# Install playwright
npm install -g playwright-chromium
playwright install chromium
```

## Headless Mode Configuration

Playwright runs in headless mode by default, which is perfect for server environments.

### With Display Server (Xvfb)

If you need X11 for other tools:

```bash
# Install Xvfb
sudo apt-get install -y xvfb

# Start virtual display
export DISPLAY=:99
Xvfb :99 -screen 0 1920x1080x24 &

# Now run browser
./sandbox
```

### Without Display Server

Playwright doesn't need X11 - it works headless by default. Just ensure Node.js and playwright-chromium are installed.

## Common Issues

### Issue 1: "Cannot find module 'playwright-chromium'"

**Solution:**
```bash
npm install -g playwright-chromium
playwright install chromium
```

### Issue 2: "browserType.launch: Executable doesn't exist"

**Solution:**
```bash
# Install browser binaries
playwright install chromium --with-deps

# Or specify path explicitly
export PLAYWRIGHT_BROWSERS_PATH=/path/to/browsers
```

### Issue 3: Missing system libraries

**Symptoms:**
```
error while loading shared libraries: libnss3.so
```

**Solution:**
```bash
# Ubuntu/Debian
playwright install-deps chromium

# Or install manually
sudo apt-get install -y libnss3 libnspr4 libatk1.0-0
```

### Issue 4: Permission denied

**Solution:**
```bash
# Fix npm permissions
mkdir -p ~/.npm-global
npm config set prefix '~/.npm-global'
echo 'export PATH=~/.npm-global/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

# Reinstall
npm install -g playwright-chromium
```

### Issue 5: Out of memory in Docker

**Solution:**
```bash
# Increase shared memory
docker run --shm-size=2g ...

# Or use tmpfs
docker run --tmpfs /tmp:exec ...
```

## Testing Browser Features

### Basic Navigation and Screenshot

```bash
TOKEN="$GATEWAY_TOKEN"
URL="http://localhost:7860"

# Navigate to page
curl -s -X POST "$URL/browser/navigate" \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"url":"https://example.com"}'

# Wait for element
curl -s -X POST "$URL/browser/wait_for" \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"selector":"h1"}'

# Screenshot
curl -s -X POST "$URL/browser/screenshot" \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"full_page":true}' | jq -r '.screenshot' | base64 -d > screenshot.png
```

### Interactive Actions

```bash
# Click element
curl -s -X POST "$URL/browser/click" \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"selector":"button.submit"}'

# Type text
curl -s -X POST "$URL/browser/type" \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"selector":"input[name=search]","text":"hello world"}'

# Evaluate JavaScript
curl -s -X POST "$URL/browser/evaluate" \
  -H "X-Gateway-Token: $TOKEN" \
  -d '{"script":"document.title"}'
```

## Performance Tips

1. **Browser lifecycle** - The gateway manages one shared browser instance automatically

2. **Memory usage** - Browser consumes ~100-200MB RAM typically

3. **Speed** - First launch takes 2-3 seconds, subsequent operations are fast

4. **Cleanup** - Browser closes automatically when server stops

## Docker Configuration

Add to Dockerfile for browser support:

```dockerfile
FROM golang:1.21-alpine

# Install Node.js and dependencies
RUN apk add --no-cache nodejs npm chromium nss freetype harfbuzz ca-certificates ttf-freefont

# Set environment
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
ENV PLAYWRIGHT_BROWSERS_PATH=/usr/bin/chromium-browser

# Install playwright-chromium
RUN npm install -g playwright-chromium

# Copy and build Go app
COPY . /app
WORKDIR /app
RUN go build -o sandbox .

EXPOSE 7860
CMD ["./sandbox"]
```

## Updating Playwright

```bash
# Update to latest version
npm update -g playwright-chromium

# Reinstall browsers
playwright install chromium --with-deps

# Verify version
npm list -g playwright-chromium
```

## Uninstalling

```bash
# Remove playwright
npm uninstall -g playwright-chromium

# Remove browser binaries (optional)
rm -rf ~/.cache/ms-playwright

# Or system-wide
sudo rm -rf /usr/local/lib/node_modules/playwright-chromium
```

## Production Checklist

- [ ] Node.js v14+ installed
- [ ] playwright-chromium installed globally or locally
- [ ] Browser binaries downloaded (`playwright install chromium`)
- [ ] System dependencies installed (`--with-deps` flag)
- [ ] Sufficient memory (2GB+ recommended)
- [ ] Shared memory configured in Docker (`--shm-size=2g`)
- [ ] Browser status returns `"available": true`
- [ ] Test screenshot works and produces valid PNG

## Alternative: Use Chromium Directly

If you can't use Playwright, you can call chromium directly via `/exec`:

```bash
# Install chromium
sudo apt-get install -y chromium-browser

# Take screenshot via chromium
curl -X POST http://localhost:7860/exec \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -d '{"command":"chromium-browser --headless --screenshot=/tmp/screenshot.png https://example.com"}'

# Read screenshot
curl -X POST http://localhost:7860/read_file \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -d '{"path":"/tmp/screenshot.png"}' | jq -r '.content' | base64 -d > screenshot.png
```

However, the Playwright integration provides more features (click, type, evaluate, wait) and better API.
