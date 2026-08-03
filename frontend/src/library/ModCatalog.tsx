import { memo } from "react";
import type { discovery } from "../../wailsjs/go/models";
import type { LibraryState, ViewMode } from "./libraryTypes";

type ModCatalogProps = {
	entries: discovery.Entry[];
	state: LibraryState;
	scanError: string;
	hasLibrary: boolean;
	viewMode: ViewMode;
};

type CatalogMessage = {
	heading: string;
	message: string;
	isError?: boolean;
};

// Presents orphaned sidecars differently from primary-backed entries.
function stateLabel(entry: discovery.Entry): string {
	if (entry.kind === "orphaned_sidecar") return "Orphaned sidecar";

	return entry.state === "disabled" ? "Disabled" : "Enabled";
}

// Produces a stable key when orphaned entries lack a primary path.
function entryKey(entry: discovery.Entry): string {
	return entry.primaryPath ?? entry.sidecars.ucas ?? entry.sidecars.utoc ?? entry.relativeFolder;
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
export function ModCatalog({ entries, state, scanError, hasLibrary, viewMode }: ModCatalogProps) {
	const stateMessage = scanStateMessage(state, scanError);
	if (stateMessage) return <CatalogState {...stateMessage} />;

	if (hasLibrary && entries.length === 0)
		return (
			<CatalogState
				heading="No matching mods"
				message="Try another folder or adjust the search query."
			/>
		);

	return (
		<div className={`mod-grid view-${viewMode}`}>
			{entries.map((entry) => (
				<ModCard entry={entry} key={entryKey(entry)} viewMode={viewMode} />
			))}
		</div>
	);
}

// Avoids work for retained entries during local navigation changes.
const ModCard = memo(function ModCard({
	entry,
	viewMode,
}: {
	entry: discovery.Entry;
	viewMode: ModCatalogProps["viewMode"];
}) {
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
	const issues = entry.issues?.length ? (
		<ul className="issues">
			{entry.issues.map((issue) => (
				<li key={issue.code}>{issue.message}</li>
			))}
		</ul>
	) : null;

	if (viewMode === "list")
		return (
			<article className="list-mod-row">
				<div className="list-mod-summary">
					{heading}
					{facts}
				</div>
				{issues}
			</article>
		);
	return (
		<article className="mod-card">
			<div className="mod-card-summary">
				{heading}
				{facts}
			</div>
			{issues}
		</article>
	);
});
