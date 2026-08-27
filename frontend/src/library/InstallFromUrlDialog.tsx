import { useCallback, useEffect, useRef, useState } from "react";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type InstallFromUrlDialogProps = {
	onSubmit: (url: string) => void;
	onCancel: () => void;
};

// Client-side validation is a courtesy only: the backend (install.DownloadRemoteFile)
// re-validates HTTPS and the file name independently and is the actual enforcement point.
export function InstallFromUrlDialog({ onSubmit, onCancel }: InstallFromUrlDialogProps) {
	const [url, setUrl] = useState("");
	const [validationError, setValidationError] = useState("");
	const inputRef = useRef<HTMLInputElement>(null);
	const handleEscape = useCallback(() => onCancel(), [onCancel]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		inputRef.current?.focus();
	}, []);

	function submit() {
		const trimmed = url.trim();
		if (!trimmed) {
			setValidationError("Enter a download URL.");
			return;
		}

		let parsed: URL;
		try {
			parsed = new URL(trimmed);
		} catch {
			setValidationError("Enter a valid URL.");
			return;
		}
		if (parsed.protocol !== "https:") {
			setValidationError("The URL must start with https://.");
			return;
		}

		onSubmit(trimmed);
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="install-url-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Cratebug</p>
					<h2 id="install-url-dialog-title">Install from URL</h2>
				</div>
				<form
					onSubmit={(event) => {
						event.preventDefault();
						submit();
					}}
				>
					<label className="mutation-dialog-field" htmlFor="install-url-input">
						<span>Direct download link</span>
						<input
							ref={inputRef}
							id="install-url-input"
							type="text"
							value={url}
							placeholder="https://example.com/MyMod.zip"
							spellCheck={false}
							onChange={(event) => {
								setUrl(event.target.value);
								setValidationError("");
							}}
						/>
					</label>
					{validationError && (
						<p className="mutation-dialog-error" role="alert">
							{validationError}
						</p>
					)}
					<div className="mutation-dialog-actions">
						<button type="button" className="quiet-button" onClick={onCancel}>
							Cancel
						</button>
						<button type="submit">Download &amp; install</button>
					</div>
				</form>
			</section>
		</div>
	);
}
