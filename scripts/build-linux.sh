#!/usr/bin/env bash
# Build the Linux binary. Requires gcc + libgl1-mesa-dev + xorg-dev +
# libxkbcommon-dev. When HEAD is exactly on a release tag, that version is
# stamped into the binary.
set -euo pipefail
ldflags='-s -w'
if tag=$(git describe --tags --exact-match 2>/dev/null); then
  # Only tags both version.Parse and the updater accept (pinned by
  # internal/updater/buildtag_test.go); an unparsable tag would silently ship
  # the fallback version or never be offered as an update, so refuse to build.
  if ! [[ $tag =~ ^v?[0-9]{1,9}\.[0-9]{1,9}(\.[0-9]{1,9})?([.-]?(dev|a|b|rc)[0-9]{0,9})?$ ]]; then
    echo "tag $tag is not MAJOR.MINOR[.PATCH][stageN]" >&2
    exit 1
  fi
  ldflags="$ldflags -X github.com/y0f/dbd-region-changer/internal/version.tag=$tag"
fi
mkdir -p dist
CGO_ENABLED=1 go build -ldflags="$ldflags" -trimpath -o dist/dbdregion ./cmd/dbd
sha256sum dist/dbdregion > dist/dbdregion_sha256checksum.txt
echo "Built dist/dbdregion"
cat dist/dbdregion_sha256checksum.txt
