# Cratebug user guide

This covers installing Cratebug, keeping it updated, and installing mods from a URL. For everyday library management (enabling mods, organizing folders, tags, conflict checking), the app itself is the reference — this guide only covers the parts that happen outside normal day-to-day use. If something goes wrong, see [Troubleshooting](TROUBLESHOOTING.md).

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

A link that requires clicking through a webpage (like a mod page's "Download" button that lands on another page) won't work directly — you need the URL the browser actually downloads from, not the page that links to it.
