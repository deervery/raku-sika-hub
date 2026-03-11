#!/bin/bash
set -e

# Brother QL-800/QL-820 printer setup for Raspberry Pi
# Run as: sudo bash deploy/setup-printer.sh
#
# Prerequisites: Brother QL printer connected via USB

echo "=== Brother QL printer setup ==="

DEPLOY_USER="${SUDO_USER:-rakusika}"
PRINTER_NAME="${1:-Brother_QL-800}"

# 1. Install CUPS and Brother driver
echo "[1/4] Installing CUPS and printer driver"
apt-get update -qq
apt-get install -y -qq cups printer-driver-ptouch

# 2. Add user to lpadmin group
echo "[2/4] Adding $DEPLOY_USER to lpadmin group"
if id -nG "$DEPLOY_USER" | grep -qw lpadmin; then
    echo "  → $DEPLOY_USER is already in lpadmin group"
else
    usermod -aG lpadmin "$DEPLOY_USER"
    echo "  → Added $DEPLOY_USER to lpadmin group"
fi

# 3. Enable and start CUPS
echo "[3/4] Enabling CUPS service"
systemctl enable cups
systemctl start cups

# Allow remote access to CUPS web admin (optional, for setup)
cupsctl --remote-admin

# 4. Auto-detect and add Brother QL printer
echo "[4/4] Detecting Brother QL printer"

# Wait briefly for USB device
sleep 2

# Find Brother QL USB device URI
DEVICE_URI=$(lpinfo -v 2>/dev/null | grep -i "usb://Brother" | grep -i "QL" | head -1 | awk '{print $2}')

if [ -z "$DEVICE_URI" ]; then
    echo "  ! Brother QL printer not detected via USB."
    echo "  Manual setup:"
    echo "    1. Connect the printer and power it on"
    echo "    2. Run: lpinfo -v | grep Brother"
    echo "    3. Run: lpadmin -p $PRINTER_NAME -E -v <device-uri> -m everywhere"
    echo "    Or use CUPS Web UI: http://localhost:631/admin"
    echo ""
    echo "  Alternatively, after connecting the printer:"
    echo "    sudo bash deploy/setup-printer.sh $PRINTER_NAME"
    exit 0
fi

echo "  → Found: $DEVICE_URI"

# Find PPD/driver
PPD=$(lpinfo -m 2>/dev/null | grep -i "QL" | head -1 | awk '{print $1}')

if [ -n "$PPD" ]; then
    echo "  → Driver: $PPD"
    lpadmin -p "$PRINTER_NAME" -E -v "$DEVICE_URI" -m "$PPD"
else
    echo "  → Using driverless mode"
    lpadmin -p "$PRINTER_NAME" -E -v "$DEVICE_URI" -m everywhere
fi

# Set as default printer
lpadmin -d "$PRINTER_NAME"

# Enable the printer
cupsenable "$PRINTER_NAME"
cupsaccept "$PRINTER_NAME"

echo ""
echo "=== Printer setup complete ==="
echo "  Printer:  $PRINTER_NAME"
echo "  URI:      $DEVICE_URI"
echo "  CUPS UI:  http://raku-sika-hub.local:631"
echo ""
echo "Test:"
echo "  echo 'test' | lp -d $PRINTER_NAME -"
echo "  Or via hub: {\"type\":\"print_test\",\"requestId\":\"t1\"}"
