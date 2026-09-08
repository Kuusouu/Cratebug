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

## [2026.09.08]

### Making it yours

- Feeling brave? Settings can skip the three-second countdown on delete, encrypt, and rebuild. You still confirm, the button just stops making you wait for it. Off by default!

### Opening the crates faster

- We've made improvements in how we handle classifying mods, resulting in a startup that is 8x faster than it was before!

### Lighter crates

- 85 MB down to about 36 MB, portraits and all. Updates grab the whole installer, so every one from here is a lighter haul!

## [2026.09.07]

### Getting new crates in

- Nexus Mods is in! Paste your personal API key in Settings. It stays on this machine, Cratebug never shows it again, and Connect is what makes it real. Premium and free accounts both work.
- The download icon in the header wants a Marvel Rivals mod page now, not a mystery file URL. If the page has more than one file, you pick MAIN or OPTIONAL. Cratebug pre-selects the primary file and does not install on its own.
- Premium accounts download through the Nexus API and land in the same install preview as a local archive.
- Free accounts still get the crate! Cratebug opens the Nexus page, you click Mod Manager Download, and Windows hands the link over. If Cratebug is closed, that click launches it. If it is already open, the window pops to the front.
- Turn on "Open Nexus downloads in Cratebug" and the Mod Manager Download button comes here. If another app already owns that button, Cratebug names it and asks before taking over.

### Several crates at once

- Check a bunch of mods and use the new Actions menu to enable, disable, move, tag, encrypt, or delete them together. Click a card to look at it. Ctrl+click to add more. Shift+click for a whole stretch. Select all grabs everything in the current folder and search. No little checkboxes parked on every card!

### A lock Marvel Rivals can actually open

- Encrypt or decrypt a complete IoStore mod from that same menu. Cratebug rebuilds the bundle with the game's own key so the game can load it, and a lock mark shows which crates are wrapped. Close the game first. Big mods can take a minute!
- Encrypted mods classify like everyone else now. Hero, skin, category, the works. They are not a mysterious Unknown anymore.
- Mods that wander outside character skins will not load until they are encrypted. On first load Cratebug offers to wrap those for you. Say not now if you want, and Refresh will not ask again.

### Keeping the crate tidy

- Leftover `chunknames` and `patched_files` entries inside a companion `.pak` crash Marvel Rivals anti-cheat these days. Cratebug spots them on first load and offers to rewrite just those `.pak` files. The rest of the mod stays put. Install preview warns too, and the cleanup happens before anything lands in the library.

### Settling in

- A library scan no longer leaves checks on mods that already left. If it is gone from the crate, it is gone from the checked set too.

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
