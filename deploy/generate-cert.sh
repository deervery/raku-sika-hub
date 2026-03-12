#!/bin/bash
# Generate a self-signed TLS certificate for raku-sika-hub WSS support.
# Usage: ./deploy/generate-cert.sh [output_dir] [lan_ip]
#
# Creates cert.pem and key.pem valid for 10 years.

set -euo pipefail

CERT_DIR="${1:-/home/rakusika/raku-sika-hub/certs}"
LAN_IP="${2:-$(hostname -I | awk '{print $1}')}"

mkdir -p "$CERT_DIR"

echo "Generating self-signed certificate..."
echo "  Output:  $CERT_DIR"
echo "  LAN IP:  $LAN_IP"

openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 3650 \
  -keyout "$CERT_DIR/key.pem" \
  -out "$CERT_DIR/cert.pem" \
  -subj "/CN=$LAN_IP" \
  -addext "subjectAltName=IP:$LAN_IP,IP:127.0.0.1,DNS:localhost,DNS:raku-sika-hub.local" \
  -addext "keyUsage=critical,digitalSignature,keyEncipherment" \
  -addext "extendedKeyUsage=serverAuth"

chmod 600 "$CERT_DIR/key.pem"
chmod 644 "$CERT_DIR/cert.pem"

echo ""
echo "Done! Add to config.json or environment:"
echo "  TLS_CERT_PATH=$CERT_DIR/cert.pem"
echo "  TLS_KEY_PATH=$CERT_DIR/key.pem"
