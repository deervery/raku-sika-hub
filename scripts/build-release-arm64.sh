#!/usr/bin/env bash
set -euo pipefail

OUTPUT="${1:-raku-sika-hub-linux-arm64}"
VERSION="${VERSION:-$(git describe --tags --exact-match 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

echo "[build-release] output=${OUTPUT}"
echo "[build-release] version=${VERSION} commit=${COMMIT} buildDate=${BUILD_DATE}"

GOOS=linux GOARCH=arm64 go build \
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
  -o "${OUTPUT}" .

sha256sum "${OUTPUT}" > "${OUTPUT}.sha256"
echo "[build-release] wrote ${OUTPUT} and ${OUTPUT}.sha256"
