import { useCallback, useEffect, useRef } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { nexusAdultContentBlockedMessage, nexusContentBlockingURL } from "./nexusPresentation";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type NexusAdultBlockedDialogProps = {
	onClose: () => void;
};

export function NexusAdultBlockedDialog({ onClose }: NexusAdultBlockedDialogProps) {
	const closeRef = useRef<HTMLButtonElement>(null);
	const handleEscape = useCallback(() => onClose(), [onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		closeRef.current?.focus();
	}, []);

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="nexus-adult-blocked-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Nexus Mods</p>
					<h2 id="nexus-adult-blocked-title">Adult content is hidden</h2>
				</div>
				<p className="mutation-dialog-subtitle" role="alert">
					{nexusAdultContentBlockedMessage}
				</p>
				<div className="mutation-dialog-actions">
					<button ref={closeRef} type="button" className="quiet-button" onClick={onClose}>
						Close
					</button>
					<button type="button" onClick={() => BrowserOpenURL(nexusContentBlockingURL)}>
						Open Content Blocking
					</button>
				</div>
			</section>
		</div>
	);
}
