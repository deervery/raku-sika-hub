#!/usr/bin/env bash
# update.sh - Download and install the latest raku-sika-hub release from GitHub.
# Usage: bash deploy/update.sh
set -euo pipefail

REPO="deervery/raku-sika-hub"
INSTALL_DIR="${HOME}/raku-sika-hub"
BINARY="raku-sika-hub"
SERVICE="raku-sika-hub"

echo "=== raku-sika-hub updater ==="

# Get latest release download URL.
DOWNLOAD_URL=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"browser_download_url"' \
  | grep 'linux-arm64' \
  | head -1 \
  | cut -d '"' -f 4)

if [ -z "${DOWNLOAD_URL}" ]; then
  echo "ERROR: Could not find latest release binary."
  echo "Make sure a release exists at https://github.com/${REPO}/releases"
  exit 1
fi

echo "Downloading: ${DOWNLOAD_URL}"
curl -sL "${DOWNLOAD_URL}" -o "/tmp/${BINARY}"
chmod +x "/tmp/${BINARY}"

# Show version of the new binary.
echo "New version:"
/tmp/${BINARY} --version 2>/dev/null || echo "(version check not supported)"

# Stop service, replace binary, start service.
echo "Stopping ${SERVICE}..."
sudo systemctl stop "${SERVICE}" || true

echo "Installing to ${INSTALL_DIR}/${BINARY}..."
mkdir -p "${INSTALL_DIR}"
cp "${INSTALL_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}.bak" 2>/dev/null || true
mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Starting ${SERVICE}..."
sudo systemctl start "${SERVICE}"

# Health check
sleep 3
if curl -sf http://127.0.0.1:19800/health >/dev/null 2>&1; then
  echo "=== Update complete (healthy) ==="
else
  echo "WARNING: Hub not responding. Rolling back..."
  if [ -f "${INSTALL_DIR}/${BINARY}.bak" ]; then
    mv "${INSTALL_DIR}/${BINARY}.bak" "${INSTALL_DIR}/${BINARY}"
    sudo systemctl restart "${SERVICE}"
    echo "Rolled back to previous version."
  fi
  exit 1
fi

sudo systemctl status "${SERVICE}" --no-pager || true
