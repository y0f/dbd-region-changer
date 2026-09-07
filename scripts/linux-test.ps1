# Verify the Linux build + tests from Windows using Docker Desktop.
# Run from the repo root:  ./scripts/linux-test.ps1
$ErrorActionPreference = "Stop"
docker build -f build/Dockerfile.linux-test -t dbd-linux-test .
docker run --rm dbd-linux-test
