import { Button } from "@/components/ui/button";
import { useEffect, useRef, useState } from "react";
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
import type { discovery } from "../../wailsjs/go/models";
import { maximumPriorityFor } from "./modPriority";
import { renameValidationError } from "./nameValidation";

export type ModMutationMode = "priority" | "rename" | "move";

type ModMutationDialogProps = {
	entry: discovery.Entry;
	batchCount?: number;
	folders: string[];
	isMutating: boolean;
	mode: ModMutationMode;
	onClose: () => void;
	onMove: (entry: discovery.Entry, destinationFolder: string) => Promise<boolean>;
	onRename: (entry: discovery.Entry, name: string) => Promise<boolean>;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
};

/** Rename, priority, or move for the viewed mod, or move for a checked set. */
export function ModMutationDialog({
	entry,
	batchCount = 1,
	folders,
	isMutating,
	mode,
	onClose,
	onMove,
	onRename,
	onSetPriority,
}: ModMutationDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const selectRef = useRef<HTMLButtonElement>(null);
	const [name, setName] = useState(entry.displayName);
	const [priority, setPriority] = useState(String(entry.priority.value));
	const [destinationFolder, setDestinationFolder] = useState(entry.relativeFolder);
	const [validationError, setValidationError] = useState("");
	const renameMode = mode === "rename";
	const priorityMode = mode === "priority";
	const moveMode = mode === "move";
	const title = renameMode
		? "Rename mod"
		: priorityMode
			? "Set priority"
			: batchCount > 1
				? `Move ${batchCount} mods`
				: "Move mod";

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
			if (batchCount === 1 && destinationFolder === entry.relativeFolder) {
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
						Mod action
					</p>
					<DialogTitle>{title}</DialogTitle>
					<DialogDescription className="wrap-anywhere">
						{batchCount > 1 ? `${batchCount} mods` : entry.displayName}
					</DialogDescription>
				</DialogHeader>
				<form
					className="grid gap-5"
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{renameMode ? (
						<label
							className="grid gap-[0.4rem] text-[0.78rem] font-bold text-muted-foreground"
							htmlFor="rename-mod-name"
						>
							<span>New name</span>
							<input
								className="min-w-0 px-3 py-[0.65rem]"
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
						<label
							className="grid gap-[0.4rem] text-[0.78rem] font-bold text-muted-foreground"
							htmlFor="move-mod-folder"
						>
							<span>Destination folder</span>
							<Select
								value={destinationFolder}
								onValueChange={(value) => {
									// "" is the library root; Base UI treats it as an empty
									// value, so its label can't come from the item list.
									setDestinationFolder(value ?? "");
									setValidationError("");
								}}
							>
								<SelectTrigger
									className="h-auto w-full rounded-md px-3 py-[0.65rem] data-placeholder:text-foreground data-[size=default]:h-auto"
									id="move-mod-folder"
									ref={selectRef}
								>
									<SelectValue>
										{(value: string) => (value === "" ? "Library root" : value)}
									</SelectValue>
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="">Library root</SelectItem>
									{folders.map((folder) => (
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
							htmlFor="set-mod-priority"
						>
							<span>Priority</span>
							<input
								className="min-w-0 px-3 py-[0.65rem]"
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
						<p className="text-[0.82rem] text-muted-foreground">
							If this list is a little cluttered, you can drag and drop the mod onto a
							folder in the sidebar! ^^
						</p>
					)}
					{validationError && (
						<p className="text-[0.8rem] font-semibold text-destructive" role="alert">
							{validationError}
						</p>
					)}
					<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
						<Button
							size="action"
							type="button"
							variant="secondary"
							disabled={isMutating}
							onClick={onClose}
						>
							Cancel
						</Button>
						<Button size="action" type="submit" disabled={isMutating}>
							{isMutating
								? "Saving..."
								: renameMode
									? "Rename"
									: priorityMode
										? "Set priority"
										: "Move"}
						</Button>
					</div>
				</form>
			</DialogContent>
		</Dialog>
	);
}
