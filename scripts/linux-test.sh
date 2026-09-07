#!/usr/bin/env bash
# Verify the Linux build + tests via Docker. Run from the repo root.
set -e
docker build -f build/Dockerfile.linux-test -t dbd-linux-test .
docker run --rm dbd-linux-test
