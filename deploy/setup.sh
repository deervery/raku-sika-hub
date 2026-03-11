#!/bin/bash
set -e

# raku-sika-hub deployment setup for Raspberry Pi
# Run as: sudo bash deploy/setup.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== raku-sika-hub deploy setup ==="

DEPLOY_USER="${SUDO_USER:-rakusika}"

# 1. Set hostname to raku-sika-hub (enables raku-sika-hub.local via mDNS)
CURRENT_HOSTNAME=$(hostname)
if [ "$CURRENT_HOSTNAME" != "raku-sika-hub" ]; then
    echo "[1/6] Setting hostname to raku-sika-hub (current: $CURRENT_HOSTNAME)"
    hostnamectl set-hostname raku-sika-hub
    sed -i "s/127\.0\.1\.1.*$/127.0.1.1\traku-sika-hub/" /etc/hosts
    echo "  → hostname set. raku-sika-hub.local will resolve after avahi restarts."
else
    echo "[1/6] Hostname already set to raku-sika-hub"
fi

# 2. Ensure avahi-daemon is installed and running
echo "[2/6] Setting up avahi-daemon (mDNS)"
if ! command -v avahi-daemon &>/dev/null; then
    apt-get update -qq && apt-get install -y -qq avahi-daemon
fi
cp "$SCRIPT_DIR/avahi/raku-sika-hub.service" /etc/avahi/services/
systemctl enable avahi-daemon
systemctl restart avahi-daemon
echo "  → avahi-daemon configured. raku-sika-hub.local is now resolvable."

# 3. Serial port permission
echo "[3/6] Adding $DEPLOY_USER to dialout group (serial port access)"
if id -nG "$DEPLOY_USER" | grep -qw dialout; then
    echo "  → $DEPLOY_USER is already in dialout group"
else
    usermod -aG dialout "$DEPLOY_USER"
    echo "  → Added $DEPLOY_USER to dialout group (re-login required)"
fi

# 4. Install Japanese fonts (required for label image rendering)
echo "[4/6] Installing Japanese fonts"
if fc-list :lang=ja 2>/dev/null | grep -q "."; then
    echo "  → Japanese fonts already installed"
else
    apt-get update -qq && apt-get install -y -qq fonts-noto-cjk
    echo "  → fonts-noto-cjk installed"
fi

# 5. Install CUPS and Brother printer driver
echo "[5/6] Installing CUPS and Brother printer driver"
if command -v lp &>/dev/null && dpkg -l printer-driver-ptouch &>/dev/null 2>&1; then
    echo "  → CUPS and printer-driver-ptouch already installed"
else
    apt-get update -qq && apt-get install -y -qq cups printer-driver-ptouch
    echo "  → CUPS and printer-driver-ptouch installed"
fi
usermod -aG lpadmin "$DEPLOY_USER"
systemctl enable cups
systemctl start cups
echo "  → CUPS enabled. Web UI: http://raku-sika-hub.local:631"
echo "  → Run 'sudo bash deploy/setup-printer.sh' after connecting the printer"

# 6. Install systemd service
echo "[6/6] Installing systemd service"
cp "$SCRIPT_DIR/raku-sika-hub.service" /etc/systemd/system/

# Override WorkingDirectory and ExecStart if project is not at default path
if [ "$PROJECT_DIR" != "/home/$DEPLOY_USER/raku-sika-hub" ]; then
    sed -i "s|WorkingDirectory=.*|WorkingDirectory=$PROJECT_DIR|" /etc/systemd/system/raku-sika-hub.service
    sed -i "s|ExecStart=.*|ExecStart=$PROJECT_DIR/raku-sika-hub|" /etc/systemd/system/raku-sika-hub.service
    sed -i "s|User=.*|User=$DEPLOY_USER|" /etc/systemd/system/raku-sika-hub.service
    sed -i "s|Environment=HOME=.*|Environment=HOME=/home/$DEPLOY_USER|" /etc/systemd/system/raku-sika-hub.service
fi

systemctl daemon-reload
systemctl enable raku-sika-hub
echo "  → systemd service installed and enabled"

echo ""
echo "=== Setup complete ==="
echo "  Hostname:  raku-sika-hub"
echo "  mDNS:      raku-sika-hub.local"
echo "  WebSocket: ws://raku-sika-hub.local:19800"
echo "  CUPS UI:   http://raku-sika-hub.local:631"
echo "  User:      $DEPLOY_USER"
echo ""
echo "Next steps:"
echo "  1. Build: go build -o raku-sika-hub . (or copy pre-built binary)"
echo "  2. Printer: sudo bash deploy/setup-printer.sh"
echo "  3. Start: sudo systemctl start raku-sika-hub"
echo "  4. Check: curl http://raku-sika-hub.local:19800/"
echo "  5. Logs:  journalctl -u raku-sika-hub -f"
