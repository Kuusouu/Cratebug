import { CircleAlert, CircleCheckBig, Grid2X2, List, PanelsTopLeft, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ScanLibrary, SetModEnabled } from "../../wailsjs/go/main/App";
import { discovery } from "../../wailsjs/go/models";
import { entryStateLabel } from "./entryPresentation";
import { FolderNavigation } from "./FolderNavigation";
import { type LibraryState, type ViewMode, viewModeLabels, viewModes } from "./libraryTypes";
import { ModCatalog } from "./ModCatalog";

type Theme = "system" | "light" | "dark";

type LibraryIndex = {
	folders: string[];
	folderEntries: ReadonlyMap<string, discovery.Entry[]>;
	folderEntryCounts: ReadonlyMap<string, number>;
};

type ViewModeButtonProps = {
	active: boolean;
	mode: ViewMode;
	onSelect: (mode: ViewMode) => void;
};

type SelectedModPanelProps = {
	entry: discovery.Entry | null;
	isMutating: boolean;
	isMutationLocked: boolean;
	onClear: () => void;
	onSetEnabled: (entry: discovery.Entry) => void;
};

type MutationFeedback = {
	id: number;
	kind: "error" | "success";
	message: string;
};

type MutationToastProps = {
	feedback: MutationFeedback;
	onDismiss: () => void;
};

const viewModeIcons = {
	compact: Grid2X2,
	large: PanelsTopLeft,
	list: List,
} satisfies Record<ViewMode, typeof Grid2X2>;

const successToastDurationMilliseconds = 5000;
// Errors stay visible longer so people can read and dismiss actionable recovery guidance.
const errorToastDurationMilliseconds = 8000;

// Converts unknown Wails failures into displayable scan errors.
function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}

// Build subtree lookups once per scan so folder navigation does not repeatedly scan the library.
function indexLibrary(entries: discovery.Entry[]): LibraryIndex {
	const folderEntries = new Map<string, discovery.Entry[]>();

	for (const entry of entries) {
		const segments = entry.relativeFolder.split("/").filter(Boolean);
		for (let index = 1; index <= segments.length; index += 1) {
			const folder = segments.slice(0, index).join("/");
			const entriesInFolder = folderEntries.get(folder);
			if (entriesInFolder) {
				entriesInFolder.push(entry);
			} else {
				folderEntries.set(folder, [entry]);
			}
		}
	}

	return {
		folders: [...folderEntries.keys()].sort((left, right) => left.localeCompare(right)),
		folderEntries,
		folderEntryCounts: new Map(
			[...folderEntries].map(([folder, entriesInFolder]) => [folder, entriesInFolder.length]),
		),
	};
}

// Keeps live announcements concise when the catalog changes.
function libraryStatusMessage(
	state: LibraryState,
	scanError: string,
	entryCount: number,
	selectedFolder: string,
	search: string,
	viewMode: ViewMode,
): string {
	switch (state) {
		case "initial":
			return "Choose a mod library.";
		case "loading":
			return "Scanning library.";
		case "error":
			return `Scan failed: ${scanError}`;
		case "empty":
			return "No supported mods found.";
		case "populated": {
			const scope = selectedFolder === "all" ? "library" : selectedFolder;
			const matchesSearch = search.trim() !== "" ? " matching" : "";
			return `${viewModeLabels[viewMode]} view. ${entryCount}${matchesSearch} mods shown in ${scope}.`;
		}
	}
}

