# Troubleshooting

Common problems and fixes. For how things work, see the [user guide](USER_GUIDE.md).

## Windows showed an "unrecognized app" warning

Cratebug is not code-signed yet, so SmartScreen steps in on first run. Click **More info**, then **Run anyway**. You only need to do this once per install.

## The app doesn't start

Cratebug renders through Microsoft's WebView2 Runtime. Windows 11 ships with it; some Windows 10 systems do not have it. Install it from [Microsoft's WebView2 page](https://developer.microsoft.com/microsoft-edge/webview2/) and launch Cratebug again.

## Where Cratebug is installed, and how to remove it

Cratebug installs to `%LOCALAPPDATA%\Programs\Cratebug` for your user account only. Uninstall from **Windows Settings > Apps**, or run `uninstall.exe` from that folder. Uninstalling removes Cratebug itself; the mods in your mod library folder are left alone.

## Auto-detect can't find the game

The **Detect library** button searches the store selected in Settings > **Mod library detection**.

Steam: searches your Steam installation and every Steam library folder for Marvel Rivals.

Epic Games: reads the Epic Games launcher's install records (`%ProgramData%\Epic\EpicGamesLauncher\Data\Manifests`) and looks for Marvel Rivals. The game's folder name often has a random suffix; Cratebug uses the path the launcher recorded, not a hardcoded folder name.

If it still reports it couldn't find one: make sure the game is installed through that store, then try again. You can always paste the mod folder path (the game's `Paks\~mods` folder) into the **Mod library folder** box and scan; that always works.

## Check for updates says I'm up to date

Then you are on the latest release. You can always browse what is published on the [releases page](https://github.com/Kuusouu/Cratebug/releases).

## An update failed to download or install

The app reports download and install failures as toasts - note the message, then try again. If it keeps failing, use the **View release** button (or the [releases page](https://github.com/Kuusouu/Cratebug/releases/latest)) to download `Cratebug-amd64-installer.exe` and run it yourself; it installs over the existing copy and keeps your settings and tags.

## Install from URL says it could not determine a file name

The link must be the direct download - the URL the browser actually saves the file from - and it must start with `https://`. A link to a mod *page* (where you click a Download button) won't work; copy the file's direct link instead. Supported targets are `.zip`, `.7z`, `.rar` archives and bare `.pak`/`.utoc`/`.ucas` files.

## Encrypt or Decrypt is greyed out

The Actions menu only encrypts complete IoStore mods (`.pak` + `.utoc` + `.ucas`). Classic PAK mods, incomplete bundles, and orphaned sidecars are ineligible. If the checked set mixes encrypted and unencrypted IoStore mods, the action stays disabled until you check only one kind.

## Encrypt or Decrypt says the game is running

Cratebug will not rebuild a live bundle while Marvel Rivals is running. Close the game, then try again.

## An encrypt or decrypt failed partway through

Each mod is rebuilt on its own. A failure leaves that mod as it was. Mods that already finished stay encrypted or decrypted. Read the toast, then retry the ones that failed. If a hybrid mod (audio or other raw files next to Unreal assets) fails with a message about raw files that could not be extracted, stop and report it rather than retrying blindly.

## Something else

Open an issue on the [issue tracker](https://github.com/Kuusouu/Cratebug/issues) with what you did, what you expected, and what happened.
