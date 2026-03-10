param(
    [string]$ConfigPath = "config.yml"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$resolvedConfigPath = Join-Path $root $ConfigPath

if (-not (Test-Path $resolvedConfigPath)) {
    throw "Config file not found: $resolvedConfigPath"
}

$bun = Get-Command bun -ErrorAction SilentlyContinue
$frontendCommand = if ($bun) { "bun run dev" } else { "npm run dev" }

$frontendScript = "Set-Location '$root\web'; $frontendCommand"
$backendScriptPath = Join-Path $root "scripts\dev-backend.ps1"

Write-Host ""
Write-Host "Starting Mizu development mode..." -ForegroundColor Cyan
Write-Host "Frontend: http://localhost:5173" -ForegroundColor DarkGray
Write-Host "Backend:  http://localhost:8978" -ForegroundColor DarkGray
Write-Host ""

Start-Process -FilePath "powershell" -ArgumentList @(
    "-NoExit",
    "-ExecutionPolicy", "Bypass",
    "-Command", $frontendScript
) -WorkingDirectory (Join-Path $root "web")

Start-Process -FilePath "powershell" -ArgumentList @(
    "-NoExit",
    "-ExecutionPolicy", "Bypass",
    "-File", $backendScriptPath,
    "-ConfigPath", $resolvedConfigPath
) -WorkingDirectory $root

Write-Host "Mizu dev windows launched." -ForegroundColor Green
Write-Host "Close the two spawned PowerShell windows to stop development mode." -ForegroundColor DarkGray
