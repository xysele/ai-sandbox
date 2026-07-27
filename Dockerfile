FROM golang:1.21-bullseye AS builder

WORKDIR /build

# Copy go module files
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o ai-sandbox-go -ldflags="-w -s" .

# Final stage
FROM debian:bullseye-slim

# Install virtual desktop, GUI automation tools, and development tools
#   xvfb       - Virtual frame buffer (X server without display)
#   fluxbox    - Lightweight window manager
#   xdotool    - Mouse/keyboard automation (click, type, press key)
#   scrot      - Screenshot tool
#   imagemagick - Backup screenshot/image processing
#   git        - Version control
#   python3    - Python runtime
#   python3-pip - Python package manager
#   nodejs/npm - JavaScript runtime
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
        python3 \
        python3-pip \
        nodejs \
        npm \
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
    rm -rf /root/.cache

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ai-sandbox-go .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /app/ai-sandbox-go

ENV DISPLAY=:99 \
    PORT=7860 \
    SCREENSHOT_DIR=/tmp/cs_screenshots

EXPOSE 7860

ENTRYPOINT ["/entrypoint.sh"]
