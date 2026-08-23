import { Package, X } from "lucide-react";
import { type DragEvent, memo, type MouseEvent } from "react";
import type { discovery, metadata } from "../../wailsjs/go/models";
import { canChangeModState, canOrganizeMod, canTagMod, entryStateLabel } from "./entryPresentation";
import type { LibraryState, ViewMode } from "./libraryTypes";

// Caps visible chips so a heavily-tagged mod cannot push a card's other
// controls out of a reasonable height; the rest collapse into a "+N" chip
// whose title lists the overflow by name.
const maximumVisibleTagChips = 4;

type ModCatalogProps = {
	entries: discovery.Entry[];
	state: LibraryState;
	scanError: string;
	hasLibrary: boolean;
	mutatingEntryIDs: ReadonlySet<string>;
	isMutationLocked: boolean;
	tagsByEntryID: ReadonlyMap<string, metadata.Tag[]>;
	draggedEntryID: string | null;
	onSetEnabled: (entry: discovery.Entry) => void;
	onSelect: (entry: discovery.Entry) => void;
	onContextMenu: (entry: discovery.Entry, event: MouseEvent) => void;
	onRemoveTag: (entry: discovery.Entry, tagID: string) => void;
	onDragStartMod: (entry: discovery.Entry) => void;
	onDragEndMod: () => void;
	selectedEntryID: string | null;
	viewMode: ViewMode;
};

type CatalogMessage = {
	heading: string;
	message: string;
	isError?: boolean;
};

type ModCardProps = {
	entry: discovery.Entry;
	isMutating: boolean;
	isMutationLocked: boolean;
	tags: metadata.Tag[];
	isDragging: boolean;
	onSetEnabled: ModCatalogProps["onSetEnabled"];
	onSelect: ModCatalogProps["onSelect"];
	onContextMenu: ModCatalogProps["onContextMenu"];
	onRemoveTag: ModCatalogProps["onRemoveTag"];
	onDragStartMod: ModCatalogProps["onDragStartMod"];
	onDragEndMod: ModCatalogProps["onDragEndMod"];
	selected: boolean;
	viewMode: ModCatalogProps["viewMode"];
};

// Uses the scanner-issued identity so a primary suffix transition does not remount the card.
function entryKey(entry: discovery.Entry): string {
	return entry.id;
}

// Centralizes catalog states so they cannot diverge visually.
function scanStateMessage(state: LibraryState, scanError: string): CatalogMessage | null {
	switch (state) {
		case "initial":
			return {
				heading: "Choose a mod library",
				message: "Paste a folder path above to scan it without changing any files.",
			};
		case "loading":
			return {
				heading: "Scanning library",
				message: "Cratebug is discovering supported mod bundles.",
			};
		case "error":
			return {
				heading: "Could not scan this folder",
				message: scanError,
				isError: true,
			};
		case "empty":
			return {
				heading: "No supported mods found",
				message:
					"This folder scanned successfully, but it contains no supported primary files or sidecars.",
			};
		case "populated":
			return null;
	}
}

// Renders a concise catalog state without duplicating its visual structure.
function CatalogState({ heading, message, isError = false }: CatalogMessage) {
	return (
		<div className={`catalog-state${isError ? " error" : ""}`}>
			<h3>{heading}</h3>
			<p>{message}</p>
		</div>
	);
}

// Renders already-scanned, locally-filtered entries without filesystem access.
export function ModCatalog({
	entries,
	state,
	scanError,
	hasLibrary,
	mutatingEntryIDs,
	isMutationLocked,
	tagsByEntryID,
	draggedEntryID,
	onSetEnabled,
	onSelect,
	onContextMenu,
	onRemoveTag,
	onDragStartMod,
	onDragEndMod,
	selectedEntryID,
	viewMode,
}: ModCatalogProps) {
	const stateMessage = scanStateMessage(state, scanError);
	if (stateMessage) {
		return <CatalogState {...stateMessage} />;
	}

	if (hasLibrary && entries.length === 0) {
		return (
			<CatalogState
				heading="No matching mods"
				message="Try another folder or adjust the search query."
			/>
		);
	}

	return (
		<div className={`mod-grid view-${viewMode}`}>
			{entries.map((entry) => (
				<ModCard
					entry={entry}
					key={entryKey(entry)}
					isMutating={mutatingEntryIDs.has(entry.id)}
					isMutationLocked={isMutationLocked}
					tags={tagsByEntryID.get(entry.id) ?? []}
					isDragging={draggedEntryID === entry.id}
					onSetEnabled={onSetEnabled}
					onSelect={onSelect}
					onContextMenu={onContextMenu}
					onRemoveTag={onRemoveTag}
					onDragStartMod={onDragStartMod}
					onDragEndMod={onDragEndMod}
					selected={selectedEntryID === entry.id}
					viewMode={viewMode}
				/>
			))}
		</div>
	);
}

