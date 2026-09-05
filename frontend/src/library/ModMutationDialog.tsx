import { useCallback, useEffect, useRef, useState } from "react";
import type { discovery } from "../../wailsjs/go/models";
import { maximumPriorityFor } from "./modPriority";
import { renameValidationError } from "./nameValidation";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

export type ModMutationMode = "priority" | "rename" | "move";

// Native <select> dropdown popups paint their own scrollbar, so the themed
// .scroll-y rules never reach them. size>1 keeps the picker in-page.
const FOLDER_PICKER_VISIBLE_ROWS = 10;

type ModMutationDialogProps = {
	entry: discovery.Entry;
	folders: string[];
	isMutating: boolean;
	mode: ModMutationMode;
	onClose: () => void;
	onMove: (entry: discovery.Entry, destinationFolder: string) => Promise<boolean>;
	onRename: (entry: discovery.Entry, name: string) => Promise<boolean>;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
};

export function ModMutationDialog({
	entry,
	folders,
	isMutating,
	mode,
	onClose,
	onMove,
	onRename,
	onSetPriority,
}: ModMutationDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const selectRef = useRef<HTMLSelectElement>(null);
	const [name, setName] = useState(entry.displayName);
	const [priority, setPriority] = useState(String(entry.priority.value));
	const [destinationFolder, setDestinationFolder] = useState(entry.relativeFolder);
	const [validationError, setValidationError] = useState("");
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const renameMode = mode === "rename";
	const priorityMode = mode === "priority";
	const moveMode = mode === "move";
	const title = renameMode ? "Rename mod" : priorityMode ? "Set priority" : "Move mod";

	useEffect(() => {
		if (moveMode) {
			selectRef.current?.focus();
		} else {
			inputRef.current?.focus();
		}
	}, [moveMode]);

	async function submit() {
		if (renameMode) {
			const error = renameValidationError(name);
			if (error) {
				setValidationError(error);
				return;
			}

			if (name === entry.displayName) {
				setValidationError("Enter a different mod name.");
				return;
			}

			if (await onRename(entry, name)) onClose();
			return;
		}

		if (moveMode) {
			if (destinationFolder === entry.relativeFolder) {
				setValidationError("Choose a different folder.");
				return;
			}

			if (await onMove(entry, destinationFolder)) onClose();
			return;
		}

		if (priority.trim() === "") {
			setValidationError("Enter a priority.");
			return;
		}

		const value = Number(priority);
		const maximumPriority = maximumPriorityFor(entry);
		if (!Number.isSafeInteger(value) || value < 0 || value > maximumPriority) {
			setValidationError(`Priority must be a whole number from 0 to ${maximumPriority}.`);
			return;
		}

		if (value === entry.priority.value) {
			setValidationError("Choose a different priority.");
			return;
		}

		if (await onSetPriority(entry, value)) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="mutation-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="mutation-dialog-title">{title}</h2>
					<p className="mutation-dialog-subtitle">{entry.displayName}</p>
				</div>
				<form
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{renameMode ? (
						<label className="mutation-dialog-field" htmlFor="rename-mod-name">
							<span>New name</span>
							<input
								id="rename-mod-name"
								ref={inputRef}
								value={name}
								onChange={(event) => {
									setName(event.target.value);
									setValidationError("");
								}}
							/>
						</label>
					) : moveMode ? (
						<label className="mutation-dialog-field" htmlFor="move-mod-folder">
							<span>Destination folder</span>
							<select
								id="move-mod-folder"
								ref={selectRef}
								className="scroll-y"
								size={Math.max(
									2,
									Math.min(folders.length + 1, FOLDER_PICKER_VISIBLE_ROWS),
								)}
								value={destinationFolder}
								onChange={(event) => {
									setDestinationFolder(event.target.value);
									setValidationError("");
								}}
							>
								<option value="">Library root</option>
								{folders.map((folder) => (
									<option key={folder} value={folder}>
										{folder}
									</option>
								))}
							</select>
						</label>
					) : (
						<label className="mutation-dialog-field" htmlFor="set-mod-priority">
							<span>Priority</span>
							<input
								id="set-mod-priority"
								inputMode="numeric"
								min="0"
								ref={inputRef}
								step="1"
								type="number"
								value={priority}
								onChange={(event) => {
									setPriority(event.target.value);
									setValidationError("");
								}}
							/>
						</label>
					)}
					{moveMode && (
						<p className="mutation-dialog-subtitle">
							If this list is a little cluttered, you can drag and drop the mod onto a
							folder in the sidebar! ^^
						</p>
					)}
					{validationError && (
						<p className="mutation-dialog-error" role="alert">
							{validationError}
						</p>
					)}
					<div className="mutation-dialog-actions">
						<button
							type="button"
							className="quiet-button"
							disabled={isMutating}
							onClick={onClose}
						>
							Cancel
						</button>
						<button type="submit" disabled={isMutating}>
							{isMutating
								? "Saving..."
								: renameMode
									? "Rename"
									: priorityMode
										? "Set priority"
										: "Move"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}
