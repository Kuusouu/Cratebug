# Cratebug user guide

This covers installing Cratebug, keeping it updated, installing mods from a URL, and acting on several mods at once (including encrypting complete IoStore bundles). For other everyday library management, the app itself is the reference. If something goes wrong, see [Troubleshooting](TROUBLESHOOTING.md).

## Installing

1. Download `Cratebug-amd64-installer.exe` from the [latest release](https://github.com/Kuusouu/Cratebug/releases/latest).
2. Run it. It installs to `%LOCALAPPDATA%\Programs\Cratebug` for your user account only — no administrator rights needed.
3. Launch Cratebug from the Start Menu or the desktop shortcut the installer creates.

Cratebug isn't code-signed yet, so Windows SmartScreen may show an "unrecognized app" warning on first run. Click **More info** then **Run anyway**.

## Finding your mod library

On first launch, Cratebug can find your Marvel Rivals mod library for you — no path hunting required.

1. Open **Settings** (gear icon) and pick **Steam** or **Epic Games** under **Mod library detection**. Steam is the default.
2. Click **Detect Steam library** or **Detect Epic Games library** in the toolbar at the top (or the same button in the middle of the screen when no library is set).
3. If the game is installed through that store, Cratebug finds it and points your library at its `~mods` folder.
4. If the game is there but the `~mods` folder doesn't exist yet, Cratebug asks first: **Create library** makes exactly that one empty folder, nothing else.

You can always paste a folder path into the **Mod library folder** box instead — both paths end at the same scan.

## Updating

Cratebug doesn't update itself silently in the background — you're always the one who decides when to check.

1. Open **Settings** (gear icon in the header).
2. Under **Updates**, click **Check for updates**.
3. If a newer version exists, a changelog opens showing what's new. Click **Download update**, then **Install & restart** once it's ready.

Cratebug closes itself, applies the update silently, and reopens automatically — no installer window, no extra clicks. The first time you open the app after an update, you'll see the same changelog again as a one-time "what's new" notice.

If you'd rather update manually, the **View release** button opens the GitHub release page, where you can download and run the installer yourself.

## Installing a mod from a URL

If you have a direct download link to a mod archive (a `.zip`, `.7z`, `.rar`, or a bare `.pak`/`.utoc`/`.ucas`), you don't need to download it yourself first.

1. Click the link icon in the header (**Install from URL**).
2. Paste the direct download link. It must start with `https://`.
3. Click **Download & install**.

Cratebug downloads the file and takes you straight to the same install preview you'd get from picking a local file — same collision checks, same hero/skin detection, same control over the destination folder and mod name before anything is actually installed.

A link that requires clicking through a webpage (like a mod page's "Download" button that lands on another page) won't work directly. You need the URL the browser actually downloads from, not the page that links to it.

## Checking several mods at once

Clicking a card opens it in the details panel and makes it the only checked mod. Click that same card again to deselect it.

- Hold Ctrl and click another card to add or remove it without clearing the rest.
- Shift+click selects every visible mod from the last checked one to the one you clicked. Shift+Ctrl+click removes that range.
- **Select all** checks every mod in the current folder and search filter. **Clear** empties the set and deselects the viewed card.

The catalog header shows how many mods are checked. The **Actions** menu (three-dot button next to Tags) runs Enable, Disable, Move, Tags, Encrypt/Decrypt, or Delete on that set. Right-click selects that card if it was not already checked, then offers rename, priority, move, tags, and delete for that one row.

A batch that only partly succeeds says so. It does not call the whole run a success.

## Encrypting or decrypting IoStore mods

Encrypt and Decrypt live only in the Actions menu. They rebuild each complete IoStore bundle (`.pak` + `.utoc` + `.ucas`) so the Marvel Rivals game key wraps the container. Classic PAK mods cannot be encrypted. A mix of encrypted and unencrypted IoStore mods disables the action until the set is uniform.

Encryption is a rebuild, not a bit-flip. Large mods can take several minutes. Close Marvel Rivals first. A failed rebuild leaves that one mod as it was. Mods that already finished in the same batch stay changed. The rebuilt companion `.pak` is stripped of `chunknames` and `patched_files` before it replaces the live files.

A lock mark on a card means that IoStore bundle is encrypted. The category pill (Mesh, UI, and so on) does not change.

## Unsupported companion PAK entries

As of 3 September 2026, leftover `chunknames` and `patched_files` entries inside a companion `.pak` crash Marvel Rivals anti-cheat. The first time Cratebug loads a library in a session, it lists those files and asks to rewrite the affected `.pak` files. The IoStore `.utoc` and `.ucas` stay as they are. You can decline for this session. A later Refresh does not ask again.

Install preview shows the same warning. Installing still proceeds, and Cratebug rewrites those staged `.pak` files before they land in the library. Close the game first. A failed rewrite leaves that one mod as it was.