// Avoids work for retained entries during local navigation changes.
const ModCard = memo(function ModCard({
	entry,
	isMutating,
	isMutationLocked,
	tags,
	isDragging,
	onSetEnabled,
	onSelect,
	onContextMenu,
	onRemoveTag,
	onDragStartMod,
	onDragEndMod,
	selected,
	viewMode,
}: ModCardProps) {
	const canChangeState = canChangeModState(entry);
	const canDrag = canOrganizeMod(entry) && !isMutationLocked;
	// Shared by both the grid-card and list-row branches below, which are
	// otherwise identical for drag purposes.
	function handleDragStart(event: DragEvent<HTMLElement>) {
		event.dataTransfer.effectAllowed = "move";
		event.dataTransfer.setData("text/plain", entry.id);
		onDragStartMod(entry);
	}
	const enabled = entry.state === "enabled";
	const disabled = entry.state === "disabled";
	const facts = (
		<div className="mod-facts">
			<span>{entryStateLabel(entry)}</span>
			{entry.bundleFormat && (
				<span className={`bundle-format-badge ${entry.bundleFormat}`}>
					{entry.bundleFormat === "iostore" ? "IoStore" : "Classic"}
				</span>
			)}
			<span>Priority {entry.priority.value}</span>
		</div>
	);
	const heading = (
		<div className="mod-card-heading">
			<div className="mod-thumbnail" aria-hidden="true">
				<Package aria-hidden="true" />
			</div>
			<div className="mod-card-heading-info">
				<h3>{entry.displayName}</h3>
				<p>{entry.relativeFolder || "Library root"}</p>
			</div>
		</div>
	);
	const visibleTags = tags.slice(0, maximumVisibleTagChips);
	const overflowTags = tags.slice(maximumVisibleTagChips);
	// Rendered as a sibling of mod-card-select-area, never inside it: that
	// area's select button is absolutely positioned over its own children to
	// make the whole heading/facts region clickable, which would swallow
	// pointer events meant for a remove chip nested underneath it.
	const tagsRow =
		canTagMod(entry) && tags.length > 0 ? (
			<ul className="mod-card-tags" aria-label="Tags">
				{visibleTags.map((tag) => (
					<li key={tag.id}>
						<span>{tag.name}</span>
						<button
							type="button"
							aria-label={`Remove tag ${tag.name}`}
							onClick={(event) => {
								event.stopPropagation();
								onRemoveTag(entry, tag.id);
							}}
						>
							<X aria-hidden="true" />
						</button>
					</li>
				))}
				{overflowTags.length > 0 && (
					<li
						className="mod-card-tags-overflow"
						title={overflowTags.map((tag) => tag.name).join(", ")}
					>
						+{overflowTags.length}
					</li>
				)}
			</ul>
		) : null;
	// Renders heading (and facts, for grid cards) as plain siblings rather than
	// button children, since <button> only permits phrasing content and the
	// heading/facts blocks contain <h3>/<p>/<div>. The overlay button below
	// covers the same area and carries the keyboard/click target instead.
	function selectionArea(includeFacts: boolean) {
		return (
			<div className="mod-card-select-area">
				{heading}
				{includeFacts && facts}
				<button
					type="button"
					aria-pressed={selected}
					className="mod-card-select"
					onClick={(event) => {
						event.stopPropagation();
						onSelect(entry);
					}}
					onContextMenu={(event) => {
						event.preventDefault();
						event.stopPropagation();
						onContextMenu(entry, event);
					}}
				>
					<span className="visually-hidden">Select {entry.displayName}</span>
				</button>
			</div>
		);
	}
	// A switch, not a button: the previous status dot + "DISABLED" badge +
	// Enable/Disable button said the same thing three ways. Rename, priority,
	// move, and delete stay context-menu-only, unchanged (see
	// docs/decisions/0002-organize-action-pattern.md) — this only changes
	// enable/disable's existing front-facing control from a button to a switch.
	const toggleControl = canChangeState ? (
		<button
			type="button"
			role="switch"
			aria-checked={enabled}
			aria-busy={isMutating}
			aria-label={`${enabled ? "Disable" : "Enable"} ${entry.displayName}`}
			className="mod-toggle"
			disabled={isMutationLocked}
			onClick={(event) => {
				event.stopPropagation();
				onSetEnabled(entry);
			}}
		>
			<span className="mod-toggle-knob" aria-hidden="true" />
		</button>
	) : null;
	const issues = entry.issues?.length ? (
		<ul className="issues">
			{entry.issues.map((issue) => (
				<li key={issue.code}>{issue.message}</li>
			))}
		</ul>
	) : null;

	if (viewMode === "list")
		return (
			// biome-ignore lint/a11y/useKeyWithClickEvents: The dedicated card button remains the keyboard control; the row click only extends selection to its empty pointer area.
			<article
				aria-busy={isMutating}
				className={`list-mod-row${disabled ? " is-disabled" : ""}${selected ? " is-selected" : ""}${isDragging ? " dragging" : ""}`}
				draggable={canDrag}
				onClick={() => onSelect(entry)}
				onContextMenu={(event) => {
					event.preventDefault();
					onContextMenu(entry, event);
				}}
				onDragStart={handleDragStart}
				onDragEnd={onDragEndMod}
			>
				<div className="list-mod-summary">
					{selectionArea(false)}
					<div className="list-mod-controls">
						{facts}
						{toggleControl}
					</div>
				</div>
				{tagsRow}
				{issues}
			</article>
		);
	return (
		// biome-ignore lint/a11y/useKeyWithClickEvents: The dedicated card button remains the keyboard control; the card click only extends selection to its empty pointer area.
		<article
			aria-busy={isMutating}
			className={`mod-card${disabled ? " is-disabled" : ""}${selected ? " is-selected" : ""}${isDragging ? " dragging" : ""}`}
			draggable={canDrag}
			onClick={() => onSelect(entry)}
			onContextMenu={(event) => {
				event.preventDefault();
				onContextMenu(entry, event);
			}}
			onDragStart={handleDragStart}
			onDragEnd={onDragEndMod}
		>
			{selectionArea(true)}
			{tagsRow}
			{toggleControl}
			{issues}
		</article>
	);
});
