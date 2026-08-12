import { memo } from "react";
import type { discovery } from "../../wailsjs/go/models";
import type { LibraryState, ViewMode } from "./libraryTypes";

type ModCatalogProps = {
	entries: discovery.Entry[];
	state: LibraryState;
	scanError: string;
	hasLibrary: boolean;
	mutatingEntryIDs: ReadonlySet<string>;
	onSetEnabled: (entry: discovery.Entry) => void;
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
	onSetEnabled: ModCatalogProps["onSetEnabled"];
	viewMode: ModCatalogProps["viewMode"];
};

// Presents orphaned sidecars differently from primary-backed entries.
function stateLabel(entry: discovery.Entry): string {
	if (entry.kind === "orphaned_sidecar") return "Orphaned sidecar";

	return entry.state === "disabled" ? "Disabled" : "Enabled";
}

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
	onSetEnabled,
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
					onSetEnabled={onSetEnabled}
					viewMode={viewMode}
				/>
			))}
		</div>
	);
}

// Avoids work for retained entries during local navigation changes.
const ModCard = memo(function ModCard({ entry, isMutating, onSetEnabled, viewMode }: ModCardProps) {
	const hasAmbiguousPrimary = entry.issues?.some((issue) => issue.code === "ambiguous-primary");
	const canChangeState =
		entry.kind === "mod" && entry.primaryPath !== undefined && !hasAmbiguousPrimary;
	const enabled = entry.state === "enabled";
	const actionLabel = enabled ? "Disable" : "Enable";
	const facts = (
		<div className="mod-facts">
			<span>{stateLabel(entry)}</span>
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
				<h3>{entry.displayName}</h3>
				<p>{entry.relativeFolder || "Library root"}</p>
			</div>
		</div>
	);
	const action = canChangeState ? (
		<button
			type="button"
			aria-busy={isMutating}
			className="mod-action"
			disabled={isMutating}
			onClick={() => onSetEnabled(entry)}
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
			<article aria-busy={isMutating} className="list-mod-row">
				<div className="list-mod-summary">
					{heading}
					<div className="list-mod-controls">
						{facts}
						{action}
					</div>
				</div>
				{issues}
			</article>
		);
	return (
		<article aria-busy={isMutating} className="mod-card">
			<div className="mod-card-summary">
				{heading}
				{facts}
			</div>
			{action}
			{issues}
		</article>
	);
});
