#!/bin/bash
# Health check script for watchdog daemon
# Exit non-zero = unhealthy -> watchdog will reboot the system
#
# Used by: /etc/watchdog.conf (test-binary)

# Check hub is responding
if ! curl -sf --max-time 5 http://localhost:19800/health >/dev/null 2>&1; then
    echo "[healthcheck] hub is not responding" >&2
    exit 1
fi

# Check chromium is running (kiosk mode)
if ! pgrep -f "chromium.*--kiosk" >/dev/null 2>&1; then
    echo "[healthcheck] chromium kiosk is not running" >&2
    exit 1
fi

exit 0
