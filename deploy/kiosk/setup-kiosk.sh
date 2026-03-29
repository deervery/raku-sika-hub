#!/bin/bash
# RakuSika Kiosk Setup Script
# Sets up Raspberry Pi as a kiosk displaying https://rakusika.com
# with raku-sika-hub running locally for hardware access.
#
# Prerequisites:
#   - Raspberry Pi OS Desktop (not Lite) already installed
#   - deploy/setup.sh already executed (hub service configured)
#   - SSH access working
#
# Usage:
#   sudo bash deploy/kiosk/setup-kiosk.sh

set -euo pipefail

# --- Validate ---
if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: Run as root (sudo bash $0)"
    exit 1
fi

DEPLOY_USER="${DEPLOY_USER:-rakusika}"
DEPLOY_DIR="/home/${DEPLOY_USER}/raku-sika-hub"
KIOSK_DIR="${DEPLOY_DIR}/deploy/kiosk"

if [ ! -f "${KIOSK_DIR}/chromium-kiosk.sh" ]; then
    echo "ERROR: ${KIOSK_DIR}/chromium-kiosk.sh not found"
    echo "Run this script from the raku-sika-hub directory"
    exit 1
fi

echo "=== RakuSika Kiosk Setup ==="
echo "User: ${DEPLOY_USER}"
echo "Dir:  ${DEPLOY_DIR}"
echo ""

# --- 1. Run base setup if hub service not installed ---
if ! systemctl list-unit-files | grep -q raku-sika-hub.service; then
    echo "[1/7] Running base setup (hub service)..."
    bash "${DEPLOY_DIR}/deploy/setup.sh"
else
    echo "[1/7] Hub service already installed, skipping base setup"
fi

# --- 2. Install kiosk packages ---
echo "[2/7] Installing kiosk packages..."
apt-get update -qq
apt-get install -y -qq \
    chromium \
    zram-tools \
    watchdog \
    unclutter \
    > /dev/null 2>&1
echo "  Installed: chromium-browser, zram-tools, watchdog, unclutter"

# --- 3. Configure auto-login ---
echo "[3/7] Configuring auto-login for ${DEPLOY_USER}..."

# For Raspberry Pi OS Desktop with LightDM
if [ -f /etc/lightdm/lightdm.conf ]; then
    sed -i "s/^#\?autologin-user=.*/autologin-user=${DEPLOY_USER}/" /etc/lightdm/lightdm.conf
    # Ensure autologin is in the [Seat:*] section
    if ! grep -q "autologin-user=${DEPLOY_USER}" /etc/lightdm/lightdm.conf; then
        sed -i "/^\[Seat:\*\]/a autologin-user=${DEPLOY_USER}" /etc/lightdm/lightdm.conf
    fi
    echo "  LightDM auto-login configured"
fi

# For Raspberry Pi OS with labwc/wayfire (Pi 5 default)
WAYFIRE_AUTOSTART="/home/${DEPLOY_USER}/.config/wayfire.ini"
if [ -f "$WAYFIRE_AUTOSTART" ] || command -v wayfire >/dev/null 2>&1; then
    echo "  Wayland compositor detected (wayfire/labwc)"
fi

# --- 4. Set up kiosk autostart ---
echo "[4/7] Setting up kiosk autostart..."
chmod +x "${KIOSK_DIR}/chromium-kiosk.sh"
chmod +x "${KIOSK_DIR}/healthcheck.sh"

# XDG autostart (works with most desktop environments)
AUTOSTART_DIR="/home/${DEPLOY_USER}/.config/autostart"
mkdir -p "$AUTOSTART_DIR"
cp "${KIOSK_DIR}/raku-sika-kiosk.desktop" "${AUTOSTART_DIR}/"
chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "$AUTOSTART_DIR"
echo "  Autostart desktop entry installed"

# Also set up for labwc (Pi 5 default Wayland compositor)
LABWC_AUTOSTART_DIR="/home/${DEPLOY_USER}/.config/labwc"
if [ -d "$LABWC_AUTOSTART_DIR" ] || command -v labwc >/dev/null 2>&1; then
    mkdir -p "$LABWC_AUTOSTART_DIR"
    # Append kiosk launch to labwc autostart
    LABWC_AUTOSTART="${LABWC_AUTOSTART_DIR}/autostart"
    if ! grep -q "chromium-kiosk" "$LABWC_AUTOSTART" 2>/dev/null; then
        echo "${KIOSK_DIR}/chromium-kiosk.sh &" >> "$LABWC_AUTOSTART"
        chmod +x "$LABWC_AUTOSTART"
    fi
    chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "$LABWC_AUTOSTART_DIR"
    echo "  labwc autostart configured"
fi

# --- 5. Disable screen blanking ---
echo "[5/7] Disabling screen blanking..."

# Kernel parameter
CMDLINE="/boot/firmware/cmdline.txt"
if [ -f "$CMDLINE" ]; then
    if ! grep -q "consoleblank=0" "$CMDLINE"; then
        sed -i 's/$/ consoleblank=0/' "$CMDLINE"
        echo "  Added consoleblank=0 to kernel cmdline"
    fi
fi

# Disable DPMS for Wayland (labwc)
LABWC_ENV="/home/${DEPLOY_USER}/.config/labwc/environment"
if [ -d "/home/${DEPLOY_USER}/.config/labwc" ]; then
    mkdir -p "$(dirname "$LABWC_ENV")"
    grep -q "IDLE_TIMEOUT" "$LABWC_ENV" 2>/dev/null || \
        echo "IDLE_TIMEOUT=0" >> "$LABWC_ENV"
    chown "${DEPLOY_USER}:${DEPLOY_USER}" "$LABWC_ENV"
fi

# --- 6. Configure zram (SD card protection) ---
echo "[6/7] Configuring zram swap..."

# Disable SD card swap
systemctl disable --now dphys-swapfile 2>/dev/null || true

# Configure zram
cat > /etc/default/zramswap << 'EOF'
ALGO=lz4
PERCENT=25
EOF
systemctl enable zramswap

# Reduce swappiness
if ! grep -q "vm.swappiness" /etc/sysctl.d/99-rakusika.conf 2>/dev/null; then
    echo "vm.swappiness=10" > /etc/sysctl.d/99-rakusika.conf
    sysctl -p /etc/sysctl.d/99-rakusika.conf 2>/dev/null || true
fi
echo "  zram enabled, dphys-swapfile disabled, swappiness=10"

# --- 7. Configure hardware watchdog ---
echo "[7/7] Configuring hardware watchdog..."

cat > /etc/watchdog.conf << EOF
watchdog-device = /dev/watchdog
watchdog-timeout = 15
max-load-1 = 24
interval = 10
test-binary = ${KIOSK_DIR}/healthcheck.sh
EOF

systemctl enable watchdog
echo "  Hardware watchdog configured (15s timeout)"

# --- Done ---
echo ""
echo "=== Kiosk setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Reboot: sudo reboot"
echo "  2. After reboot, Chromium will open https://rakusika.com"
echo "  3. In rakusika.com settings, set hub URL to: http://localhost:19800"
echo "     (This should already be the default fallback)"
echo "  4. Test scale/printer/scanner from the UI"
echo ""
echo "Troubleshooting:"
echo "  journalctl -u raku-sika-hub -f    # Hub logs"
echo "  systemctl status raku-sika-hub     # Hub status"
echo "  bash ${KIOSK_DIR}/healthcheck.sh   # Health check"
