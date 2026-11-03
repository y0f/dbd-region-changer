#!/usr/bin/env bash
# Build the Linux binary. Requires gcc + libgl1-mesa-dev + xorg-dev +
# libxkbcommon-dev. When HEAD is exactly on a release tag, that version is
# stamped into the binary.
set -euo pipefail
ldflags='-s -w'
if tag=$(git describe --tags --exact-match 2>/dev/null); then
  ldflags="$ldflags -X github.com/y0f/dbd-region-changer/internal/version.tag=$tag"
fi
mkdir -p dist
CGO_ENABLED=1 go build -ldflags="$ldflags" -trimpath -o dist/dbdregion ./cmd/dbd
sha256sum dist/dbdregion > dist/dbdregion_sha256checksum.txt
echo "Built dist/dbdregion"
cat dist/dbdregion_sha256checksum.txt