// Owns local browsing state while scanning remains behind the Go binding.
export function LibraryScreen() {
	const [modRoot, setModRoot] = useState("");
	const [library, setLibrary] = useState<discovery.Library | null>(null);
	const [libraryState, setLibraryState] = useState<LibraryState>("initial");
	const [scanError, setScanError] = useState("");
	const [mutationFeedback, setMutationFeedback] = useState<MutationFeedback | null>(null);
	const [mutatingEntryIDs, setMutatingEntryIDs] = useState<ReadonlySet<string>>(new Set());
	const [search, setSearch] = useState("");
	const [selectedFolder, setSelectedFolder] = useState("all");
	const [selectedEntryID, setSelectedEntryID] = useState<string | null>(null);
	const [theme, setTheme] = useState<Theme>("system");
	const [viewMode, setViewMode] = useState<ViewMode>("compact");
	const activeLibraryRootRef = useRef<string | null>(null);
	const mutatingEntryIDsRef = useRef(new Set<string>());
	const nextMutationFeedbackIDRef = useRef(0);
	const libraryRoot = library?.root;

	const libraryIndex = useMemo(() => indexLibrary(library?.entries ?? []), [library]);

	const displayedEntries = useMemo(() => {
		const normalizedSearch = search.trim().toLocaleLowerCase();
		const scopedEntries =
			selectedFolder === "all"
				? (library?.entries ?? [])
				: (libraryIndex.folderEntries.get(selectedFolder) ?? []);
		if (normalizedSearch === "") {
			return scopedEntries;
		}

		return scopedEntries.filter((entry) => {
			const matchesSearch =
				entry.displayName.toLocaleLowerCase().includes(normalizedSearch) ||
				entry.primaryPath?.toLocaleLowerCase().includes(normalizedSearch) ||
				entry.relativeFolder.toLocaleLowerCase().includes(normalizedSearch);
			return matchesSearch;
		});
	}, [library, libraryIndex, search, selectedFolder]);
	const statusMessage = libraryStatusMessage(
		libraryState,
		scanError,
		displayedEntries.length,
		selectedFolder,
		search,
		viewMode,
	);
	const selectedEntry = library?.entries.find((entry) => entry.id === selectedEntryID) ?? null;
	const isMutationLocked = mutatingEntryIDs.size > 0;
	const dismissMutationFeedback = useCallback(() => setMutationFeedback(null), []);
	const showMutationFeedback = useCallback((kind: MutationFeedback["kind"], message: string) => {
		nextMutationFeedbackIDRef.current += 1;
		setMutationFeedback({ id: nextMutationFeedbackIDRef.current, kind, message });
	}, []);

	const setModEnabled = useCallback(
		async (entry: discovery.Entry) => {
			if (!libraryRoot) return;

			if (!entry.primaryPath || mutatingEntryIDsRef.current.size > 0) return;

			const requestRoot = libraryRoot;
			const enabled = entry.state !== "enabled";
			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				const result = await SetModEnabled(requestRoot, entry.id, enabled);
				if (activeLibraryRootRef.current !== requestRoot) return;

				setLibrary((currentLibrary) => {
					if (!currentLibrary || currentLibrary.root !== requestRoot) {
						return currentLibrary;
					}

					const entries = currentLibrary.entries.map((currentEntry) => {
						if (currentEntry.id !== result.id) return currentEntry;

						return new discovery.Entry({
							...currentEntry,
							primaryPath: result.primaryPath,
							state: result.state,
						});
					});

					return new discovery.Library({ ...currentLibrary, entries });
				});
				showMutationFeedback(
					"success",
					`${enabled ? "Enabled" : "Disabled"} ${entry.displayName}.`,
				);
			} catch (error) {
				if (activeLibraryRootRef.current !== requestRoot) return;

				showMutationFeedback(
					"error",
					`Could not ${enabled ? "enable" : "disable"} ${entry.displayName}: ${errorMessage(error)}`,
				);
			} finally {
				mutatingEntryIDsRef.current.delete(entry.id);
				setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));
			}
		},
		[libraryRoot, showMutationFeedback],
	);

	// Replaces the catalog only after a scan finishes successfully.
	async function scan() {
		if (isMutationLocked) return;

		const root = modRoot.trim();
		if (!root) {
			activeLibraryRootRef.current = null;
			setLibrary(null);
			setScanError("");
			setMutationFeedback(null);
			setSearch("");
			setSelectedFolder("all");
			setSelectedEntryID(null);
			setLibraryState("initial");
			return;
		}

		activeLibraryRootRef.current = root;
		setLibraryState("loading");
		setScanError("");
		setMutationFeedback(null);
		try {
			const result = await ScanLibrary(root);
			if (activeLibraryRootRef.current !== root) return;

			setLibrary(result);
			// A fresh catalog may not contain the previous selection.
			setSelectedFolder("all");
			setSelectedEntryID(null);
			setLibraryState(result.entries.length === 0 ? "empty" : "populated");
		} catch (error) {
			if (activeLibraryRootRef.current !== root) return;

			setScanError(errorMessage(error));
			setLibraryState("error");
		}
	}

	return (
		<main className="app-shell" data-theme={theme}>
			<header className="app-header">
				<div className="brand">
					<div className="brand-mark" aria-hidden="true">
						C
					</div>
					<div>
						<p className="brand-kicker">Marvel Rivals mod manager</p>
						<h1>Cratebug</h1>
					</div>
				</div>
				<div className="header-controls">
					<label className="theme-control">
						<span>Appearance</span>
						<select
							value={theme}
							onChange={(event) => setTheme(event.target.value as Theme)}
						>
							<option value="system">System</option>
							<option value="light">Light</option>
							<option value="dark">Dark</option>
						</select>
					</label>
				</div>
			</header>

			<section className="library-toolbar" aria-label="Library scan controls">
				<form
					className="root-form"
					onSubmit={(event) => {
						event.preventDefault();
						void scan();
					}}
				>
					<label htmlFor="mod-root">Mod library folder</label>
					<input
						id="mod-root"
						type="text"
						value={modRoot}
						onChange={(event) => setModRoot(event.target.value)}
						placeholder="Paste the Marvel Rivals mod folder path"
						autoComplete="off"
					/>
					<button type="submit" disabled={libraryState === "loading" || isMutationLocked}>
						{libraryState === "loading" ? "Scanning..." : "Scan library"}
					</button>
				</form>
				{library && (
					<button
						type="button"
						className="quiet-button"
						onClick={() => scan()}
						disabled={libraryState === "loading" || isMutationLocked}
					>
						Refresh
					</button>
				)}
			</section>

			<section className="library-layout">
				<p className="visually-hidden" role="status">
					{statusMessage}
				</p>
				{mutatingEntryIDs.size > 0 && (
					<p className="visually-hidden" role="status">
						Updating mod state.
					</p>
				)}
				<aside className="library-sidebar" aria-label="Library folders">
					<div className="sidebar-heading">
						<span>Folders</span>
						{library && <span>{library.entries?.length ?? 0}</span>}
					</div>
					<FolderNavigation
						folders={libraryIndex.folders}
						selectedFolder={selectedFolder}
						onSelect={setSelectedFolder}
						entryCount={library?.entries?.length ?? 0}
						folderEntryCounts={libraryIndex.folderEntryCounts}
					/>
				</aside>

				<section className="catalog-panel" aria-label="Discovered mods">
					<div className="catalog-header">
						<div>
							<p className="eyebrow">Mod library</p>
							<h2>{selectedFolder === "all" ? "All mods" : selectedFolder}</h2>
						</div>
						<label className="search-control">
							<span className="visually-hidden">Search mods</span>
							<input
								value={search}
								onChange={(event) => setSearch(event.target.value)}
								placeholder="Search mods"
								type="search"
							/>
						</label>
						<fieldset className="view-mode-controls">
							<legend className="visually-hidden">Catalog view</legend>
							{viewModes.map((mode) => (
								<ViewModeButton
									active={viewMode === mode}
									key={mode}
									mode={mode}
									onSelect={setViewMode}
								/>
							))}
						</fieldset>
					</div>
					<SelectedModPanel
						entry={selectedEntry}
						isMutating={selectedEntry ? mutatingEntryIDs.has(selectedEntry.id) : false}
						isMutationLocked={isMutationLocked}
						onClear={() => setSelectedEntryID(null)}
						onSetEnabled={setModEnabled}
					/>
					<ModCatalog
						entries={displayedEntries}
						state={libraryState}
						scanError={scanError}
						hasLibrary={library !== null}
						mutatingEntryIDs={mutatingEntryIDs}
						onSetEnabled={setModEnabled}
						onSelect={(entry) =>
							setSelectedEntryID((currentEntryID) =>
								currentEntryID === entry.id ? null : entry.id,
							)
						}
						selectedEntryID={selectedEntryID}
						viewMode={viewMode}
					/>
				</section>
			</section>
			{mutationFeedback && (
				<MutationToast
					feedback={mutationFeedback}
					key={mutationFeedback.id}
					onDismiss={dismissMutationFeedback}
				/>
			)}
		</main>
	);
}

