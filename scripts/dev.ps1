param(
    [switch]$SeparateWindows
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backendScriptPath = Join-Path $root "scripts\dev-backend.ps1"

function Write-DevLog {
    param(
        [string]$Prefix,
        [string]$Message,
        [ConsoleColor]$Color = [ConsoleColor]::Gray
    )

    Write-Host "[$Prefix] $Message" -ForegroundColor $Color
}

function Start-FrontendProcess {
    Start-Process -FilePath "npx" -ArgumentList @(
        "vite"
    ) -WorkingDirectory (Join-Path $root "web-astryx") -NoNewWindow -PassThru
}

Write-Host ""
Write-Host "Starting Mizu development mode..." -ForegroundColor Cyan
Write-Host "Frontend: http://localhost:3001" -ForegroundColor DarkGray
Write-Host "Backend:  http://localhost:8978" -ForegroundColor DarkGray
Write-Host ""

if ($SeparateWindows) {
    $frontendScript = "Set-Location '$root\web-astryx'; npx vite"

    Start-Process -FilePath "powershell" -ArgumentList @(
        "-NoExit",
        "-ExecutionPolicy", "Bypass",
        "-Command", $frontendScript
    ) -WorkingDirectory (Join-Path $root "web-astryx")

    Start-Process -FilePath "powershell" -ArgumentList @(
        "-NoExit",
        "-ExecutionPolicy", "Bypass",
        "-File", $backendScriptPath
    ) -WorkingDirectory $root

    Write-Host "Mizu dev windows launched." -ForegroundColor Green
    Write-Host "Close the two spawned PowerShell windows to stop development mode." -ForegroundColor DarkGray
    return
}

$frontendProcess = $null

try {
    $frontendProcess = Start-FrontendProcess
    Write-DevLog "mizu:web" "frontend started in this terminal; use Ctrl+C once to stop both processes" DarkGray
    & $backendScriptPath
}
finally {
    if ($frontendProcess -and -not $frontendProcess.HasExited) {
        Write-DevLog "mizu:web" "stopping frontend dev server" DarkYellow
        try {
            $frontendProcess.Kill()
            $frontendProcess.WaitForExit()
        } catch {
        }
    }
}
