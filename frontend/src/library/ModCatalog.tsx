import { memo, type MouseEvent } from "react";
import type { discovery } from "../../wailsjs/go/models";
import { entryStateLabel } from "./entryPresentation";
import type { LibraryState, ViewMode } from "./libraryTypes";

type ModCatalogProps = {
	entries: discovery.Entry[];
	state: LibraryState;
	scanError: string;
	hasLibrary: boolean;
	mutatingEntryIDs: ReadonlySet<string>;
	isMutationLocked: boolean;
	onSetEnabled: (entry: discovery.Entry) => void;
	onSelect: (entry: discovery.Entry) => void;
	onContextMenu: (entry: discovery.Entry, event: MouseEvent) => void;
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
	onSetEnabled: ModCatalogProps["onSetEnabled"];
	onSelect: ModCatalogProps["onSelect"];
	onContextMenu: ModCatalogProps["onContextMenu"];
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
	onSetEnabled,
	onSelect,
	onContextMenu,
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
					onSetEnabled={onSetEnabled}
					onSelect={onSelect}
					onContextMenu={onContextMenu}
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
	onSetEnabled,
	onSelect,
	onContextMenu,
	selected,
	viewMode,
}: ModCardProps) {
	const hasAmbiguousPrimary = entry.issues?.some((issue) => issue.code === "ambiguous-primary");
	const canChangeState =
		entry.kind === "mod" && entry.primaryPath !== undefined && !hasAmbiguousPrimary;
	const enabled = entry.state === "enabled";
	const disabled = entry.state === "disabled";
	const actionLabel = enabled ? "Disable" : "Enable";
	const facts = (
		<div className="mod-facts">
			{!disabled && <span>{entryStateLabel(entry)}</span>}
			{entry.bundleFormat && (
				<span>{entry.bundleFormat === "iostore" ? "IoStore" : "Classic"}</span>
			)}
			<span>Priority {entry.priority.value}</span>
		</div>
	);
	const heading = (
		<div className="mod-card-heading">
			<span className={`status-dot ${entry.state ?? "unknown"}`} aria-hidden="true" />
			<div>
				<div className="mod-title-row">
					<h3>{entry.displayName}</h3>
					{disabled && <span className="disabled-badge">Disabled</span>}
				</div>
				<p>{entry.relativeFolder || "Library root"}</p>
			</div>
		</div>
	);
	function selection(includeFacts: boolean) {
		return (
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
				{heading}
				{includeFacts && facts}
			</button>
		);
	}
	const action = canChangeState ? (
		<button
			type="button"
			aria-busy={isMutating}
			className="mod-action"
			disabled={isMutationLocked}
			onClick={(event) => {
				event.stopPropagation();
				onSetEnabled(entry);
			}}
		>
			{isMutating ? (enabled ? "Disabling..." : "Enabling...") : actionLabel}
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
				className={`list-mod-row${disabled ? " is-disabled" : ""}${selected ? " is-selected" : ""}`}
				onClick={() => onSelect(entry)}
				onContextMenu={(event) => {
					event.preventDefault();
					onContextMenu(entry, event);
				}}
			>
				<div className="list-mod-summary">
					{selection(false)}
					<div className="list-mod-controls">
						{facts}
						{action}
					</div>
				</div>
				{issues}
			</article>
		);
	return (
		// biome-ignore lint/a11y/useKeyWithClickEvents: The dedicated card button remains the keyboard control; the card click only extends selection to its empty pointer area.
		<article
			aria-busy={isMutating}
			className={`mod-card${action ? " has-action" : ""}${disabled ? " is-disabled" : ""}${selected ? " is-selected" : ""}`}
			onClick={() => onSelect(entry)}
			onContextMenu={(event) => {
				event.preventDefault();
				onContextMenu(entry, event);
			}}
		>
			{selection(true)}
			{action}
			{issues}
		</article>
	);
});
