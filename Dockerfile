FROM golang:1.21-bullseye AS builder

WORKDIR /build

# Copy go module files
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o ai-sandbox-go -ldflags="-w -s" .

# Final stage. Keep Bullseye for the existing system packages, but use the
# official Node image because Debian Bullseye only provides Node.js 12.
FROM node:20-bullseye-slim

ENV PATH=/usr/local/go/bin:${PATH} \
    NODE_PATH=/usr/local/lib/node_modules \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

# Install virtual desktop, GUI automation tools, and development tools
#   xvfb       - Virtual frame buffer (X server without display)
#   fluxbox    - Lightweight window manager
#   xdotool    - Mouse/keyboard automation (click, type, press key)
#   scrot      - Screenshot tool
#   imagemagick - Backup screenshot/image processing
#   git        - Version control
#   build-essential - C/C++ compiler and build tools for CGO projects
#   python3    - Python runtime
#   python3-pip - Python package manager
#   Node.js/npm are provided by the base image
#   curl/wget  - HTTP clients
RUN apt-get update && apt-get install -y --no-install-recommends \
        xvfb \
        fluxbox \
        xdotool \
        scrot \
        imagemagick \
        curl \
        wget \
        ca-certificates \
        x11-utils \
        git \
        build-essential \
        python3 \
        python3-pip \
        vim \
        nano \
        zip \
        unzip \
        tar \
        gzip \
        procps \
        net-tools \
        dnsutils \
        iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Install common Python packages for AI/data work
RUN pip3 install --no-cache-dir \
        requests \
        numpy \
        pandas

# Install Playwright for browser automation
RUN npm install -g playwright-chromium@1.40.0 && \
    playwright install chromium --with-deps && \
    npm cache clean --force && \
    rm -rf /root/.cache /var/lib/apt/lists/*

WORKDIR /app

# Keep the Go toolchain in the runtime image for remote Go development.
COPY --from=builder /usr/local/go /usr/local/go
COPY --from=builder /build/ai-sandbox-go .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /app/ai-sandbox-go

ENV DISPLAY=:99 \
    PORT=7860 \
    SCREENSHOT_DIR=/tmp/cs_screenshots

EXPOSE 7860

ENTRYPOINT ["/entrypoint.sh"]
