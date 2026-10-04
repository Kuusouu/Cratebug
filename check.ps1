[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = $PSScriptRoot
$frontendRoot = Join-Path $repositoryRoot "frontend"

function Invoke-Step {
    param(
        [Parameter(Mandatory)]
        [string]$Label,

        [Parameter(Mandatory)]
        [scriptblock]$Script,

        [Parameter(Mandatory)]
        [string]$WorkingDirectory
    )

    Write-Host "==> $Label"
    Push-Location $WorkingDirectory
    try {
        & $Script
        if ($LASTEXITCODE -ne 0) {
            throw "$Label failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}

Write-Host "==> Go formatting"
$goCommand = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCommand) {
    throw "Unable to locate Go on PATH. Install Go 1.26.5 (see CONTRIBUTING.md) and restart the terminal."
}

$gofmtCommand = Get-Command gofmt -ErrorAction SilentlyContinue
if (-not $gofmtCommand) {
    throw "Unable to locate gofmt on PATH. Ensure the Go toolchain's bin directory is on PATH."
}

$gofmtExecutable = $gofmtCommand.Source
$goFiles = @(
    Get-ChildItem -Path $repositoryRoot -Recurse -Filter "*.go" -File |
        Where-Object {
            $_.FullName -notlike "$repositoryRoot\.git\*" -and
            $_.FullName -notlike "$repositoryRoot\build\bin\*" -and
            $_.FullName -notlike "$frontendRoot\node_modules\*"
        } |
        Select-Object -ExpandProperty FullName
)

$unformattedFiles = @(& $gofmtExecutable -l $goFiles)
if ($LASTEXITCODE -ne 0) {
    throw "gofmt failed with exit code $LASTEXITCODE."
}
if ($unformattedFiles.Count -gt 0) {
    throw "Go formatting check failed:`n$($unformattedFiles -join [Environment]::NewLine)"
}

Invoke-Step -Label "Frontend checks" -Script { bun run check } -WorkingDirectory $frontendRoot
Invoke-Step -Label "Go vet" -Script { go vet ./... } -WorkingDirectory $repositoryRoot
Invoke-Step -Label "Go tests" -Script { go test ./... } -WorkingDirectory $repositoryRoot

Write-Host "All checks passed."
