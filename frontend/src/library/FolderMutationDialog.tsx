import { useEffect, useMemo, useRef, useState } from "react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { renameValidationError } from "./nameValidation";

export type FolderMutationMode = "create" | "rename" | "move";

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
	const selectRef = useRef<HTMLButtonElement>(null);
	const createMode = mode === "create";
	const renameMode = mode === "rename";
	const moveMode = mode === "move";
	const currentName = targetFolder.split("/").at(-1) ?? targetFolder;
	const currentParent = folderParent(targetFolder);
	const [name, setName] = useState(renameMode ? currentName : "");
	const [destinationParent, setDestinationParent] = useState(currentParent);
	const [validationError, setValidationError] = useState("");
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
		<Dialog
			open
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				if (!nextOpen && !isMutating) onClose();
			}}
		>
			<DialogContent>
				<DialogHeader>
					<p className="text-[0.7rem] font-extrabold tracking-[0.12em] text-primary uppercase">
						Folder action
					</p>
					<DialogTitle>{title}</DialogTitle>
					<DialogDescription className="wrap-anywhere">{subtitle}</DialogDescription>
				</DialogHeader>
				<form
					className="grid gap-5"
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{moveMode ? (
						<label
							className="grid gap-[0.4rem] text-[0.78rem] font-bold text-muted-foreground"
							htmlFor="move-folder-destination"
						>
							<span>Destination folder</span>
							<Select
								value={destinationParent}
								onValueChange={(value) => {
									// "" is the library root; Base UI treats it as an empty
									// value, so its label can't come from the item list.
									setDestinationParent(value ?? "");
									setValidationError("");
								}}
							>
								<SelectTrigger
									className="h-auto w-full rounded-md px-3 py-[0.65rem] data-placeholder:text-foreground data-[size=default]:h-auto"
									id="move-folder-destination"
									ref={selectRef}
								>
									<SelectValue>
										{(value: string) => (value === "" ? "Library root" : value)}
									</SelectValue>
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="">Library root</SelectItem>
									{moveDestinations.map((folder) => (
										<SelectItem key={folder} value={folder}>
											{folder}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</label>
					) : (
						<label
							className="grid gap-[0.4rem] text-[0.78rem] font-bold text-muted-foreground"
							htmlFor="folder-name"
						>
							<span>{createMode ? "Folder name" : "New name"}</span>
							<input
								className="min-w-0 px-3 py-[0.65rem]"
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
						<p className="text-[0.82rem] text-muted-foreground">
							If this list is a little cluttered, you can drag and drop the folder
							onto another folder in the sidebar! ^^
						</p>
					)}
					{validationError && (
						<p className="text-[0.8rem] font-semibold text-destructive" role="alert">
							{validationError}
						</p>
					)}
					<div className="dialog-actions">
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
			</DialogContent>
		</Dialog>
	);
}
