[CmdletBinding()]
param()

# Fetches and verifies the pinned UTOC signature bypass release for Marvel
# Rivals. The payload is third-party tooling that belongs with the game, not
# Cratebug: Ultimate ASI Loader (MIT, ThirteenAG) and the signature bypass
# plugin (LGPL-2.1, DeathChaos25). Releases bundle the fetched files the same
# way the pinned UAssetTool worker is bundled; neither payload is committed.
#
# The release zip contains:
#   dsound.dll - Ultimate ASI Loader 7.7.0 renamed so the game loads it
#   plugins\MarvelRivalsUTOCSignatureBypass.asi - the bypass plugin
#   Ultimate ASI Loader License.txt - the loader's MIT license text
#
# The release does not ship the plugin's own license text, so the script also
# fetches it from the release tag. The LGPL-2.1 copy must travel with the
# installed payload for redistribution.
#
# Distribution note: the author publishes this build on their own GitHub
# releases. Cratebug fetches from that release instead of re-hosting it, so a
# version bump is an edit to the pins below, followed by a Cratebug release.

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = $PSScriptRoot
$releaseRepo = "DeathChaos25/MarvelRivalsUTOCSignatureBypass"
$releaseTag = "1.0.0"
$assetName = "Marvel.Rivals.UTOC.Signature.Bypass.Patch.zip"
$expectedSha256 = "4d01514adc70629628a0b89d37ed24e4805a879eabef0f48267a263f37f3d70b"
$licenseName = "MarvelRivalsUTOCSignatureBypass License.txt"
$expectedLicenseSha256 = "20c17d8b8c48a600800dfd14f95d5cb9ff47066a9641ddeab48dc54aec96e331"

$downloadUrl = "https://github.com/$releaseRepo/releases/download/$releaseTag/$assetName"
$licenseUrl = "https://raw.githubusercontent.com/$releaseRepo/$releaseTag/LICENSE.txt"
$targetDir = Join-Path $repositoryRoot "build\sigbypass"
$zipPath = Join-Path $targetDir $assetName
$licensePath = Join-Path $targetDir $licenseName
$expectedFiles = @(
    "dsound.dll",
    "plugins\MarvelRivalsUTOCSignatureBypass.asi",
    "Ultimate ASI Loader License.txt",
    $licenseName
)

function Test-PinnedSHA256 {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$Expected
    )

    if (-not (Test-Path $Path)) {
        return $false
    }
    $actual = (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    return $actual -eq $Expected.ToLowerInvariant()
}

function Test-PayloadComplete {
    foreach ($file in $expectedFiles) {
        if (-not (Test-Path (Join-Path $targetDir $file))) {
            return $false
        }
    }
    return $true
}

New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

if ((Test-PinnedSHA256 -Path $zipPath -Expected $expectedSha256) -and (Test-PayloadComplete)) {
    Write-Host "==> Pinned signature bypass already present and verified: $targetDir"
}
else {
    Write-Host "==> Downloading $assetName from $releaseRepo@$releaseTag"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

    Write-Host "==> Verifying SHA-256 against the pinned checksum"
    if (-not (Test-PinnedSHA256 -Path $zipPath -Expected $expectedSha256)) {
        $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
        Remove-Item -Path $zipPath -Force
        throw "Checksum mismatch for ${assetName}: expected $expectedSha256, got $actual. The downloaded file was deleted; do not trust a signature bypass payload that fails this check."
    }

    Write-Host "==> Extracting to $targetDir"
    Expand-Archive -Path $zipPath -DestinationPath $targetDir -Force

    Write-Host "==> Downloading the bypass plugin license from $releaseRepo@$releaseTag"
    Invoke-WebRequest -Uri $licenseUrl -OutFile $licensePath

    Write-Host "==> Verifying SHA-256 against the pinned license checksum"
    if (-not (Test-PinnedSHA256 -Path $licensePath -Expected $expectedLicenseSha256)) {
        $actual = (Get-FileHash -Path $licensePath -Algorithm SHA256).Hash.ToLowerInvariant()
        Remove-Item -Path $licensePath -Force
        throw "Checksum mismatch for ${licenseName}: expected $expectedLicenseSha256, got $actual. The downloaded file was deleted; do not trust a signature bypass license that fails this check."
    }
}

foreach ($file in $expectedFiles) {
    if (-not (Test-Path (Join-Path $targetDir $file))) {
        throw "Expected $file in $targetDir after extraction."
    }
}

Write-Host "Pinned signature bypass verified: $targetDir"
