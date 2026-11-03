# Verify the Linux build + tests from Windows using Docker Desktop.
# Run from the repo root:  ./scripts/linux-test.ps1
$ErrorActionPreference = "Stop"
# $ErrorActionPreference does not stop on a native exit code; check each one so
# a failed build never runs the previous image.
docker build -f build/Dockerfile.linux-test -t dbd-linux-test .
if ($LASTEXITCODE -ne 0) { throw "docker build failed with exit code $LASTEXITCODE" }
docker run --rm dbd-linux-test
if ($LASTEXITCODE -ne 0) { throw "Linux checks failed with exit code $LASTEXITCODE" }
