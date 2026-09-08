import { Check, Monitor, Moon, RefreshCw, RotateCcw, Sun, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
	ClearNexusAPIKey,
	NexusAccount,
	NexusProtocolStatus,
	RegisterNexusProtocol,
	SetNexusAPIKey,
	UnregisterNexusProtocol,
} from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { accentPresets, isValidHexColor } from "./accentColor";
import { formatWailsError } from "./installPresentation";
import {
	type LibraryProvider,
	libraryProviderLabels,
	libraryProviders,
	type Theme,
	themeLabels,
	themes,
} from "./libraryTypes";
import {
	formatNexusRateLimit,
	formatProtocolOwner,
	nexusAccountKind,
	nexusApiKeyPageURL,
	protocolSwitchDisabledReason,
} from "./nexusSettingsPresentation";
import styles from "./SettingsDialog.module.css";
import { providerLogos } from "./StoreLogos";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type SettingsDialogProps = {
	theme: Theme;
	accentColor: string;
	appVersion: string;
	isCheckingForUpdate: boolean;
	libraryProvider: LibraryProvider;
	skipConfirmDelay: boolean;
	onClose: () => void;
	onSelectTheme: (theme: Theme) => void;
	onSelectAccentColor: (hex: string) => void;
	onSelectLibraryProvider: (provider: LibraryProvider) => void;
	onToggleSkipConfirmDelay: (skip: boolean) => void;
	onCheckForUpdate: () => void;
};

const themeIcons = {
	system: Monitor,
	light: Sun,
	dark: Moon,
} satisfies Record<Theme, typeof Sun>;

