import { useEffect, useRef, useState } from "react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import type { discovery, metadata } from "../../wailsjs/go/models";

type ModTagDialogProps = {
	entry: discovery.Entry;
	batchCount?: number;
	catalog: metadata.Tag[];
	assignedTagIDs: ReadonlySet<string>;
	onClose: () => void;
	onCreateAndAssign: (name: string) => Promise<boolean>;
	onToggle: (tag: metadata.Tag, assign: boolean) => Promise<boolean>;
};

/**
 * Applies each tag toggle and the create-and-assign action immediately
 * rather than staging changes behind a Save button, since every checkbox is
 * already its own independent, atomic backend call. batchCount > 1 applies
 * the same toggle to every checked mod.
 */
export function ModTagDialog({
	entry,
	batchCount = 1,
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
		<Dialog
			open
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				if (!nextOpen && !isBusy) onClose();
			}}
		>
			<DialogContent>
				<DialogHeader>
					<p className="text-[0.7rem] font-extrabold tracking-[0.12em] text-primary uppercase">
						Mod action
					</p>
					<DialogTitle>Tags</DialogTitle>
					<DialogDescription className="wrap-anywhere">
						{batchCount > 1 ? `${batchCount} mods` : entry.displayName}
					</DialogDescription>
				</DialogHeader>
				{catalog.length > 0 ? (
					<ul className="grid max-h-56 gap-[0.35rem] overflow-y-auto">
						{catalog.map((tag) => {
							const assigned = assignedTagIDs.has(tag.id);
							return (
								<li key={tag.id}>
									<label className="flex items-center gap-[0.6rem] rounded-md border border-border px-[0.6rem] py-2 text-[0.85rem] font-semibold">
										<input
											type="checkbox"
											className="w-auto"
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
					<p className="text-[0.82rem] text-muted-foreground">
						No tags yet. Create one below.
					</p>
				)}
				<form
					className="grid gap-5"
					onSubmit={(event) => {
						event.preventDefault();
						void submitNewTag();
					}}
				>
					<label
						className="grid gap-[0.4rem] text-[0.78rem] font-bold text-muted-foreground"
						htmlFor="new-tag-name"
					>
						<span>New tag</span>
						<input
							className="min-w-0 px-3 py-[0.65rem]"
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
						<p className="text-[0.8rem] font-semibold text-destructive" role="alert">
							{validationError}
						</p>
					)}
					<div className="dialog-actions">
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
			</DialogContent>
		</Dialog>
	);
}
