param(
    [string]$ConfigPath = "config.yml"
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

$state = [hashtable]::Synchronized(@{
    RestartPending = $false
})

$script:backendProcess = $null
$watchers = @()
$subscriptions = @()

function Write-DevLog {
    param(
        [string]$Message,
        [ConsoleColor]$Color = [ConsoleColor]::Cyan
    )

    Write-Host "[mizu:api] $Message" -ForegroundColor $Color
}

function Stop-BackendProcess {
    if ($script:backendProcess -and -not $script:backendProcess.HasExited) {
        Write-DevLog "stopping backend" DarkYellow
        Stop-Process -Id $script:backendProcess.Id -Force -ErrorAction SilentlyContinue
        $script:backendProcess.WaitForExit()
    }
    $script:backendProcess = $null
}

function Start-BackendProcess {
    Write-DevLog "starting backend watcher target"
    $script:backendProcess = Start-Process -FilePath "go" -ArgumentList @(
        "run",
        "./cmd/server",
        "--config",
        $resolvedConfigPath
    ) -WorkingDirectory $root -NoNewWindow -PassThru
}

function Restart-BackendProcess {
    Stop-BackendProcess
    Start-Sleep -Milliseconds 200
    Start-BackendProcess
}

function New-CodeWatcher {
    param(
        [string]$Path,
        [bool]$IncludeSubdirectories = $false,
        [string]$Filter = "*"
    )

    $watcher = New-Object System.IO.FileSystemWatcher
    $watcher.Path = $Path
    $watcher.IncludeSubdirectories = $IncludeSubdirectories
    $watcher.Filter = $Filter
    $watcher.NotifyFilter = [IO.NotifyFilters]'FileName, DirectoryName, LastWrite, CreationTime, Size'
    $watcher.EnableRaisingEvents = $true

    $action = {
        $name = $Event.SourceEventArgs.Name
        if ($name -match '\\.tmp$' -or $name -match '\\.swp$') {
            return
        }
        $state.RestartPending = $true
    }

    $script:subscriptions += Register-ObjectEvent -InputObject $watcher -EventName Changed -Action $action
    $script:subscriptions += Register-ObjectEvent -InputObject $watcher -EventName Created -Action $action
    $script:subscriptions += Register-ObjectEvent -InputObject $watcher -EventName Deleted -Action $action
    $script:subscriptions += Register-ObjectEvent -InputObject $watcher -EventName Renamed -Action $action

    $script:watchers += $watcher
}

New-CodeWatcher -Path (Join-Path $root "cmd") -IncludeSubdirectories $true
New-CodeWatcher -Path (Join-Path $root "internal") -IncludeSubdirectories $true
New-CodeWatcher -Path $root -Filter "go.mod"
New-CodeWatcher -Path $root -Filter "go.sum"
New-CodeWatcher -Path $root -Filter ([System.IO.Path]::GetFileName($resolvedConfigPath))

Write-DevLog "watching Go sources and config for changes" DarkGray
Write-DevLog "using config $resolvedConfigPath" DarkGray
Start-BackendProcess

try {
    while ($true) {
        Start-Sleep -Milliseconds 350

        if ($state.RestartPending) {
            $state.RestartPending = $false
            Start-Sleep -Milliseconds 250
            while ($state.RestartPending) {
                $state.RestartPending = $false
                Start-Sleep -Milliseconds 250
            }
            Write-DevLog "change detected, restarting backend" Yellow
            Restart-BackendProcess
            continue
        }

        if ($script:backendProcess -and $script:backendProcess.HasExited) {
            Write-DevLog "backend exited, restarting" Yellow
            Start-Sleep -Milliseconds 400
            Start-BackendProcess
        }
    }
}
finally {
    foreach ($subscription in $script:subscriptions) {
        try {
            Unregister-Event -SubscriptionId $subscription.Id -ErrorAction SilentlyContinue
        } catch {
        }
    }

    foreach ($watcher in $script:watchers) {
        try {
            $watcher.EnableRaisingEvents = $false
            $watcher.Dispose()
        } catch {
        }
    }

    Stop-BackendProcess
}