// Keeps mutation feedback out of the catalog layout while still allowing it to be dismissed.
function MutationToast({ feedback, onDismiss }: MutationToastProps) {
	const duration =
		feedback.kind === "success"
			? successToastDurationMilliseconds
			: errorToastDurationMilliseconds;
	const Icon = feedback.kind === "success" ? CircleCheckBig : CircleAlert;

	useEffect(() => {
		const timeout = window.setTimeout(onDismiss, duration);
		return () => window.clearTimeout(timeout);
	}, [duration, onDismiss]);

	return (
		<div
			className={`mutation-toast ${feedback.kind}`}
			role={feedback.kind === "error" ? "alert" : "status"}
		>
			<Icon aria-hidden="true" />
			<p>{feedback.message}</p>
			<button
				type="button"
				className="mutation-toast-close"
				onClick={onDismiss}
				aria-label="Dismiss"
			>
				<X aria-hidden="true" />
			</button>
		</div>
	);
}

// Keeps the current selection and its available actions in one stable location.
function SelectedModPanel({
	entry,
	isMutating,
	isMutationLocked,
	onClear,
	onSetEnabled,
}: SelectedModPanelProps) {
	if (!entry) {
		return (
			<section className="selected-mod-panel empty" aria-label="Mod actions">
				<div>
					<p className="eyebrow">Mod actions</p>
					<p>Select a mod to organize it.</p>
				</div>
				<p className="selected-mod-hint">
					Rename, priority, move, and deletion controls arrive next.
				</p>
			</section>
		);
	}

	const hasAmbiguousPrimary = entry.issues?.some((issue) => issue.code === "ambiguous-primary");
	const canChangeState =
		entry.kind === "mod" && entry.primaryPath !== undefined && !hasAmbiguousPrimary;
	const enabled = entry.state === "enabled";
	const stateLabel = entryStateLabel(entry);

	return (
		<section className="selected-mod-panel" aria-label="Selected mod actions">
			<div className="selected-mod-details">
				<p className="eyebrow">Selected mod</p>
				<h3>{entry.displayName}</h3>
				<p>
					{entry.relativeFolder || "Library root"} · {stateLabel} · Priority{" "}
					{entry.priority.value}
				</p>
			</div>
			<div className="selected-mod-actions">
				{canChangeState && (
					<button
						type="button"
						className="mod-action"
						disabled={isMutationLocked}
						onClick={() => onSetEnabled(entry)}
					>
						{isMutating
							? enabled
								? "Disabling..."
								: "Enabling..."
							: isMutationLocked
								? "Working..."
								: enabled
									? "Disable"
									: "Enable"}
					</button>
				)}
				<button
					type="button"
					className="quiet-button"
					disabled={isMutationLocked}
					onClick={onClear}
				>
					Clear selection
				</button>
			</div>
		</section>
	);
}

// Keeps the accessible name alongside an icon-only control.
function ViewModeButton({ active, mode, onSelect }: ViewModeButtonProps) {
	const Icon = viewModeIcons[mode];
	const label = viewModeLabels[mode];

	return (
		<button
			type="button"
			aria-pressed={active}
			className={active ? "selected" : ""}
			onClick={() => onSelect(mode)}
			title={`${label} view`}
		>
			<span className="visually-hidden">{label} view</span>
			<Icon aria-hidden="true" />
		</button>
	);
}
