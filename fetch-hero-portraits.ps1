[CmdletBinding()]
param(
    [switch]$Force,
    [switch]$Clean
)

# Sourcing script for Marvel Rivals hero portraits and skin icons from Rivalskins.com.
# Hero headshots are keyed by 4-digit Hero IDs (1011 to 1066).
# Skin icons are keyed by 7-digit Skin IDs (e.g. 1029305.png).
#
# Process:
# 1. Fetches the community-maintained Character ID reference table.
# 2. Filters strictly to playable hero IDs (1011 to 1066), excluding stale/(Old) rows.
# 3. Downloads the official PNG hero headshots and skin icons directly from Rivalskins,
#    converting each to a downscaled WebP at frontend/src/assets/heroes/<id>.webp.
# 4. Performs self-validation ensuring all playable heroes and downloaded skins are valid images.
#
# Fetching is incremental by default: an ID whose .webp already exists is left
# alone, so a run after a new skin ships downloads only that skin. A hero's page
# is only requested when at least one of its known skins is still missing, which
# keeps a no-op run down to the character table fetch alone. Use -Force to
# re-download and re-encode everything, or -Clean to delete the assets first.

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = $PSScriptRoot
$targetDir = Join-Path $repositoryRoot "frontend\src\assets\heroes"

$portraitSize = 128
$webpQuality = 82
$userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"

function Resolve-Magick {
    $onPath = Get-Command magick -ErrorAction SilentlyContinue
    if ($onPath) { return $onPath.Source }

    $resolved = & mise which magick 2>$null | Select-Object -First 1
    if ($resolved -and (Test-Path $resolved)) { return $resolved }

    throw "ImageMagick not found. Run 'mise install' from the repository root, or put magick on PATH."
}

# Downloads one source image and writes it as a downscaled WebP. The '>' on the
# resize geometry means shrink-only, so the smaller hero avatars are never
# upscaled into blur.
function Save-Portrait {
    param(
        [Parameter(Mandatory)][string]$Url,
        [Parameter(Mandatory)][string]$Destination
    )

    $temp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName() + ".png")
    try {
        Invoke-WebRequest -Uri $Url -OutFile $temp -UserAgent $userAgent -ErrorAction Stop
        & $magick $temp -strip -resize "$($portraitSize)x$($portraitSize)>" -quality $webpQuality -define webp:method=6 $Destination
        if ($LASTEXITCODE -ne 0) {
            throw "magick exited with code $LASTEXITCODE converting $Url"
        }
    }
    finally {
        Remove-Item $temp -Force -ErrorAction SilentlyContinue
    }
}

$magick = Resolve-Magick

if ($Clean -and (Test-Path $targetDir)) {
    Write-Host "==> Cleaning existing hero/skin portrait assets..."
    Get-ChildItem -Path $targetDir -Include "*.png", "*.webp" -Recurse | Remove-Item -Force
}

New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

$characterTableUrl = "https://raw.githubusercontent.com/donutman07/MarvelRivalsCharacterIDs/main/MarvelRivalsCharacterIDs.md"

Write-Host "==> Fetching character table from reference repository..."
$markdown = (Invoke-WebRequest -Uri $characterTableUrl -UseBasicParsing).Content

function Test-IsPlayableHero {
    param([string]$ID, [string]$Name)
    if ($ID -notmatch '^10[1-6]\d$') { return $false }
    if ($ID -lt '1011' -or $ID -gt '1066') { return $false }
    if ($Name -match '\(Old\)|\(For Dev\)|Proxy|Upcoming') { return $false }
    return $true
}

# Parse character ID -> Name and skin ID -> Name mappings
$heroMap = @{}
$skinMap = @{}
$currentHeroID = ""

