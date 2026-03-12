#!/bin/bash
# Generate a self-signed TLS certificate for raku-sika-hub WSS support.
# Usage: ./deploy/generate-cert.sh [output_dir]
#
# Creates cert.pem and key.pem valid for 10 years.
# Includes SANs for common local access patterns.

set -euo pipefail

CERT_DIR="${1:-/home/rakusika/raku-sika-hub/certs}"
DAYS=3650

mkdir -p "$CERT_DIR"

# Detect the device's LAN IP address
LAN_IP=$(hostname -I | awk '{print $1}')

echo "Generating self-signed certificate..."
echo "  Output:  $CERT_DIR"
echo "  LAN IP:  $LAN_IP"
echo "  Valid:   $DAYS days"

openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
  -nodes \
  -days "$DAYS" \
  -keyout "$CERT_DIR/key.pem" \
  -out "$CERT_DIR/cert.pem" \
  -subj "/CN=raku-sika-hub" \
  -addext "subjectAltName=DNS:raku-sika-hub.local,DNS:localhost,IP:$LAN_IP,IP:127.0.0.1"

chmod 600 "$CERT_DIR/key.pem"
chmod 644 "$CERT_DIR/cert.pem"

echo ""
echo "Done! Add to config.json or environment:"
echo "  TLS_CERT_PATH=$CERT_DIR/cert.pem"
echo "  TLS_KEY_PATH=$CERT_DIR/key.pem"
