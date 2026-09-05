import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { renameValidationError } from "./nameValidation";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

export type FolderMutationMode = "create" | "rename" | "move";

// Native <select> dropdown popups paint their own scrollbar, so the themed
// .scroll-y rules never reach them. size>1 keeps the picker in-page.
const FOLDER_PICKER_VISIBLE_ROWS = 10;

type FolderMutationDialogProps = {
	folders: string[];
	isMutating: boolean;
	mode: FolderMutationMode;
	targetFolder: string;
	onClose: () => void;
	onCreate: (parentFolder: string, name: string) => Promise<boolean>;
	onMove: (folder: string, destinationParent: string) => Promise<boolean>;
	onRename: (folder: string, name: string) => Promise<boolean>;
};

function folderParent(folder: string): string {
	const separatorIndex = folder.lastIndexOf("/");
	return separatorIndex === -1 ? "" : folder.slice(0, separatorIndex);
}

export function FolderMutationDialog({
	folders,
	isMutating,
	mode,
	targetFolder,
	onClose,
	onCreate,
	onMove,
	onRename,
}: FolderMutationDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const selectRef = useRef<HTMLSelectElement>(null);
	const createMode = mode === "create";
	const renameMode = mode === "rename";
	const moveMode = mode === "move";
	const currentName = targetFolder.split("/").at(-1) ?? targetFolder;
	const currentParent = folderParent(targetFolder);
	const [name, setName] = useState(renameMode ? currentName : "");
	const [destinationParent, setDestinationParent] = useState(currentParent);
	const [validationError, setValidationError] = useState("");
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const title = createMode ? "New folder" : renameMode ? "Rename folder" : "Move folder";
	const subtitle = createMode
		? targetFolder
			? `Inside ${targetFolder}`
			: "Inside the library root"
		: targetFolder;

	useEffect(() => {
		if (moveMode) {
			selectRef.current?.focus();
		} else {
			inputRef.current?.focus();
		}
	}, [moveMode]);

	// Excludes the folder itself and its descendants: a folder cannot move into itself or a child.
	const moveDestinations = useMemo(
		() =>
			folders.filter(
				(folder) => folder !== targetFolder && !folder.startsWith(`${targetFolder}/`),
			),
		[folders, targetFolder],
	);

	async function submit() {
		if (moveMode) {
			if (destinationParent === currentParent) {
				setValidationError("Choose a different folder.");
				return;
			}

			if (await onMove(targetFolder, destinationParent)) onClose();
			return;
		}

		const error = renameValidationError(name, "folder");
		if (error) {
			setValidationError(error);
			return;
		}

		if (createMode) {
			if (await onCreate(targetFolder, name)) onClose();
			return;
		}

		if (name === currentName) {
			setValidationError("Enter a different folder name.");
			return;
		}

		if (await onRename(targetFolder, name)) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="folder-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Folder action</p>
					<h2 id="folder-dialog-title">{title}</h2>
					<p className="mutation-dialog-subtitle">{subtitle}</p>
				</div>
				<form
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{moveMode ? (
						<label className="mutation-dialog-field" htmlFor="move-folder-destination">
							<span>Destination folder</span>
							<select
								id="move-folder-destination"
								ref={selectRef}
								className="scroll-y"
								size={Math.max(
									2,
									Math.min(
										moveDestinations.length + 1,
										FOLDER_PICKER_VISIBLE_ROWS,
									),
								)}
								value={destinationParent}
								onChange={(event) => {
									setDestinationParent(event.target.value);
									setValidationError("");
								}}
							>
								<option value="">Library root</option>
								{moveDestinations.map((folder) => (
									<option key={folder} value={folder}>
										{folder}
									</option>
								))}
							</select>
						</label>
					) : (
						<label className="mutation-dialog-field" htmlFor="folder-name">
							<span>{createMode ? "Folder name" : "New name"}</span>
							<input
								id="folder-name"
								ref={inputRef}
								value={name}
								onChange={(event) => {
									setName(event.target.value);
									setValidationError("");
								}}
							/>
						</label>
					)}
					{moveMode && (
						<p className="mutation-dialog-subtitle">
							If this list is a little cluttered, you can drag and drop the folder
							onto another folder in the sidebar! ^^
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
								: createMode
									? "Create"
									: renameMode
										? "Rename"
										: "Move"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}
