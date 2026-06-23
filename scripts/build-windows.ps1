# Build the Windows binary with embedded icon, version info, and the
# requireAdministrator manifest. Run from the repo root with MinGW gcc on PATH.
$ErrorActionPreference = "Stop"

# Generate the Windows resource object (.syso) consumed by the linker.
go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest `
    -o cmd/dbd/resource_windows.syso `
    build/versioninfo.json

$env:CGO_ENABLED = "1"
New-Item -ItemType Directory -Force -Path dist | Out-Null
go build -ldflags "-s -w -H windowsgui" -trimpath -o dist/dbdregion.exe ./cmd/dbd

# SHA256 checksum (parity with the original build.bat).
$hash = (Get-FileHash dist/dbdregion.exe -Algorithm SHA256).Hash
Set-Content -Path dist/dbdregion.exe_sha256checksum.txt -Value $hash -Encoding utf8
Write-Host "Built dist/dbdregion.exe  SHA256=$hash"
