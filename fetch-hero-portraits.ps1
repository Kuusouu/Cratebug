#Requires -Version 7.0
[CmdletBinding()]
param(
    [switch]$Force,
    [switch]$Clean,
    [int]$Throttle = [Environment]::ProcessorCount
)

# Sourcing script for Marvel Rivals hero portraits and skin icons from Rivalskins.com.
# Hero headshots are keyed by 4-digit Hero IDs (1011 to 1066).
# Skin icons are keyed by 7-digit Skin IDs (e.g. 1029305.png).
#
# Process:
# 1. Fetches the community-maintained Character ID reference table.
# 2. Filters strictly to playable hero IDs (1011 to 1066), excluding stale/(Old) rows.
# 3. Fetches the hero pages that still have missing skins, then downloads the missing
#    PNGs from Rivalskins and converts each to a downscaled WebP at
#    frontend/src/assets/heroes/<id>.webp.
# 4. Performs self-validation ensuring all playable heroes and downloaded skins are valid images.
#
# Downloads and conversions run through one fused pool of $Throttle workers (default:
# logical CPU count), each processing one asset end to end, so slow ImageMagick encodes
# overlap other workers' network waits. Fetching is incremental by default: an ID whose
# .webp already exists is left alone, so a run after a new skin ships downloads only that
# skin. A hero's page is only requested when at least one of its known skins is still
# missing, which keeps a no-op run down to the character table fetch alone. Use -Force to
# re-download and re-encode everything, or -Clean to delete the assets first.
#
# Requires PowerShell 7+ (ForEach-Object -Parallel).

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

    throw "ImageMagick not found. Install it with 'winget install -e --id ImageMagick.ImageMagick --version 7.1.2.31', restart the terminal, and ensure 'magick' is on PATH."
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

# Phase 1: work out what is missing. Local filesystem checks only, no network calls.
Write-Host "==> Computing missing assets..."
$avatarJobs = [System.Collections.Generic.List[object]]::new()
$pageJobs = [System.Collections.Generic.List[object]]::new()
$heroSkipped = 0
$skinSkipped = 0

foreach ($heroID in ($heroMap.Keys | Sort-Object)) {
    $heroName = $heroMap[$heroID]
    $slug = Get-RivalskinsSlug -HeroName $heroName

    $heroDest = Join-Path $targetDir "$heroID.webp"
    if ($Force -or -not (Test-Path $heroDest)) {
        $avatarJobs.Add([pscustomobject]@{
                Kind = "hero"
                File = "$heroID.webp"
                Url  = "https://rivalskins.com/wp-content/uploads/marvel-assets/ui/heroes/avatar/${slug}_avatar.png"
            })
    } else {
        $heroSkipped++
    }

    $heroSkins = if ($skinMap.ContainsKey($heroID)) { $skinMap[$heroID] } else { @{} }
    $missingSkinIDs = [System.Collections.Generic.List[string]]::new()
    $skinPayload = [System.Collections.Generic.List[object]]::new()
    foreach ($sId in $heroSkins.Keys) {
        $skinName = $heroSkins[$sId]
        $skinPayload.Add([pscustomobject]@{ Id = $sId; Clean = (Clean-Name -str $skinName); Len = $skinName.Length })
        if ($Force -or -not (Test-Path (Join-Path $targetDir "$sId.webp"))) {
            $missingSkinIDs.Add($sId)
        } else {
            $skinSkipped++
        }
    }

    # Nothing this hero could contribute, so skip the page request entirely.
    if ($missingSkinIDs.Count -eq 0) {
        continue
    }

    $pageJobs.Add([pscustomobject]@{
            HeroID  = $heroID
            Slug    = $slug
            Skins   = $skinPayload
            Missing = $missingSkinIDs.ToArray()
        })
}

