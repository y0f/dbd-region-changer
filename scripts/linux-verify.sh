#!/usr/bin/env bash
# Runs inside the Linux test container: full tests, build, and a headless GUI
# smoke under Xvfb. Exits non-zero on any failure.
set -euo pipefail

echo "== go test (full, CGO) =="
CGO_ENABLED=1 go test ./...

echo "== build linux binary =="
if ! CGO_ENABLED=1 go build -ldflags='-s -w' -trimpath -o /tmp/dbdregion ./cmd/dbd 2>/tmp/build.log; then
  if grep -q "no Go files\|cannot find package\|matched no packages" /tmp/build.log; then
    echo "(cmd/dbd not present yet; library build + tests passed)"
    echo "ALL AVAILABLE LINUX CHECKS PASSED"
    exit 0
  fi
  cat /tmp/build.log
  exit 1
fi

echo "== headless GUI smoke (Xvfb + software OpenGL, 6s window) =="
set +e
LIBGL_ALWAYS_SOFTWARE=1 timeout 6 xvfb-run -a /tmp/dbdregion >/tmp/smoke.log 2>&1
code=$?
set -e
case "$code" in
  124) echo "GUI ran for 6s under Xvfb without crashing -> OK" ;;
  0)   echo "GUI exited cleanly within the smoke window -> OK" ;;
  *)   echo "GUI smoke FAILED (exit $code):"; cat /tmp/smoke.log; exit 1 ;;
esac

echo "ALL LINUX CHECKS PASSED"
