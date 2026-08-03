import { type FormEvent, useMemo, useState } from "react";
import { Grid2X2, List, PanelsTopLeft } from "lucide-react";
import { ScanLibrary } from "../../wailsjs/go/main/App";
import type { discovery } from "../../wailsjs/go/models";
import { FolderNavigation } from "./FolderNavigation";
import { type LibraryState, type ViewMode, viewModeLabels, viewModes } from "./libraryTypes";
import { ModCatalog } from "./ModCatalog";

type Theme = "system" | "light" | "dark";

const viewModeIcons = {
	compact: Grid2X2,
	large: PanelsTopLeft,
	list: List,
} satisfies Record<ViewMode, typeof Grid2X2>;

function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}

// LibraryScreen owns local browsing state; scanning remains behind the Go binding.
export function LibraryScreen() {
	const [modRoot, setModRoot] = useState("");
	const [library, setLibrary] = useState<discovery.Library | null>(null);
	const [libraryState, setLibraryState] = useState<LibraryState>("initial");
	const [scanError, setScanError] = useState("");
	const [search, setSearch] = useState("");
	const [selectedFolder, setSelectedFolder] = useState("all");
	const [theme, setTheme] = useState<Theme>("system");
	const [viewMode, setViewMode] = useState<ViewMode>("compact");

	const folders = useMemo(() => {
		const paths = new Set<string>();
		for (const entry of library?.entries ?? []) {
			// Include ancestors so a parent folder can represent its full subtree.
			const segments = entry.relativeFolder.split("/").filter(Boolean);
			for (let index = 1; index <= segments.length; index += 1) {
				paths.add(segments.slice(0, index).join("/"));
			}
		}
		return [...paths].sort((left, right) => left.localeCompare(right));
	}, [library]);

	const folderEntryCounts = useMemo(() => {
		const counts = new Map<string, number>();
		for (const entry of library?.entries ?? []) {
			const segments = entry.relativeFolder.split("/").filter(Boolean);
			for (let index = 1; index <= segments.length; index += 1) {
				const folder = segments.slice(0, index).join("/");
				// Match the catalog's subtree filter, including entries beneath this folder.
				counts.set(folder, (counts.get(folder) ?? 0) + 1);
			}
		}
		return counts;
	}, [library]);

	const displayedEntries = useMemo(() => {
		const normalizedSearch = search.trim().toLocaleLowerCase();
		return (library?.entries ?? []).filter((entry) => {
			// Folder navigation filters the current catalog locally, including descendants.
			const inFolder =
				selectedFolder === "all" ||
				entry.relativeFolder === selectedFolder ||
				entry.relativeFolder.startsWith(`${selectedFolder}/`);
			const matchesSearch =
				normalizedSearch === "" ||
				entry.displayName.toLocaleLowerCase().includes(normalizedSearch) ||
				entry.primaryPath?.toLocaleLowerCase().includes(normalizedSearch) ||
				entry.relativeFolder.toLocaleLowerCase().includes(normalizedSearch);
			return inFolder && matchesSearch;
		});
	}, [library, search, selectedFolder]);

	async function scan(event?: FormEvent) {
		event?.preventDefault();
		const root = modRoot.trim();
		if (!root) {
			setLibraryState("initial");
			return;
		}

		setLibraryState("loading");
		setScanError("");
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
					<button type="submit" disabled={libraryState === "loading"}>
						{libraryState === "loading" ? "Scanning..." : "Scan library"}
					</button>
				</form>
				{library && (
					<button
						type="button"
						className="quiet-button"
						onClick={() => scan()}
						disabled={libraryState === "loading"}
					>
						Refresh
					</button>
				)}
			</section>

			<section className="library-layout" aria-live="polite">
				<aside className="library-sidebar" aria-label="Library folders">
					<div className="sidebar-heading">
						<span>Folders</span>
						{library && <span>{library.entries?.length ?? 0}</span>}
					</div>
					<FolderNavigation
						folders={folders}
						selectedFolder={selectedFolder}
						onSelect={setSelectedFolder}
						entryCount={library?.entries?.length ?? 0}
						folderEntryCounts={folderEntryCounts}
					/>
				</aside>

				<section className="catalog-panel" aria-label="Discovered mods">
					<div className="catalog-header">
						<div>
							<p className="eyebrow">Read-only library</p>
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
					<ModCatalog
						entries={displayedEntries}
						state={libraryState}
						scanError={scanError}
						hasLibrary={library !== null}
						viewMode={viewMode}
					/>
				</section>
			</section>
		</main>
	);
}

function ViewModeButton({
	active,
	mode,
	onSelect,
}: {
	active: boolean;
	mode: ViewMode;
	onSelect: (mode: ViewMode) => void;
}) {
	const Icon = viewModeIcons[mode];
	const label = viewModeLabels[mode];

	return (
		<button
			type="button"
			className={active ? "selected" : ""}
			onClick={() => onSelect(mode)}
			title={`${label} view`}
		>
			<span className="visually-hidden">{label} view</span>
			<Icon aria-hidden="true" />
		</button>
	);
}
