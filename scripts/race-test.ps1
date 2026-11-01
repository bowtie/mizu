# Runs the full test suite under Go's race detector.
#
# -race requires CGO, and CGO on Windows requires a C compiler. MinGW gcc
# works out of the box; an MSVC-targeting clang also works on Go 1.26+,
# so the default LLVM install location is probed as a fallback.
#
# Usage:  .\scripts\race-test.ps1            # clang from PATH or LLVM default
#         .\scripts\race-test.ps1 -CC gcc    # explicit compiler

param(
    [string]$CC = ""
)

$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

if ($CC -eq "") {
    $CC = "clang"
    if (-not (Get-Command clang -ErrorAction SilentlyContinue)) {
        $llvm = "C:\Program Files\LLVM\bin\clang.exe"
        if (Test-Path $llvm) {
            $CC = $llvm
        } else {
            Write-Error "No C compiler found. Install MinGW gcc or LLVM, or pass -CC <path>."
        }
    }
}

$env:CGO_ENABLED = "1"
# Go splits the CC value on whitespace — a path with spaces needs
# embedded quotes so the toolchain resolver keeps it intact.
if ($CC -match ' ') {
    $env:CC = "`"$CC`""
} else {
    $env:CC = $CC
}

Write-Host "Running go test -race with CC=$CC ..."
go test -race ./... $args
exit $LASTEXITCODE
