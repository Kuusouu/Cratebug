import { Check, Moon, Monitor, RefreshCw, RotateCcw, Sun } from "lucide-react";
import styles from "./SettingsDialog.module.css";
import { useCallback, useEffect, useRef, useState } from "react";
import { accentPresets, isValidHexColor } from "./accentColor";
import {
	type LibraryProvider,
	libraryProviders,
	libraryProviderLabels,
	type Theme,
	themeLabels,
	themes,
} from "./libraryTypes";
import { providerLogos } from "./StoreLogos";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type SettingsDialogProps = {
	theme: Theme;
	accentColor: string;
	appVersion: string;
	isCheckingForUpdate: boolean;
	libraryProvider: LibraryProvider;
	onClose: () => void;
	onSelectTheme: (theme: Theme) => void;
	onSelectAccentColor: (hex: string) => void;
	onSelectLibraryProvider: (provider: LibraryProvider) => void;
	onCheckForUpdate: () => void;
};

const themeIcons = {
	system: Monitor,
	light: Sun,
	dark: Moon,
} satisfies Record<Theme, typeof Sun>;

// Every control here applies immediately, the same as ModTagDialog's
// checkboxes: there is nothing to buffer for a single-click preference like
// theme, so this has no Save/Cancel pair, only Close.
export function SettingsDialog({
	theme,
	accentColor,
	appVersion,
	isCheckingForUpdate,
	libraryProvider,
	onClose,
	onSelectTheme,
	onSelectAccentColor,
	onSelectLibraryProvider,
	onCheckForUpdate,
}: SettingsDialogProps) {
	const closeButtonRef = useRef<HTMLButtonElement>(null);
	const handleEscape = useCallback(() => onClose(), [onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const [hexDraft, setHexDraft] = useState(accentColor);

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

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="settings-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Cratebug</p>
					<h2 id="settings-dialog-title">Settings</h2>
				</div>
				<div className={styles["setting-section"]}>
					<h3>Appearance</h3>
					<div className={styles["theme-picker"]} role="radiogroup" aria-label="Theme">
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
									<span className="visually-hidden">{themeLabels[option]}</span>
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
							const selected = accentColor.toLowerCase() === preset.hex.toLowerCase();
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
						Which store's Marvel Rivals installation the library auto-detect searches.
					</p>
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
				<div className="mutation-dialog-actions">
					<button
						ref={closeButtonRef}
						type="button"
						className="quiet-button"
						onClick={onClose}
					>
						Close
					</button>
				</div>
			</section>
		</div>
	);
}
