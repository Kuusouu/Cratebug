[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = $PSScriptRoot
$frontendRoot = Join-Path $repositoryRoot "frontend"

function Invoke-MiseCommand {
    param(
        [Parameter(Mandatory)]
        [string]$Label,

        [Parameter(Mandatory)]
        [string]$Command,

        [Parameter(Mandatory)]
        [string]$WorkingDirectory
    )

    Write-Host "==> $Label"
    Push-Location $WorkingDirectory
    try {
        & mise exec -c $Command
        if ($LASTEXITCODE -ne 0) {
            throw "$Label failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}

Write-Host "==> Go formatting"
$goExecutable = (& mise which go).Trim()
if ($LASTEXITCODE -ne 0 -or -not $goExecutable) {
    throw "Unable to locate the pinned Go toolchain through mise."
}

$gofmtExecutable = Join-Path (Split-Path -Parent $goExecutable) "gofmt.exe"
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

Invoke-MiseCommand -Label "Frontend checks" -Command "bun run check" -WorkingDirectory $frontendRoot
Invoke-MiseCommand -Label "Go vet" -Command "go vet ./..." -WorkingDirectory $repositoryRoot
Invoke-MiseCommand -Label "Go tests" -Command "go test ./..." -WorkingDirectory $repositoryRoot

Write-Host "All checks passed."
