import { Check, ChevronDown, Pencil, Plus, Trash2, X } from "lucide-react";
import styles from "./TagMenu.module.css";
import { useCallback, useState } from "react";
import type { metadata } from "../../wailsjs/go/models";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

type TagMenuProps = {
	catalog: metadata.Tag[];
	filterIDs: ReadonlySet<string>;
	onToggleFilter: (tagID: string) => void;
	onCreateTag: (name: string) => Promise<boolean>;
	onRenameTag: (tag: metadata.Tag, name: string) => Promise<boolean>;
	onDeleteTag: (tag: metadata.Tag) => Promise<boolean>;
};

type EditingState = { tagID: string; name: string } | null;

// Tag filtering has no single mod target, so per
// docs/decisions/0002-organize-action-pattern.md it earns its own toolbar
// control instead of living behind a per-mod context menu. The panel mixes
// toggle rows, inline rename, and a create form, so it is a popover rather
// than a plain menu of actions. Its rows keep their module classes: the
// unlayered button rules in App.css outrank Tailwind utilities, so styling
// them with utilities here would render the global accent treatment instead.
export function TagMenu({
	catalog,
	filterIDs,
	onToggleFilter,
	onCreateTag,
	onRenameTag,
	onDeleteTag,
}: TagMenuProps) {
	const [newTagName, setNewTagName] = useState("");
	const [validationError, setValidationError] = useState("");
	const [editing, setEditing] = useState<EditingState>(null);
	const [isBusy, setIsBusy] = useState(false);

	const close = useCallback(() => {
		setEditing(null);
		setValidationError("");
	}, []);

	async function submitNewTag() {
		const name = newTagName.trim();
		if (name === "") {
			setValidationError("Enter a tag name.");
			return;
		}

		setIsBusy(true);
		try {
			if (await onCreateTag(name)) {
				setNewTagName("");
				setValidationError("");
			}
		} finally {
			setIsBusy(false);
		}
	}

	async function submitRename(tag: metadata.Tag) {
		const name = editing?.name.trim() ?? "";
		if (name === "" || name === tag.name) {
			setEditing(null);
			return;
		}

		setIsBusy(true);
		try {
			if (await onRenameTag(tag, name)) {
				setEditing(null);
			}
		} finally {
			setIsBusy(false);
		}
	}

	async function handleDelete(tag: metadata.Tag) {
		setIsBusy(true);
		try {
			await onDeleteTag(tag);
		} finally {
			setIsBusy(false);
		}
	}

	return (
		<Popover
			onOpenChange={(open) => {
				if (!open) close();
			}}
		>
			<PopoverTrigger
				render={
					<button
						type="button"
						className={[
							styles["tag-menu-trigger"],
							filterIDs.size > 0 ? styles.active : "",
						]
							.filter(Boolean)
							.join(" ")}
					/>
				}
			>
				<span>Tags{filterIDs.size > 0 ? ` (${filterIDs.size})` : ""}</span>
				<ChevronDown aria-hidden="true" />
			</PopoverTrigger>
			<PopoverContent align="start" sideOffset={6} className="w-74 gap-1.5">
				{catalog.length > 0 ? (
					<ul className={styles["tag-menu-list"]}>
						{catalog.map((tag) => {
							const selected = filterIDs.has(tag.id);
							const isEditing = editing?.tagID === tag.id;
							return (
								<li key={tag.id} className={styles["tag-menu-row"]}>
									{isEditing ? (
										<form
											className={styles["tag-menu-edit-form"]}
											onSubmit={(event) => {
												event.preventDefault();
												void submitRename(tag);
											}}
										>
											<input
												// Opening the rename form should hand focus to the field it exists for.
												autoFocus
												value={editing.name}
												disabled={isBusy}
												onChange={(event) =>
													setEditing({
														tagID: tag.id,
														name: event.target.value,
													})
												}
												onKeyDown={(event) => {
													// Escape cancels the rename only, not the whole panel.
													if (event.key === "Escape") {
														event.stopPropagation();
														setEditing(null);
													}
												}}
											/>
											<button
												type="submit"
												className={styles["tag-menu-icon-action"]}
												aria-label="Save tag name"
												disabled={isBusy}
											>
												<Check aria-hidden="true" />
											</button>
											<button
												type="button"
												className={styles["tag-menu-icon-action"]}
												aria-label="Cancel rename"
												disabled={isBusy}
												onClick={() => setEditing(null)}
											>
												<X aria-hidden="true" />
											</button>
										</form>
									) : (
										<>
											<button
												type="button"
												className={styles["tag-menu-toggle"]}
												aria-pressed={selected}
												disabled={isBusy}
												onClick={() => onToggleFilter(tag.id)}
											>
												{selected && <Check aria-hidden="true" />}
												<span>{tag.name}</span>
											</button>
											<div className={styles["tag-menu-row-actions"]}>
												<button
													type="button"
													className={styles["tag-menu-icon-action"]}
													aria-label={`Rename ${tag.name}`}
													disabled={isBusy}
													onClick={() =>
														setEditing({
															tagID: tag.id,
															name: tag.name,
														})
													}
												>
													<Pencil aria-hidden="true" />
												</button>
												<button
													type="button"
													className={[
														styles["tag-menu-icon-action"],
														styles.destructive,
													].join(" ")}
													aria-label={`Delete ${tag.name}`}
													disabled={isBusy}
													onClick={() => void handleDelete(tag)}
												>
													<Trash2 aria-hidden="true" />
												</button>
											</div>
										</>
									)}
								</li>
							);
						})}
					</ul>
				) : (
					<p className={styles["tag-menu-empty"]}>No tags yet.</p>
				)}
				<form
					className={styles["tag-menu-create-form"]}
					onSubmit={(event) => {
						event.preventDefault();
						void submitNewTag();
					}}
				>
					<input
						value={newTagName}
						disabled={isBusy}
						placeholder="New tag"
						onChange={(event) => {
							setNewTagName(event.target.value);
							setValidationError("");
						}}
					/>
					<button
						type="submit"
						className="icon-button"
						disabled={isBusy}
						aria-label="Add tag"
					>
						<Plus aria-hidden="true" />
					</button>
				</form>
				{validationError && (
					<p className={styles["tag-menu-error"]} role="alert">
						{validationError}
					</p>
				)}
			</PopoverContent>
		</Popover>
	);
}
