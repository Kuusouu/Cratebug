import { type FormEvent, useCallback, useMemo, useState } from "react";
import { Grid2X2, List, PanelsTopLeft } from "lucide-react";
import { ScanLibrary, SetModEnabled } from "../../wailsjs/go/main/App";
import { discovery } from "../../wailsjs/go/models";
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

const viewModeIcons = {
	compact: Grid2X2,
	large: PanelsTopLeft,
	list: List,
} satisfies Record<ViewMode, typeof Grid2X2>;

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
	const [mutationError, setMutationError] = useState("");
	const [mutatingPrimaryPath, setMutatingPrimaryPath] = useState<string | null>(null);
	const [search, setSearch] = useState("");
	const [selectedFolder, setSelectedFolder] = useState("all");
	const [theme, setTheme] = useState<Theme>("system");
	const [viewMode, setViewMode] = useState<ViewMode>("compact");

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

	const setModEnabled = useCallback(
		async (entry: discovery.Entry) => {
			if (!library) return;

			if (!entry.primaryPath) return;

			if (mutatingPrimaryPath) return;

			const enabled = entry.state !== "enabled";
			setMutationError("");
			setMutatingPrimaryPath(entry.primaryPath);

			try {
				const result = await SetModEnabled(library.root, entry.primaryPath, enabled);
				setLibrary((currentLibrary) => {
					if (!currentLibrary) return currentLibrary;

					const entries = currentLibrary.entries.map((currentEntry) => {
						if (currentEntry.primaryPath !== result.previousPrimaryPath)
							return currentEntry;

						return new discovery.Entry({
							...currentEntry,
							primaryPath: result.primaryPath,
							state: result.state,
						});
					});

					return new discovery.Library({ ...currentLibrary, entries });
				});
			} catch (error) {
				setMutationError(errorMessage(error));
			} finally {
				setMutatingPrimaryPath(null);
			}
		},
		[library, mutatingPrimaryPath],
	);

	// Replaces the catalog only after a scan finishes successfully.
	async function scan(event?: FormEvent) {
		event?.preventDefault();
		const root = modRoot.trim();
		if (!root) {
			setLibrary(null);
			setScanError("");
			setMutationError("");
			setSearch("");
			setSelectedFolder("all");
			setLibraryState("initial");
			return;
		}

		setLibraryState("loading");
		setScanError("");
		setMutationError("");
		try {
			const result = await ScanLibrary(root);
			setLibrary(result);
			// A fresh catalog may not contain the previous selection.
			setSelectedFolder("all");
			setLibraryState(result.entries.length === 0 ? "empty" : "populated");
		} catch (error) {
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
				<form className="root-form" onSubmit={scan}>
					<label htmlFor="mod-root">Mod library folder</label>
					<input
						id="mod-root"
						type="text"
						value={modRoot}
						onChange={(event) => setModRoot(event.target.value)}
						placeholder="Paste the Marvel Rivals mod folder path"
						autoComplete="off"
					/>
					<button
						type="submit"
						disabled={libraryState === "loading" || mutatingPrimaryPath !== null}
					>
						{libraryState === "loading" ? "Scanning..." : "Scan library"}
					</button>
				</form>
				{library && (
					<button
						type="button"
						className="quiet-button"
						onClick={() => scan()}
						disabled={libraryState === "loading" || mutatingPrimaryPath !== null}
					>
						Refresh
					</button>
				)}
			</section>

			<section className="library-layout">
				<p className="visually-hidden" role="status">
					{statusMessage}
				</p>
				{mutatingPrimaryPath && (
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
					{mutationError && (
						<p className="catalog-operation-error" role="alert">
							{mutationError}
						</p>
					)}
					<ModCatalog
						entries={displayedEntries}
						state={libraryState}
						scanError={scanError}
						hasLibrary={library !== null}
						mutatingPrimaryPath={mutatingPrimaryPath}
						onSetEnabled={setModEnabled}
						viewMode={viewMode}
					/>
				</section>
			</section>
		</main>
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
