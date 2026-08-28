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

## [2026.08.28]

### Finding your crate

- No more path hunting. Cratebug can now find your Marvel Rivals install through Steam all by itself and point your library at the right folder. First launch, click Detect, done.
- If your `~mods` folder doesn't exist yet, Cratebug asks first and then creates exactly that one folder. Nothing else is touched.
- You can pick the store Cratebug searches through in Settings. Epic Games detection is coming soon! Until then, pasting a folder path to manage your crates works the same as before.

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