$lines = $markdown -split "`r?`n"
foreach ($line in $lines) {
    if ($line -match '^\s*\|\s*(\d{4})\s*\|\s*([^\|]+)\s*\|\s*(\d{7})?\s*\|\s*([^\|]+)?') {
        $id = $matches[1].Trim()
        $name = $matches[2].Trim()
        if (Test-IsPlayableHero -ID $id -Name $name) {
            if (-not $heroMap.ContainsKey($id)) {
                $heroMap[$id] = $name
            }
            $currentHeroID = $id
            if (-not $skinMap.ContainsKey($id)) { $skinMap[$id] = @{} }
            if ($matches[3] -and $matches[4]) {
                $skinID = $matches[3].Trim()
                $skinName = $matches[4].Trim()
                if (-not $skinMap[$id].ContainsKey($skinID)) {
                    $skinMap[$id][$skinID] = $skinName
                }
            }
        } else {
            $currentHeroID = ""
        }
    }
    elseif ($line -match '^\s*\|\s*\|\s*\|\s*(\d{7})\s*\|\s*([^\|]+)') {
        if ($currentHeroID -and $skinMap.ContainsKey($currentHeroID)) {
            $skinID = $matches[1].Trim()
            $skinName = $matches[2].Trim()
            if (-not $skinMap[$currentHeroID].ContainsKey($skinID)) {
                $skinMap[$currentHeroID][$skinID] = $skinName
            }
        }
    }
}

$totalSkinsInTable = ($skinMap.Values | ForEach-Object { $_.Count } | Measure-Object -Sum).Sum
Write-Host "==> Parsed $($heroMap.Count) playable heroes (1011-1066) and $totalSkinsInTable skins."

function Get-RivalskinsSlug {
    param([string]$HeroName)
    $normalized = $HeroName.Trim().ToLowerInvariant()
    switch -Regex ($normalized) {
        "^cloak & dagger$" { return "cloak-and-dagger" }
        "^jeff the land" { return "jeff-the-land-shark" }
        "^elsa bloodstone$" { return "elsa-bloodstone" }
        "^mr\.?\s*fantastic|^mister fantastic" { return "mister-fantastic" }
        "^iron fist" { return "iron-fist" }
        "^punisher$|^the punisher$" { return "the-punisher" }
        "^the hood$" { return "the-hood" }
        "^the thing$" { return "the-thing" }
        "^white fox$" { return "white-fox" }
        "^black cat$" { return "black-cat" }
        "^black panther$" { return "black-panther" }
        "^black widow$" { return "black-widow" }
        "^captain america$" { return "captain-america" }
        "^devil dinosaur$" { return "devil-dinosaur" }
        "^doctor strange$" { return "doctor-strange" }
        "^emma frost$" { return "emma-frost" }
        "^human torch$" { return "human-torch" }
        "^luna snow$" { return "luna-snow" }
        "^moon knight$" { return "moon-knight" }
        "^peni parker$" { return "peni-parker" }
        "^rocket raccoon$" { return "rocket-raccoon" }
        "^scarlet witch$" { return "scarlet-witch" }
        "^spider-man$" { return "spider-man" }
        "^squirrel girl$" { return "squirrel-girl" }
        "^star-lord$" { return "star-lord" }
        "^winter soldier$" { return "winter-soldier" }
        "^adam warlock$" { return "adam-warlock" }
        default {
            return ($normalized -replace '[^a-z0-9]+', '-').Trim('-')
        }
    }
}

function Clean-Name {
    param([string]$str)
    return ($str.ToLowerInvariant() -replace '[^a-z0-9]', '')
}

Write-Host "==> Downloading missing hero avatars and skin icons from Rivalskins..."
$heroDownloaded = 0
$heroSkipped = 0
$skinDownloaded = 0
$skinSkipped = 0
$heroesQueried = 0
$failedHeroes = @()

