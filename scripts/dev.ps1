param(
    [switch]$SeparateWindows
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

function Write-DevLog {
    param(
        [string]$Prefix,
        [string]$Message,
        [ConsoleColor]$Color = [ConsoleColor]::Gray
    )

    Write-Host "[$Prefix] $Message" -ForegroundColor $Color
}

function Start-FrontendProcess {
    Start-Process -FilePath "vp" -ArgumentList @(
        "dev"
    ) -WorkingDirectory (Join-Path $root "web") -NoNewWindow -PassThru
}

Write-Host ""
Write-Host "Starting Mizu development mode..." -ForegroundColor Cyan
Write-Host "Frontend: http://localhost:3001" -ForegroundColor DarkGray
Write-Host "Backend:  http://localhost:8978" -ForegroundColor DarkGray
Write-Host ""

if ($SeparateWindows) {
    $frontendScript = "Set-Location '$root\web'; vp dev"
    $backendScript = "Set-Location '$root'; air"

    Start-Process -FilePath "powershell" -ArgumentList @(
        "-NoExit",
        "-ExecutionPolicy", "Bypass",
        "-Command", $frontendScript
    ) -WorkingDirectory (Join-Path $root "web")

    Start-Process -FilePath "powershell" -ArgumentList @(
        "-NoExit",
        "-ExecutionPolicy", "Bypass",
        "-Command", $backendScript
    ) -WorkingDirectory $root

    Write-Host "Mizu dev windows launched." -ForegroundColor Green
    Write-Host "Close the two spawned PowerShell windows to stop development mode." -ForegroundColor DarkGray
    return
}

$frontendProcess = $null

try {
    $frontendProcess = Start-FrontendProcess
    Write-DevLog "mizu:web" "frontend started; use Ctrl+C once to stop both processes" DarkGray

    # Run air in the foreground — it handles Go file watching + rebuild + restart
    & air
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
