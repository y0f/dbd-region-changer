# Build the Windows binary with embedded icon, version info, and the
# requireAdministrator manifest. Run from the repo root with MinGW gcc on PATH.
# When HEAD is exactly on a release tag (e.g. 1.2.3), that version is stamped
# into the binary and its file properties.
$ErrorActionPreference = "Stop"

# $ErrorActionPreference does not stop on a native command's exit code, so
# every native call goes through this.
function Invoke-Native {
    & $args[0] $args[1..($args.Count - 1)]
    if ($LASTEXITCODE -ne 0) { throw "$($args[0]) failed with exit code $LASTEXITCODE" }
}

# Not on a tag is normal; in Windows PowerShell a redirected native stderr
# line would otherwise become a terminating error under "Stop".
$ErrorActionPreference = "Continue"
$tag = git describe --tags --exact-match 2>$null
if ($LASTEXITCODE -ne 0) { $tag = "" }
$ErrorActionPreference = "Stop"
$ldflags = "-s -w -H windowsgui"
$verArgs = @()
if ($tag -match '^v?(\d+)\.(\d+)\.(\d+)') {
    $ver = "$($Matches[1]).$($Matches[2]).$($Matches[3])"
    $ldflags += " -X github.com/y0f/dbd-region-changer/internal/version.tag=$tag"
    $verArgs = @(
        "-ver-major=$($Matches[1])", "-ver-minor=$($Matches[2])", "-ver-patch=$($Matches[3])",
        "-product-ver-major=$($Matches[1])", "-product-ver-minor=$($Matches[2])", "-product-ver-patch=$($Matches[3])",
        "-file-version=$ver", "-product-version=$ver"
    )
}

# Generate the Windows resource object (.syso) consumed by the linker.
Invoke-Native go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0 `
    @verArgs -o cmd/dbd/resource_windows.syso build/versioninfo.json

$env:CGO_ENABLED = "1"
New-Item -ItemType Directory -Force -Path dist | Out-Null
Invoke-Native go build -ldflags $ldflags -trimpath -o dist/dbdregion.exe ./cmd/dbd

# SHA256 checksum (parity with the original build.bat).
$hash = (Get-FileHash dist/dbdregion.exe -Algorithm SHA256).Hash
Set-Content -Path dist/dbdregion.exe_sha256checksum.txt -Value $hash -Encoding utf8
Write-Host "Built dist/dbdregion.exe  SHA256=$hash"
