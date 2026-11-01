param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$frontendDir = Join-Path $root "web"
$embedDir = Join-Path $root "internal\web\dist"

# Resolve the version stamp: an explicit -Version argument wins; otherwise read
# the canonical version from the frontend package.json (bumped alongside
# CHANGELOG.md); fall back to "dev".
if (-not $Version) {
    $pkg = Get-Content (Join-Path $frontendDir "package.json") -Raw | ConvertFrom-Json
    $Version = $pkg.version
    if (-not $Version) { $Version = "dev" }
}

# Commit and build date for the version banner, mirroring the Dockerfile.
$Commit = git -C $root rev-parse --short HEAD 2>$null
if (-not $Commit) { $Commit = "unknown" }
$BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host ""
Write-Host "Building Mizu production binary $Version..." -ForegroundColor Cyan
Write-Host ""

# Step 1: Build the frontend — typecheck first (mirrors `bun run build`,
# which is `tsc -b && vp build`) so a type error fails the local build the
# same way it now fails the Docker image build.
Write-Host "[mizu:web] building frontend (tsc -b && vp build)" -ForegroundColor Gray
Push-Location $frontendDir
try {
    & bunx tsc -b
    if ($LASTEXITCODE -ne 0) { throw "frontend typecheck failed" }
    & vp build
    if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
}
finally {
    Pop-Location
}

# Step 2: Copy frontend output to the Go embed directory
Write-Host "[mizu:web] copying build output to internal/web/dist/" -ForegroundColor Gray
if (Test-Path $embedDir) {
    Remove-Item $embedDir -Recurse -Force
}
New-Item -ItemType Directory -Path $embedDir -Force | Out-Null
Copy-Item -Path (Join-Path $frontendDir "dist\*") -Destination $embedDir -Recurse -Force

# Step 3: Build the Go binary (version stamped via ldflags, like the Dockerfile)
Write-Host "[mizu:api] building Go binary" -ForegroundColor Gray
Push-Location $root
try {
    $ldflags = "-s -w -X main.version=$Version -X main.commit=$Commit -X main.buildDate=$BuildDate"
    & go build -ldflags "$ldflags" -o "mizu.exe" ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "Done. Binary: mizu.exe" -ForegroundColor Green
Write-Host "Run with: .\mizu.exe" -ForegroundColor DarkGray
Write-Host ""
