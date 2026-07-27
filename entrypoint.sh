#!/bin/bash
set -e

# Start virtual Linux desktop (Xvfb) + window manager (fluxbox)
# Resolution 1280x800 for screenshot/mouse/keyboard automation
export DISPLAY=:99

if command -v Xvfb >/dev/null 2>&1; then
    Xvfb :99 -screen 0 1280x800x24 -ac +extension RANDR >/tmp/xvfb.log 2>&1 &
    XPID=$!
    # Wait for X server to be ready
    for i in $(seq 1 20); do
        if [ -e /tmp/.X11-unix/X99 ]; then break; fi
        sleep 0.2
    done
    echo "[entrypoint] Xvfb started (pid=$XPID)"
else
    echo "[entrypoint] WARNING: Xvfb not found, GUI automation will be unavailable" >&2
fi

if command -v fluxbox >/dev/null 2>&1; then
    fluxbox >/tmp/fluxbox.log 2>&1 &
    echo "[entrypoint] fluxbox started (pid=$!)"
fi

# Start the Go sandbox gateway (port 7860 for ModelScope Cloud Studio)
exec /app/ai-sandbox-go