foreach ($heroID in ($heroMap.Keys | Sort-Object)) {
    $heroName = $heroMap[$heroID]
    $slug = Get-RivalskinsSlug -HeroName $heroName
    
    # Download base hero avatar
    $heroDest = Join-Path $targetDir "$heroID.webp"
    if ($Force -or -not (Test-Path $heroDest)) {
        $avatarUrl = "https://rivalskins.com/wp-content/uploads/marvel-assets/ui/heroes/avatar/${slug}_avatar.png"
        try {
            Save-Portrait -Url $avatarUrl -Destination $heroDest
            $heroDownloaded++
        }
        catch {
            $failedHeroes += "$heroID ($heroName -> $slug)"
        }
    } else {
        $heroSkipped++
    }

    $heroSkins = if ($skinMap.ContainsKey($heroID)) { $skinMap[$heroID] } else { @{} }

    # The skin IDs this hero still needs. Resolved before any network call so a
    # hero whose skins are all present costs nothing at all.
    $missingSkinIDs = New-Object 'System.Collections.Generic.HashSet[string]'
    foreach ($sId in $heroSkins.Keys) {
        if ($Force -or -not (Test-Path (Join-Path $targetDir "$sId.webp"))) {
            [void]$missingSkinIDs.Add($sId)
        } else {
            $skinSkipped++
        }
    }

    # Nothing this hero could contribute, so skip the page request entirely.
    if ($missingSkinIDs.Count -eq 0) {
        continue
    }

    # Fetch hero page to discover costume skin icons
    $heroesQueried++
    $heroPageUrl = "https://rivalskins.com/hero/${slug}/"
    $heroHtml = $null
    try {
        $heroHtml = (Invoke-WebRequest -Uri $heroPageUrl -UserAgent $userAgent -UseBasicParsing -ErrorAction Stop).Content
    }
    catch {
        continue
    }

    $costumePattern = '<img[^>]+src="(?<url>https://rivalskins\.com/wp-content/uploads/marvel-assets/items/costume/[^"]+img_icon_[^"]+\.png)"[^>]*alt="(?<alt>[^"]+)"'
    $matchesPattern = [regex]::Matches($heroHtml, $costumePattern)

    foreach ($m in $matchesPattern) {
        $altName = $m.Groups['alt'].Value
        $iconUrl = $m.Groups['url'].Value
        $cleanAlt = Clean-Name -str $altName

        # Exact match
        $matchedSkinID = $null
        foreach ($sId in $heroSkins.Keys) {
            $cleanSkin = Clean-Name -str $heroSkins[$sId]
            if ($cleanSkin -eq $cleanAlt) {
                $matchedSkinID = $sId
                break
            }
        }

        # Fallback substring match (longer/more specific names checked first)
        if (-not $matchedSkinID) {
            $sortedKeys = $heroSkins.Keys | Sort-Object { $heroSkins[$_].Length } -Descending
            foreach ($sId in $sortedKeys) {
                $cleanSkin = Clean-Name -str $heroSkins[$sId]
                if ($cleanAlt.Contains($cleanSkin) -or $cleanSkin.Contains($cleanAlt)) {
                    $matchedSkinID = $sId
                    break
                }
            }
        }

        if ($matchedSkinID -and $missingSkinIDs.Contains($matchedSkinID)) {
            $skinDest = Join-Path $targetDir "$matchedSkinID.webp"
            try {
                Save-Portrait -Url $iconUrl -Destination $skinDest
                $skinDownloaded++
            }
            catch {
                # Skip download failure for individual icon
            }
        }
    }
}

Write-Host "==> Fetched $heroDownloaded hero avatars and $skinDownloaded skin icons; skipped $heroSkipped heroes and $skinSkipped skins already present."
Write-Host "==> Requested $heroesQueried of $($heroMap.Count) hero pages."

# Built-in Self-Validation
Write-Host "==> Running self-validation on downloaded assets..."
$validationErrors = @()

foreach ($heroID in ($heroMap.Keys | Sort-Object)) {
    $heroFile = Join-Path $targetDir "$heroID.webp"
    if (-not (Test-Path $heroFile)) {
        $validationErrors += "Missing hero avatar: $heroID.png ($($heroMap[$heroID]))"
    } elseif ((Get-Item $heroFile).Length -eq 0) {
        $validationErrors += "Empty hero avatar file: $heroID.png"
    }
}

$allPortraitFiles = Get-ChildItem -Path $targetDir -Filter "*.webp"
foreach ($file in $allPortraitFiles) {
    if ($file.Length -eq 0) {
        $validationErrors += "Corrupt/empty image: $($file.Name)"
    }
}

if ($validationErrors.Count -gt 0) {
    Write-Error "Self-validation failed with $($validationErrors.Count) error(s):`n$($validationErrors -join "`n")"
    exit 1
}

Write-Host "Self-validation passed: all $($heroMap.Count) playable heroes and $($allPortraitFiles.Count - $heroMap.Count) skins verified."
Write-Host "Hero & Skin assets ready: $($allPortraitFiles.Count) images in $targetDir"
