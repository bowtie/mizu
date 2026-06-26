param(
    [string]$ConfigPath = "config.yml",
    [switch]$SeparateWindows
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$resolvedConfigPath = if ([System.IO.Path]::IsPathRooted($ConfigPath)) {
    $ConfigPath
} else {
    Join-Path $root $ConfigPath
}

if (-not (Test-Path $resolvedConfigPath)) {
    throw "Config file not found: $resolvedConfigPath"
}

$vp = Get-Command vp -ErrorAction SilentlyContinue
$frontendExecutable = if ($vp) {
    $vp.Source
} else {
    $vpBin = Join-Path $env:USERPROFILE ".vite-plus\bin\vp.exe"
    if (Test-Path $vpBin) { $vpBin } else { throw "vp not found. Install Vite+: irm https://vite.plus/ps1 | iex" }
}

$frontendArguments = "dev"
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
    Start-Process -FilePath $frontendExecutable -ArgumentList @(
        "dev"
    ) -WorkingDirectory (Join-Path $root "web") -NoNewWindow -PassThru
}

Write-Host ""
Write-Host "Starting Mizu development mode..." -ForegroundColor Cyan
Write-Host "Frontend: http://localhost:5173" -ForegroundColor DarkGray
Write-Host "Backend:  http://localhost:8978" -ForegroundColor DarkGray
Write-Host ""

if ($SeparateWindows) {
    $frontendScript = "Set-Location '$root\web'; & '$frontendExecutable' dev"

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
    return
}

$frontendProcess = $null

try {
    $frontendProcess = Start-FrontendProcess
    Write-DevLog "mizu:web" "frontend started in this terminal; use Ctrl+C once to stop both processes" DarkGray
    & $backendScriptPath -ConfigPath $resolvedConfigPath
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