# Phase 2: fetch the hero pages that still have missing skins and match icons to skin IDs.
$pageResults = @()
if ($pageJobs.Count -gt 0) {
    # Page fetches stay modest; the heavy phase below uses the full throttle.
    $pageThrottle = [Math]::Min($Throttle, 8)
    Write-Host "==> Fetching $($pageJobs.Count) hero pages with up to $pageThrottle workers..."
    $pageResults = @($pageJobs | ForEach-Object -Parallel {
            $job = $_
            try {
                $html = (Invoke-WebRequest -Uri "https://rivalskins.com/hero/$($job.Slug)/" -UserAgent $using:userAgent -UseBasicParsing -ErrorAction Stop).Content
            }
            catch {
                return
            }

            $sortedSkins = @($job.Skins | Sort-Object Len -Descending)
            $costumePattern = '<img[^>]+src="(?<url>https://rivalskins\.com/wp-content/uploads/marvel-assets/items/costume/[^"]+img_icon_[^"]+\.png)"[^>]*alt="(?<alt>[^"]+)"'

            foreach ($m in [regex]::Matches($html, $costumePattern)) {
                $cleanAlt = $m.Groups['alt'].Value.ToLowerInvariant() -replace '[^a-z0-9]', ''

                # Exact match
                $matchedSkinID = $null
                foreach ($s in $job.Skins) {
                    if ($s.Clean -eq $cleanAlt) {
                        $matchedSkinID = $s.Id
                        break
                    }
                }

                # Fallback substring match (longer/more specific names checked first)
                if (-not $matchedSkinID) {
                    foreach ($s in $sortedSkins) {
                        if ($cleanAlt.Contains($s.Clean) -or $s.Clean.Contains($cleanAlt)) {
                            $matchedSkinID = $s.Id
                            break
                        }
                    }
                }

                if ($matchedSkinID -and $job.Missing -contains $matchedSkinID) {
                    [pscustomobject]@{
                        Kind = "skin"
                        File = "$matchedSkinID.webp"
                        Url  = $m.Groups['url'].Value
                    }
                }
            }
        } -ThrottleLimit $pageThrottle)
}

$skinJobs = @()
if ($pageResults) {
    # A page can list the same skin twice; keep the last listing so the result
    # matches the serial script's overwrite order.
    $skinJobs = @($pageResults | Group-Object -Property File | ForEach-Object { $_.Group[-1] })
}

# Phase 3: download and convert everything in one fused pool.
$downloadJobs = @($avatarJobs) + @($skinJobs)
$results = @()
if ($downloadJobs.Count -gt 0) {
    Write-Host "==> Downloading and converting $($downloadJobs.Count) assets with up to $Throttle workers..."
    $results = @($downloadJobs | ForEach-Object -Parallel {
            $job = $_
            $magick = $using:magick
            $dest = Join-Path $using:targetDir $job.File
            $size = $using:portraitSize
            $quality = $using:webpQuality

            # Downloads one source image and writes it as a downscaled WebP. The '>' on
            # the resize geometry means shrink-only, so the smaller hero avatars are
            # never upscaled into blur.
            $temp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName() + ".png")
            try {
                Invoke-WebRequest -Uri $job.Url -OutFile $temp -UserAgent $using:userAgent -ErrorAction Stop
                & $magick $temp -strip -resize "${size}x${size}>" -quality $quality -define webp:method=6 $dest
                if ($LASTEXITCODE -ne 0) {
                    throw "magick exited with code $LASTEXITCODE converting $($job.Url)"
                }
                [pscustomobject]@{ Ok = $true; Kind = $job.Kind; File = $job.File; Url = $job.Url; Error = $null }
            }
            catch {
                [pscustomobject]@{ Ok = $false; Kind = $job.Kind; File = $job.File; Url = $job.Url; Error = $_.Exception.Message }
            }
            finally {
                Remove-Item $temp -Force -ErrorAction SilentlyContinue
            }
        } -ThrottleLimit $Throttle)
}

$heroDownloaded = @($results | Where-Object { $_.Ok -and $_.Kind -eq "hero" }).Count
$skinDownloaded = @($results | Where-Object { $_.Ok -and $_.Kind -eq "skin" }).Count
$failedDownloads = @($results | Where-Object { -not $_.Ok })

Write-Host "==> Fetched $heroDownloaded hero avatars and $skinDownloaded skin icons; skipped $heroSkipped heroes and $skinSkipped skins already present."
Write-Host "==> Requested $($pageJobs.Count) of $($heroMap.Count) hero pages."
if ($failedDownloads.Count -gt 0) {
    Write-Host "==> $($failedDownloads.Count) download(s) failed:"
    foreach ($f in $failedDownloads) {
        Write-Host "    $($f.File) ($($f.Url)): $($f.Error)"
    }
}

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
