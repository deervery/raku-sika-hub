#!/bin/bash
set -e

# raku-sika-hub deployment setup for Raspberry Pi
# Run as: sudo bash deploy/setup.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== raku-sika-hub deploy setup ==="

# 1. Set hostname to raku-sika-hub (enables raku-sika-hub.local via mDNS)
CURRENT_HOSTNAME=$(hostname)
if [ "$CURRENT_HOSTNAME" != "raku-sika-hub" ]; then
    echo "[1/4] Setting hostname to raku-sika-hub (current: $CURRENT_HOSTNAME)"
    hostnamectl set-hostname raku-sika-hub
    # Update /etc/hosts
    sed -i "s/127\.0\.1\.1.*$/127.0.1.1\traku-sika-hub/" /etc/hosts
    echo "  → hostname set. raku-sika-hub.local will resolve after avahi restarts."
else
    echo "[1/4] Hostname already set to raku-sika-hub"
fi

# 2. Ensure avahi-daemon is installed and running
echo "[2/4] Setting up avahi-daemon (mDNS)"
if ! command -v avahi-daemon &>/dev/null; then
    apt-get update -qq && apt-get install -y -qq avahi-daemon
fi
cp "$SCRIPT_DIR/avahi/raku-sika-hub.service" /etc/avahi/services/
systemctl enable avahi-daemon
systemctl restart avahi-daemon
echo "  → avahi-daemon configured. raku-sika-hub.local is now resolvable."

# 3. Serial port permission
echo "[3/4] Adding user to dialout group (serial port access)"
DEPLOY_USER="${SUDO_USER:-rakusika}"
if id -nG "$DEPLOY_USER" | grep -qw dialout; then
    echo "  → $DEPLOY_USER is already in dialout group"
else
    usermod -aG dialout "$DEPLOY_USER"
    echo "  → Added $DEPLOY_USER to dialout group (re-login required)"
fi

# 4. Install systemd service
echo "[4/4] Installing systemd service"
cat > /etc/systemd/system/raku-sika-hub.service << EOF
[Unit]
Description=RakuSika Hub - Scale & Printer Gateway
After=network.target avahi-daemon.service

[Service]
Type=simple
User=$DEPLOY_USER
WorkingDirectory=$PROJECT_DIR
ExecStart=$PROJECT_DIR/raku-sika-hub
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable raku-sika-hub
echo "  → systemd service installed and enabled"

echo ""
echo "=== Setup complete ==="
echo "  Hostname:  raku-sika-hub"
echo "  mDNS:      raku-sika-hub.local"
echo "  WebSocket: ws://raku-sika-hub.local:19800"
echo ""
echo "Next steps:"
echo "  1. Build: go build -o raku-sika-hub . (or copy pre-built binary)"
echo "  2. Start: sudo systemctl start raku-sika-hub"
echo "  3. Check: curl http://raku-sika-hub.local:19800/"
echo "  4. Logs:  journalctl -u raku-sika-hub -f"