// Every control here applies immediately, the same as ModTagDialog's
// checkboxes: there is nothing to buffer for a single-click preference like
// theme, so this has no Save/Cancel pair. Escape or the header X closes it.
export function SettingsDialog({
	theme,
	accentColor,
	appVersion,
	isCheckingForUpdate,
	libraryProvider,
	skipConfirmDelay,
	onClose,
	onSelectTheme,
	onSelectAccentColor,
	onSelectLibraryProvider,
	onToggleSkipConfirmDelay,
	onCheckForUpdate,
}: SettingsDialogProps) {
	const closeButtonRef = useRef<HTMLButtonElement>(null);
	const handleEscape = useCallback(() => onClose(), [onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const [hexDraft, setHexDraft] = useState(accentColor);
	const [account, setAccount] = useState<main.NexusAccountState | null>(null);
	const [protocol, setProtocol] = useState<main.NexusProtocolState | null>(null);
	const [keyDraft, setKeyDraft] = useState("");
	const [nexusBusy, setNexusBusy] = useState(false);
	const [nexusError, setNexusError] = useState("");
	const [takeOverOwner, setTakeOverOwner] = useState<string | null>(null);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	// Keeps the text field in sync when a preset swatch changes accentColor
	// out from under it, without fighting the user's own keystrokes: only
	// external changes (preset clicks, reset) should overwrite the draft.
	useEffect(() => {
		setHexDraft(accentColor);
	}, [accentColor]);

	function handleHexInput(value: string) {
		setHexDraft(value);
		if (isValidHexColor(value)) {
			onSelectAccentColor(value);
		}
	}

	const refreshNexus = useCallback(async () => {
		const [nextAccount, nextProtocol] = await Promise.all([
			NexusAccount(),
			NexusProtocolStatus(),
		]);
		setAccount(nextAccount);
		setProtocol(nextProtocol);
	}, []);

	useEffect(() => {
		let cancelled = false;
		async function load() {
			for (let attempt = 0; attempt < 8; attempt++) {
				try {
					await refreshNexus();
					return;
				} catch (error) {
					if (attempt === 7) {
						if (!cancelled) setNexusError(formatWailsError(error));
						return;
					}
					await new Promise((resolve) => window.setTimeout(resolve, 200));
					if (cancelled) return;
				}
			}
		}
		void load();
		return () => {
			cancelled = true;
		};
	}, [refreshNexus]);

	async function connectNexus() {
		if (keyDraft === "" || nexusBusy) return;
		setNexusBusy(true);
		setNexusError("");
		try {
			await SetNexusAPIKey(keyDraft);
			setKeyDraft("");
			await refreshNexus();
		} catch (error) {
			setNexusError(formatWailsError(error));
		} finally {
			setNexusBusy(false);
		}
	}

	async function disconnectNexus() {
		if (nexusBusy) return;
		setNexusBusy(true);
		setNexusError("");
		try {
			await ClearNexusAPIKey();
			await refreshNexus();
		} catch (error) {
			setNexusError(formatWailsError(error));
		} finally {
			setNexusBusy(false);
		}
	}

	async function applyHandler(enabled: boolean, takeOver: boolean) {
		if (!protocol || nexusBusy) return;
		const previous = protocol;
		setProtocol({
			...previous,
			enabled,
			ownership: enabled ? "self" : "none",
		});
		setNexusError("");
		try {
			const next = enabled
				? await RegisterNexusProtocol(takeOver)
				: await UnregisterNexusProtocol();
			setProtocol(next);
		} catch (error) {
			setProtocol(previous);
			setNexusError(formatWailsError(error));
		}
	}

	async function toggleHandler() {
		if (!protocol) return;
		if (protocol.enabled) {
			await applyHandler(false, false);
			return;
		}
		if (protocol.ownership === "other") {
			setTakeOverOwner(formatProtocolOwner(protocol));
			return;
		}
		await applyHandler(true, false);
	}

	const accountKind = nexusAccountKind(account);
	const handlerDisabledReason = protocolSwitchDisabledReason(protocol);
	const handlerEnabled = protocol?.enabled ?? false;

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className={`mutation-dialog ${styles["settings-dialog"]}`}
				aria-labelledby="settings-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div className="conflict-dialog-header">
					<div>
						<p className="eyebrow">Cratebug</p>
						<h2 id="settings-dialog-title">Settings</h2>
					</div>
					<button
						ref={closeButtonRef}
						type="button"
						className="icon-button conflict-dialog-close"
						onClick={onClose}
						aria-label="Close"
					>
						<X aria-hidden="true" />
					</button>
				</div>
				<div className={[styles["settings-body"], "scroll-y"].join(" ")}>
					<div className={styles["setting-section"]}>
						<h3>Appearance</h3>
						<div
							className={styles["theme-picker"]}
							role="radiogroup"
							aria-label="Theme"
						>
							{themes.map((option) => {
								const Icon = themeIcons[option];
								const selected = theme === option;
								return (
									<button
										key={option}
										type="button"
										className={[
											styles["theme-option"],
											selected ? styles.selected : "",
										]
											.filter(Boolean)
											.join(" ")}
										aria-pressed={selected}
										onClick={() => onSelectTheme(option)}
										title={themeLabels[option]}
									>
										<Icon aria-hidden="true" />
										{selected && (
											<Check
												className={styles["theme-option-check"]}
												aria-hidden="true"
											/>
										)}
										<span className="visually-hidden">
											{themeLabels[option]}
										</span>
									</button>
								);
							})}
						</div>
					</div>
					<div className={styles["setting-section"]}>
						<h3>Accent color</h3>
						<div className={styles["accent-picker"]}>
							<button
								type="button"
								className={[
									styles["accent-swatch"],
									styles.reset,
									accentColor === "" ? styles.selected : "",
								]
									.filter(Boolean)
									.join(" ")}
								aria-pressed={accentColor === ""}
								onClick={() => onSelectAccentColor("")}
								title="Default"
							>
								<RotateCcw aria-hidden="true" />
								<span className="visually-hidden">Default</span>
							</button>
							{accentPresets.map((preset) => {
								const selected =
									accentColor.toLowerCase() === preset.hex.toLowerCase();
								return (
									<button
										key={preset.hex}
										type="button"
										className={[
											styles["accent-swatch"],
											selected ? styles.selected : "",
										]
											.filter(Boolean)
											.join(" ")}
										aria-pressed={selected}
										style={{ background: preset.hex }}
										onClick={() => onSelectAccentColor(preset.hex)}
										title={preset.name}
									>
										{selected && <Check aria-hidden="true" />}
										<span className="visually-hidden">{preset.name}</span>
									</button>
								);
							})}
							<label className={styles["accent-hex-field"]}>
								<span className="visually-hidden">Custom hex color</span>
								<span
									className={styles["accent-hex-preview"]}
									style={{
										background: isValidHexColor(hexDraft)
											? hexDraft
											: "transparent",
									}}
									aria-hidden="true"
								/>
								<input
									type="text"
									value={hexDraft}
									placeholder="#rrggbb"
									spellCheck={false}
									maxLength={7}
									onChange={(event) => handleHexInput(event.target.value)}
								/>
							</label>
						</div>
					</div>
					<div className={styles["setting-section"]}>
						<h3>Mod library detection</h3>
						<div
							className={styles["provider-picker"]}
							role="radiogroup"
							aria-label="Store provider for library auto-detection"
						>
							{libraryProviders.map((option) => {
								const Logo = providerLogos[option];
								const selected = libraryProvider === option;
								return (
									<button
										key={option}
										type="button"
										className={[
											styles["provider-option"],
											selected ? styles.selected : "",
										]
											.filter(Boolean)
											.join(" ")}
										aria-pressed={selected}
										onClick={() => onSelectLibraryProvider(option)}
										title={`${libraryProviderLabels[option]} library auto-detection`}
									>
										<Logo className={styles["provider-option-logo"] ?? ""} />
										{libraryProviderLabels[option]}
										{selected && (
											<Check
												className={styles["theme-option-check"]}
												aria-hidden="true"
											/>
										)}
									</button>
								);
							})}
						</div>
						<p className={styles["setting-section-hint"]}>
							Which store's Marvel Rivals installation the library auto-detect
							searches.
						</p>
					</div>
					<div className={styles["setting-section"]}>
						<h3>Confirmations</h3>
						<div className={styles["setting-switch-row"]}>
							<div>
								<p className={styles["setting-switch-label"]}>
									Skip the countdown on destructive actions
								</p>
								<p className={styles["setting-section-hint"]}>
									Avoid waiting three seconds. You still have to confirm.
								</p>
							</div>
							<button
								type="button"
								role="switch"
								aria-checked={skipConfirmDelay}
								aria-label="Skip the countdown on destructive actions"
								className={styles["setting-switch"]}
								onClick={() => onToggleSkipConfirmDelay(!skipConfirmDelay)}
							>
								<span
									className={styles["setting-switch-knob"]}
									aria-hidden="true"
								/>
							</button>
						</div>
					</div>
					<div className={styles["setting-section"]}>
						<h3>Nexus Mods</h3>
						<p className={styles["setting-section-hint"]}>
							Paste a personal API key. Cratebug stores it only on this machine.
						</p>
						<button
							type="button"
							className={styles["nexus-link"]}
							onClick={() => BrowserOpenURL(nexusApiKeyPageURL)}
						>
							Get an API key on Nexus Mods
						</button>
						{accountKind === "premium" || accountKind === "free" ? (
							<div className={styles["nexus-account"]}>
								<p>
									Connected as <strong>{account?.name || "Nexus user"}</strong>
									{" · "}
									{accountKind === "premium" ? "Premium" : "Free"}
								</p>
								{account ? (
									<p className={styles["nexus-rate-limit"]}>
										{formatNexusRateLimit(account)}
									</p>
								) : null}
								<button
									type="button"
									className="quiet-button"
									disabled={nexusBusy}
									onClick={() => void disconnectNexus()}
								>
									Disconnect
								</button>
							</div>
						) : (
							<div className={styles["nexus-key-row"]}>
								{accountKind === "unverified" ? (
									<p className={styles["nexus-warning"]} role="status">
										The saved key could not be verified. Paste it again.
									</p>
								) : null}
								<label className={styles["nexus-key-field"]}>
									<span className="visually-hidden">Nexus Mods API key</span>
									<input
										type="password"
										value={keyDraft}
										autoComplete="off"
										spellCheck={false}
										placeholder="API key"
										disabled={nexusBusy}
										onChange={(event) => setKeyDraft(event.target.value)}
										onKeyDown={(event) => {
											if (event.key === "Enter") {
												event.preventDefault();
												void connectNexus();
											}
										}}
									/>
								</label>
								<button
									type="button"
									disabled={nexusBusy || keyDraft === ""}
									onClick={() => void connectNexus()}
								>
									{nexusBusy ? "Connecting..." : "Connect"}
								</button>
							</div>
						)}
						<div className={styles["setting-switch-row"]}>
							<div>
								<p className={styles["setting-switch-label"]}>
									Open Nexus downloads in Cratebug
								</p>
								<p className={styles["setting-section-hint"]}>
									Registers Cratebug as the nxm:// handler for Marvel Rivals.
								</p>
							</div>
							<button
								type="button"
								role="switch"
								aria-checked={handlerEnabled}
								aria-label="Open Nexus downloads in Cratebug"
								className={styles["setting-switch"]}
								disabled={nexusBusy || handlerDisabledReason !== null}
								onClick={() => void toggleHandler()}
							>
								<span
									className={styles["setting-switch-knob"]}
									aria-hidden="true"
								/>
							</button>
						</div>
						{handlerDisabledReason ? (
							<p className={styles["setting-section-hint"]}>
								{handlerDisabledReason}
							</p>
						) : null}
						{protocol?.userChoice ? (
							<p className={styles["nexus-warning"]} role="status">
								Windows is set to open nxm:// links in another app. Change that in
								Settings → Apps → Default apps.
							</p>
						) : null}
						{protocol?.machineWide && protocol.ownership === "none" ? (
							<p className={styles["setting-section-hint"]}>
								A machine-wide nxm:// handler is also installed. Registering here
								only changes it for this user.
							</p>
						) : null}
						{takeOverOwner ? (
							<div className={styles["nexus-takeover"]}>
								<p>
									{takeOverOwner} is currently the nxm:// handler. Take over?
									Cratebug can restore it if you turn this off later.
								</p>
								<div className={styles["nexus-takeover-actions"]}>
									<button
										type="button"
										className="quiet-button"
										onClick={() => setTakeOverOwner(null)}
									>
										Cancel
									</button>
									<button
										type="button"
										onClick={() => {
											setTakeOverOwner(null);
											void applyHandler(true, true);
										}}
									>
										Take over
									</button>
								</div>
							</div>
						) : null}
						{nexusError ? (
							<p className="mutation-dialog-error" role="alert">
								{nexusError}
							</p>
						) : null}
					</div>
					<div className={styles["setting-section"]}>
						<h3>Updates</h3>
						<div className={styles["update-settings-row"]}>
							<span className={styles["update-settings-version"]}>
								Version {appVersion}
							</span>
							<button
								type="button"
								className="quiet-button"
								onClick={onCheckForUpdate}
								disabled={isCheckingForUpdate}
							>
								<RefreshCw
									className={isCheckingForUpdate ? "spinning-loader" : undefined}
									aria-hidden="true"
								/>
								{isCheckingForUpdate ? "Checking..." : "Check for updates"}
							</button>
						</div>
					</div>
				</div>
			</section>
		</div>
	);
}
