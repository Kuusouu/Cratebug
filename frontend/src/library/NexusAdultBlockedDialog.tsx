import { useEffect, useRef } from "react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { nexusAdultContentBlockedMessage, nexusContentBlockingURL } from "./nexusPresentation";

type NexusAdultBlockedDialogProps = {
	onClose: () => void;
};

export function NexusAdultBlockedDialog({ onClose }: NexusAdultBlockedDialogProps) {
	const closeRef = useRef<HTMLButtonElement>(null);

	useEffect(() => {
		closeRef.current?.focus();
	}, []);

	return (
		<Dialog
			open
			onOpenChange={(nextOpen) => {
				if (!nextOpen) onClose();
			}}
			disablePointerDismissal
		>
			<DialogContent>
				<DialogHeader>
					<p className="eyebrow">Nexus Mods</p>
					<DialogTitle>Adult content is hidden</DialogTitle>
				</DialogHeader>
				<DialogDescription className="wrap-anywhere" role="alert">
					{nexusAdultContentBlockedMessage}
				</DialogDescription>
				<div className="dialog-actions">
					<button ref={closeRef} type="button" className="quiet-button" onClick={onClose}>
						Close
					</button>
					<button type="button" onClick={() => BrowserOpenURL(nexusContentBlockingURL)}>
						Open Content Blocking
					</button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
