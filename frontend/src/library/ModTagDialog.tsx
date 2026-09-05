import { useCallback, useEffect, useRef, useState } from "react";
import styles from "./ModTagDialog.module.css";
import type { discovery, metadata } from "../../wailsjs/go/models";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type ModTagDialogProps = {
	entry: discovery.Entry;
	catalog: metadata.Tag[];
	assignedTagIDs: ReadonlySet<string>;
	onClose: () => void;
	onCreateAndAssign: (name: string) => Promise<boolean>;
	onToggle: (tag: metadata.Tag, assign: boolean) => Promise<boolean>;
};

// Applies each tag toggle and the create-and-assign action immediately
// rather than staging changes behind a Save button, since every checkbox is
// already its own independent, atomic backend call.
export function ModTagDialog({
	entry,
	catalog,
	assignedTagIDs,
	onClose,
	onCreateAndAssign,
	onToggle,
}: ModTagDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const [newTagName, setNewTagName] = useState("");
	const [validationError, setValidationError] = useState("");
	const [togglingTagID, setTogglingTagID] = useState<string | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const isBusy = isCreating || togglingTagID !== null;
	const handleEscape = useCallback(() => {
		if (!isBusy) onClose();
	}, [isBusy, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		inputRef.current?.focus();
	}, []);

	async function handleToggle(tag: metadata.Tag, assign: boolean) {
		setTogglingTagID(tag.id);
		try {
			await onToggle(tag, assign);
		} finally {
			setTogglingTagID(null);
		}
	}

	async function submitNewTag() {
		const name = newTagName.trim();
		if (name === "") {
			setValidationError("Enter a tag name.");
			return;
		}

		setIsCreating(true);
		try {
			if (await onCreateAndAssign(name)) {
				setNewTagName("");
				setValidationError("");
			}
		} finally {
			setIsCreating(false);
		}
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="tag-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="tag-dialog-title">Tags</h2>
					<p className="mutation-dialog-subtitle">{entry.displayName}</p>
				</div>
				{catalog.length > 0 ? (
					<ul className={styles["tag-checklist"]}>
						{catalog.map((tag) => {
							const assigned = assignedTagIDs.has(tag.id);
							return (
								<li key={tag.id}>
									<label>
										<input
											type="checkbox"
											checked={assigned}
											disabled={isBusy}
											onChange={() => void handleToggle(tag, !assigned)}
										/>
										<span>{tag.name}</span>
									</label>
								</li>
							);
						})}
					</ul>
				) : (
					<p className="mutation-dialog-subtitle">No tags yet. Create one below.</p>
				)}
				<form
					onSubmit={(event) => {
						event.preventDefault();
						void submitNewTag();
					}}
				>
					<label className="mutation-dialog-field" htmlFor="new-tag-name">
						<span>New tag</span>
						<input
							id="new-tag-name"
							ref={inputRef}
							value={newTagName}
							disabled={isBusy}
							onChange={(event) => {
								setNewTagName(event.target.value);
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
						<button
							type="button"
							className="quiet-button"
							disabled={isBusy}
							onClick={onClose}
						>
							Close
						</button>
						<button type="submit" disabled={isBusy}>
							{isCreating ? "Adding..." : "Add tag"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}
