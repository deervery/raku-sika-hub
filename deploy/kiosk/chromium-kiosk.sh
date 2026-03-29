#!/bin/bash
# Chromium kiosk launcher for raku-sika-hub station
# Waits for hub to be ready, then launches Chromium in kiosk mode
# pointing to https://rakusika.com
#
# Hub URL (http://localhost:19800) is set via localStorage in the browser.
# HTTPS -> http://localhost is allowed by W3C Secure Context spec.

set -uo pipefail

KIOSK_URL="${KIOSK_URL:-https://rakusika.com}"
HUB_HEALTH_URL="http://localhost:19800/health"
MAX_WAIT=60

# --- Wayland session environment ---
export WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
export DBUS_SESSION_BUS_ADDRESS="${DBUS_SESSION_BUS_ADDRESS:-unix:path=/run/user/$(id -u)/bus}"

# --- Wait for hub to be ready ---
echo "[kiosk] Waiting for hub at ${HUB_HEALTH_URL} ..."
for i in $(seq 1 "$MAX_WAIT"); do
    if curl -sf --max-time 3 "$HUB_HEALTH_URL" >/dev/null 2>&1; then
        echo "[kiosk] Hub is ready (${i}s)"
        break
    fi
    if [ "$i" -eq "$MAX_WAIT" ]; then
        echo "[kiosk] WARNING: Hub not ready after ${MAX_WAIT}s, launching browser anyway"
    fi
    sleep 1
done

# --- Disable screen blanking (X11) ---
if command -v xset >/dev/null 2>&1; then
    xset s off
    xset s noblank
    xset -dpms
fi

# --- Disable screen blanking (Wayland/wlr) ---
if [ -n "${WAYLAND_DISPLAY:-}" ]; then
    export WLR_NO_HARDWARE_CURSORS=1
fi

# --- Clean up Chromium crash state ---
CHROMIUM_DIR="${HOME}/.config/chromium/Default"
mkdir -p "$CHROMIUM_DIR"
# Remove crash recovery flags to prevent "restore pages" dialog
sed -i 's/"exited_cleanly":false/"exited_cleanly":true/' \
    "${CHROMIUM_DIR}/Preferences" 2>/dev/null || true
sed -i 's/"exit_type":"Crashed"/"exit_type":"Normal"/' \
    "${CHROMIUM_DIR}/Preferences" 2>/dev/null || true

# --- Launch Chromium ---
echo "[kiosk] Launching Chromium: ${KIOSK_URL}"
# Virtual keyboard is handled by raku-sika-lite (JS in-page keyboard).
# No OS virtual keyboard (wvkbd/squeekboard) needed.
# --kiosk is safe to use since the keyboard is inside the page.
exec chromium \
    --kiosk \
    --noerrdialogs \
    --disable-infobars \
    --disable-session-crashed-bubble \
    --disable-features=TranslateUI \
    --check-for-update-interval=31536000 \
    --autoplay-policy=no-user-gesture-required \
    --disable-pinch \
    --overscroll-history-navigation=0 \
    --no-first-run \
    --disable-restore-session-state \
    --disable-component-update \
    --disable-background-networking \
    --disable-sync \
    --disable-default-apps \
    --disk-cache-dir=/tmp/chromium-cache \
    --disk-cache-size=52428800 \
    --password-store=basic \
    --ozone-platform=wayland \
    --lang=ja \
    "$KIOSK_URL"
