[CmdletBinding()]
param()

# Fetches and verifies the pinned UAssetToolRivals worker release. See
# docs/decisions/0004-pin-uassettool-worker.md for why this version and
# repository are pinned, and what changes require re-pinning.
#
# Does not require the .NET SDK: the release is a self-contained, single-file
# publish, and this script only downloads, verifies, and extracts it.

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = $PSScriptRoot
$releaseRepo = "XzantGaming/UassetToolRivals"
$releaseTag = "v1.5.6"
$assetName = "UAssetTool-win-x64.zip"
$expectedSha256 = "16c051cbc68bef0b9050ca83a8fd3d8d997156ed1e91f4112042f41443bdabaf"
$expectedSourceRevision = "952bd331976c6f28efb36ca320c82c27e2456023"

$downloadUrl = "https://github.com/$releaseRepo/releases/download/$releaseTag/$assetName"
$targetDir = Join-Path $repositoryRoot "build\uassettool"
$targetExe = Join-Path $targetDir "UAssetTool.exe"
$zipPath = Join-Path $targetDir $assetName

function Test-PinnedChecksum {
    param([Parameter(Mandatory)][string]$Path)

    if (-not (Test-Path $Path)) {
        return $false
    }
    $actual = (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    return $actual -eq $expectedSha256
}

New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

if ((Test-Path $targetExe) -and (Test-PinnedChecksum -Path $zipPath)) {
    Write-Host "==> Pinned worker already present and verified: $targetExe"
}
else {
    Write-Host "==> Downloading $assetName from $releaseRepo@$releaseTag"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

    Write-Host "==> Verifying SHA-256 against the pinned checksum"
    if (-not (Test-PinnedChecksum -Path $zipPath)) {
        $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
        Remove-Item -Path $zipPath -Force
        throw "Checksum mismatch for ${assetName}: expected $expectedSha256, got $actual. The downloaded file was deleted; do not trust a worker binary that fails this check."
    }

    Write-Host "==> Extracting to $targetDir"
    Expand-Archive -Path $zipPath -DestinationPath $targetDir -Force
}

Write-Host "==> Confirming the worker reports the pinned source revision"
$versionOutput = (& $targetExe --version) -join "`n"
if ($LASTEXITCODE -ne 0) {
    throw "UAssetTool.exe --version exited with code $LASTEXITCODE."
}
if ($versionOutput -notmatch [regex]::Escape($expectedSourceRevision)) {
    throw "Worker reported version '$versionOutput', which does not contain the pinned source revision $expectedSourceRevision. The release asset may have been rebuilt or replaced; re-verify before trusting it."
}

Write-Host "Pinned worker verified: $versionOutput"
