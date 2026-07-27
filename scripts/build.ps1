$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$frontendDir = Join-Path $root "web-astryx"
$embedDir = Join-Path $root "internal\web\dist"

Write-Host ""
Write-Host "Building Mizu production binary..." -ForegroundColor Cyan
Write-Host ""

# Step 1: Build the frontend
Write-Host "[mizu:web] building frontend (vp build)" -ForegroundColor Gray
Push-Location $frontendDir
try {
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

# Step 3: Build the Go binary
Write-Host "[mizu:api] building Go binary" -ForegroundColor Gray
Push-Location $root
try {
    & go build -o "mizu.exe" ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "Done. Binary: mizu.exe" -ForegroundColor Green
Write-Host "Run with: .\mizu.exe" -ForegroundColor DarkGray
Write-Host ""
