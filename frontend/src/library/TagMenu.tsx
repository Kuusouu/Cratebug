import { Check, ChevronDown, Pencil, Plus, Trash2, X } from "lucide-react";
import { useCallback, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { metadata } from "../../wailsjs/go/models";
import { usePositionedPopover } from "./usePositionedPopover";

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
// control instead of living behind a per-mod context menu. This is
// purpose-built rather than a reusable dropdown primitive: Cratebug has no
// second near-term consumer that would justify one yet.
export function TagMenu({
	catalog,
	filterIDs,
	onToggleFilter,
	onCreateTag,
	onRenameTag,
	onDeleteTag,
}: TagMenuProps) {
	const triggerRef = useRef<HTMLButtonElement>(null);
	const [anchor, setAnchor] = useState<{ x: number; y: number } | null>(null);
	const [newTagName, setNewTagName] = useState("");
	const [validationError, setValidationError] = useState("");
	const [editing, setEditing] = useState<EditingState>(null);
	const [isBusy, setIsBusy] = useState(false);
	const open = anchor !== null;

	const close = useCallback(() => {
		setAnchor(null);
		setEditing(null);
		setValidationError("");
	}, []);

	const { popoverRef, position } = usePositionedPopover<HTMLDivElement>(
		anchor?.x ?? 0,
		anchor?.y ?? 0,
		close,
	);

	function toggleOpen() {
		if (open) {
			close();
			return;
		}
		const rect = triggerRef.current?.getBoundingClientRect();
		if (!rect) return;
		setAnchor({ x: rect.left, y: rect.bottom + 6 });
	}

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

	const container = triggerRef.current?.closest<HTMLElement>(".app-shell");

	return (
		<>
			<button
				type="button"
				ref={triggerRef}
				className={`tag-menu-trigger${filterIDs.size > 0 ? " active" : ""}`}
				aria-haspopup="true"
				aria-expanded={open}
				onClick={toggleOpen}
			>
				<span>Tags{filterIDs.size > 0 ? ` (${filterIDs.size})` : ""}</span>
				<ChevronDown aria-hidden="true" />
			</button>
			{open &&
				container &&
				createPortal(
					<div
						ref={popoverRef}
						className="tag-menu-popover"
						role="menu"
						aria-label="Filter by tag"
						style={{
							left: position.left,
							top: position.top,
							visibility: position.ready ? "visible" : "hidden",
						}}
					>
						{catalog.length > 0 ? (
							<ul className="tag-menu-list">
								{catalog.map((tag) => {
									const selected = filterIDs.has(tag.id);
									const isEditing = editing?.tagID === tag.id;
									return (
										<li key={tag.id} className="tag-menu-row">
											{isEditing ? (
												<form
													className="tag-menu-edit-form"
													onSubmit={(event) => {
														event.preventDefault();
														void submitRename(tag);
													}}
												>
													<input
														// biome-ignore lint/a11y/noAutofocus: opening the rename form should hand focus to the field it exists for.
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
															if (event.key === "Escape") {
																event.stopPropagation();
																setEditing(null);
															}
														}}
													/>
													<button
														type="submit"
														className="tag-menu-icon-action"
														aria-label="Save tag name"
														disabled={isBusy}
													>
														<Check aria-hidden="true" />
													</button>
													<button
														type="button"
														className="tag-menu-icon-action"
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
														className="tag-menu-toggle"
														aria-pressed={selected}
														disabled={isBusy}
														onClick={() => onToggleFilter(tag.id)}
													>
														{selected && <Check aria-hidden="true" />}
														<span>{tag.name}</span>
													</button>
													<div className="tag-menu-row-actions">
														<button
															type="button"
															className="tag-menu-icon-action"
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
															className="tag-menu-icon-action destructive"
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
							<p className="tag-menu-empty">No tags yet.</p>
						)}
						<form
							className="tag-menu-create-form"
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
							<p className="tag-menu-error" role="alert">
								{validationError}
							</p>
						)}
					</div>,
					container,
				)}
		</>
	);
}
