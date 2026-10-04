import { Wrench, X } from "lucide-react";
import { useCallback, useEffect, useRef } from "react";
import { placeholderTools } from "./toolsPlaceholder";
import styles from "./ToolsDialog.module.css";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type ToolsDialogProps = {
	onClose: () => void;
};

// Placeholder dialog: renders dummy tools with disabled actions. Nothing here
// calls the backend; real tool wiring belongs to future roadmap phases.
export function ToolsDialog({ onClose }: ToolsDialogProps) {
	const closeButtonRef = useRef<HTMLButtonElement>(null);
	const handleEscape = useCallback(() => onClose(), [onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className={`mutation-dialog ${styles["tools-dialog"]}`}
				aria-labelledby="tools-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div className="conflict-dialog-header">
					<div>
						<p className="eyebrow">Cratebug</p>
						<h2 id="tools-dialog-title">Tools</h2>
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
				<div className={[styles["tools-body"], "scroll-y"].join(" ")}>
					<p className={styles["tools-placeholder-note"]}>
						Placeholder — these tools are not wired up yet. The entries below are dummy
						data.
					</p>
					<ul className={styles["tools-list"]}>
						{placeholderTools.map((tool) => (
							<li key={tool.id} className={styles["tool-card"]}>
								<div className={styles["tool-card-text"]}>
									<p className={styles["tool-card-title"]}>{tool.title}</p>
									<p className={styles["tool-card-description"]}>
										{tool.description}
									</p>
									<p className={styles["tool-card-status"]} role="status">
										{tool.status}
									</p>
								</div>
								<button
									type="button"
									className="quiet-button"
									disabled
									title="Not available in this placeholder"
								>
									<Wrench aria-hidden="true" />
									{tool.actionLabel}
								</button>
							</li>
						))}
					</ul>
				</div>
			</section>
		</div>
	);
}
