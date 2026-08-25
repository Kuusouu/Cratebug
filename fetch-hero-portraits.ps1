[CmdletBinding()]
param(
    [switch]$Force,
    [switch]$NoClean
)

# Sourcing script for Marvel Rivals hero portraits and skin icons from Rivalskins.com.
# Hero headshots are keyed by 4-digit Hero IDs (1011 to 1066).
# Skin icons are keyed by 7-digit Skin IDs (e.g. 1029305.png).
#
# Process:
# 1. Fetches the community-maintained Character ID reference table.
# 2. Filters strictly to playable hero IDs (1011 to 1066), excluding stale/(Old) rows.
# 3. Downloads the official PNG hero headshots and skin icons directly from Rivalskins
#    into frontend/src/assets/heroes/<id>.png.
# 4. Performs self-validation ensuring all playable heroes and downloaded skins are valid PNGs.

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = $PSScriptRoot
$targetDir = Join-Path $repositoryRoot "frontend\src\assets\heroes"

if (-not $NoClean -and (Test-Path $targetDir)) {
    Write-Host "==> Cleaning existing hero/skin portrait assets..."
    Get-ChildItem -Path $targetDir -Filter "*.png" | Remove-Item -Force
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

Write-Host "==> Downloading hero avatars and skin icons from Rivalskins..."
$heroSuccess = 0
$skinSuccess = 0
$failedHeroes = @()

foreach ($heroID in ($heroMap.Keys | Sort-Object)) {
    $heroName = $heroMap[$heroID]
    $slug = Get-RivalskinsSlug -HeroName $heroName
    
    # 1. Download base hero avatar
    $heroDest = Join-Path $targetDir "$heroID.png"
    if ($Force -or -not (Test-Path $heroDest)) {
        $avatarUrl = "https://rivalskins.com/wp-content/uploads/marvel-assets/ui/heroes/avatar/${slug}_avatar.png"
        try {
            Invoke-WebRequest -Uri $avatarUrl -OutFile $heroDest -UserAgent "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" -ErrorAction Stop
            $heroSuccess++
        }
        catch {
            $failedHeroes += "$heroID ($heroName -> $slug)"
        }
    } else {
        $heroSuccess++
    }

    # 2. Fetch hero page to discover costume skin icons
    $heroPageUrl = "https://rivalskins.com/hero/${slug}/"
    $heroHtml = $null
    try {
        $heroHtml = (Invoke-WebRequest -Uri $heroPageUrl -UserAgent "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" -UseBasicParsing -ErrorAction Stop).Content
    }
    catch {
        continue
    }

    $costumePattern = '<img[^>]+src="(?<url>https://rivalskins\.com/wp-content/uploads/marvel-assets/items/costume/[^"]+img_icon_[^"]+\.png)"[^>]*alt="(?<alt>[^"]+)"'
    $matches = [regex]::Matches($heroHtml, $costumePattern)

    $heroSkins = if ($skinMap.ContainsKey($heroID)) { $skinMap[$heroID] } else { @{} }

    foreach ($m in $matches) {
        $altName = $m.Groups['alt'].Value
        $iconUrl = $m.Groups['url'].Value
        $cleanAlt = Clean-Name -str $altName

        # Pass 1: Exact match
        $matchedSkinID = $null
        foreach ($sId in $heroSkins.Keys) {
            $cleanSkin = Clean-Name -str $heroSkins[$sId]
            if ($cleanSkin -eq $cleanAlt) {
                $matchedSkinID = $sId
                break
            }
        }

        # Pass 2: Fallback substring match (longer/more specific names checked first)
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

        if ($matchedSkinID) {
            $skinDest = Join-Path $targetDir "$matchedSkinID.png"
            if ($Force -or -not (Test-Path $skinDest)) {
                try {
                    Invoke-WebRequest -Uri $iconUrl -OutFile $skinDest -UserAgent "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" -ErrorAction Stop
                    $skinSuccess++
                }
                catch {
                    # Skip download failure for individual icon
                }
            } else {
                $skinSuccess++
            }
        }
    }
}

Write-Host "==> Download summary: $heroSuccess heroes, $skinSuccess skin icons."

# 3. Built-in Self-Validation
Write-Host "==> Running self-validation on downloaded assets..."
$validationErrors = @()

foreach ($heroID in ($heroMap.Keys | Sort-Object)) {
    $heroFile = Join-Path $targetDir "$heroID.png"
    if (-not (Test-Path $heroFile)) {
        $validationErrors += "Missing hero avatar: $heroID.png ($($heroMap[$heroID]))"
    } elseif ((Get-Item $heroFile).Length -eq 0) {
        $validationErrors += "Empty hero avatar file: $heroID.png"
    }
}

$allPngFiles = Get-ChildItem -Path $targetDir -Filter "*.png"
foreach ($file in $allPngFiles) {
    if ($file.Length -eq 0) {
        $validationErrors += "Corrupt/empty image: $($file.Name)"
    }
}

if ($validationErrors.Count -gt 0) {
    Write-Error "Self-validation failed with $($validationErrors.Count) error(s):`n$($validationErrors -join "`n")"
    exit 1
}

Write-Host "Self-validation passed: all $($heroMap.Count) playable heroes and $($allPngFiles.Count - $heroMap.Count) skins verified."
Write-Host "Hero & Skin assets ready: $($allPngFiles.Count) images in $targetDir"
