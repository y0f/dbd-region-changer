#!/usr/bin/env bash
# Build the Linux binary. Requires gcc + libgl1-mesa-dev + xorg-dev +
# libxkbcommon-dev.
set -e
mkdir -p dist
CGO_ENABLED=1 go build -ldflags='-s -w' -trimpath -o dist/dbdregion ./cmd/dbd
sha256sum dist/dbdregion > dist/dbdregion_sha256checksum.txt
echo "Built dist/dbdregion"
cat dist/dbdregion_sha256checksum.txt
