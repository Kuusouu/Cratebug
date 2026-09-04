# Changelog

All notable changes to Cratebug are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Cratebug uses CalVer (`YYYY.MM.DD`, with an optional `-rcN` suffix for
prerelease builds) instead of Semantic Versioning: releases are dated, not
numbered by compatibility impact.

Each version's section below is what the release workflow copies into that
version's GitHub release notes, so write entries for what a user would
actually notice, not internal refactors.

## [Unreleased]

## [2026.09.03]

### Finding your crates

- Epic Games is in! Pick it in Settings and Cratebug will hunt down your Marvel Rivals install from the Epic launcher, same as it already does for Steam. If the mod folder isn't there yet, it asks before creating it.

### Settling in

- The window now opens at a size that actually fits your screen, so a 1080p laptop at 125% scaling no longer has to be maximized just to see the whole crate.
- List view dropped the leftover scrollbar gutter. A short list stays clean instead of parking an empty bar down the side.

## [2026.08.30]

### Making it yours

- Your accent color now runs the whole show: the enable switches and the little X in search dress to match instead of doing their own thing.

### Keeping the crate tidy

- New "Library root" view in the sidebar! See only the mods sitting loose in the main folder, and drag a mod onto it to send it back home.
- Folders can finally be deleted, with the usual safety net. A moment to think it over, and the whole folder waits in the Recycle Bin until you empty it.
- Deleting a full folder tells you its contents go with it, and that everything can be restored if you change your mind.
- Creating a folder keeps you right where you were, so stacking up folders no longer yanks you out of your place.

## [2026.08.27]

Welcome to Cratebug! First release, and the whole crate is stocked.

### Managing your crates

- Browse your whole library at a glance, classic mods and IoStore mods alike.
- Flip mods on and off, rename them, reorder priority, sort them into folders, and delete with an actual safety net (Recycle Bin, not the void).
- Tag your crates however makes sense to you. Tags and settings stick around even after a rename or move.
- Every mod gets auto-identified: hero, skin, category, portrait and all, so your library actually looks like a library.

### Getting new crates in

- Drop in a `.zip`, `.7z`, `.rar`, or a bare `.pak` and Cratebug unpacks it and shows you exactly what's about to happen before it happens.
- Found a mod online? Paste the link and Cratebug fetches it for you, no manual download detour.

### Keeping the crate tidy

- Catch mods stepping on each other before they cause chaos in-game, with a one-click priority fix right where you spot the conflict.
- Cratebug can now update itself. Click, download, restart, done.
