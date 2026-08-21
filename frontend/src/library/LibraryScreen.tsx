import { CircleAlert, CircleCheckBig, Grid2X2, List, PanelsTopLeft, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RenameMod, ScanLibrary, SetModEnabled, SetModPriority } from "../../wailsjs/go/main/App";
import { discovery, type mutation } from "../../wailsjs/go/models";
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
	onOpenDialog: (dialog: MutationDialog) => void;
	onSetEnabled: (entry: discovery.Entry) => void;
};

type MutationDialog = "priority" | "rename";

type ModMutationDialogProps = {
	entry: discovery.Entry;
	isMutating: boolean;
	mode: MutationDialog;
	onClose: () => void;
	onRename: (entry: discovery.Entry, name: string) => Promise<boolean>;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
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
// The backend's compatible filename encoding supports priorities from 0 through 255.
const maximumModPriority = 255;

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
	const [activeDialog, setActiveDialog] = useState<MutationDialog | null>(null);
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
	const updateMutatedEntry = useCallback(
		(
			result: mutation.Result,
			changes: Partial<Pick<discovery.Entry, "displayName" | "priority">>,
		) => {
			setLibrary((currentLibrary) => {
				if (!currentLibrary) return currentLibrary;

				const entries = currentLibrary.entries.map((currentEntry) => {
					if (currentEntry.id !== result.previousID) return currentEntry;

					return new discovery.Entry({
						...currentEntry,
						...changes,
						id: result.id,
						primaryPath: result.primaryPath,
						state: result.state,
					});
				});

				return new discovery.Library({ ...currentLibrary, entries });
			});
			setSelectedEntryID(result.id);
		},
		[],
	);

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

	const renameMod = useCallback(
		async (entry: discovery.Entry, name: string): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0) return false;

			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				const result = await RenameMod(libraryRoot, entry.id, name);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				updateMutatedEntry(result, { displayName: name });
				showMutationFeedback("success", `Renamed ${entry.displayName} to ${name}.`);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not rename ${entry.displayName}: ${errorMessage(error)}`,
					);
				}
				return false;
			} finally {
				mutatingEntryIDsRef.current.delete(entry.id);
				setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));
			}
		},
		[libraryRoot, showMutationFeedback, updateMutatedEntry],
	);

	const setModPriority = useCallback(
		async (entry: discovery.Entry, priority: number): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0) return false;

			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				const result = await SetModPriority(libraryRoot, entry.id, priority);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				updateMutatedEntry(result, {
					priority: new discovery.Priority({ ...entry.priority, value: priority }),
				});
				showMutationFeedback(
					"success",
					`Set ${entry.displayName} to priority ${priority}.`,
				);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not set priority for ${entry.displayName}: ${errorMessage(error)}`,
					);
				}
				return false;
			} finally {
				mutatingEntryIDsRef.current.delete(entry.id);
				setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));
			}
		},
		[libraryRoot, showMutationFeedback, updateMutatedEntry],
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
			setActiveDialog(null);
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
			setActiveDialog(null);
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
						onClear={() => {
							setSelectedEntryID(null);
							setActiveDialog(null);
						}}
						onOpenDialog={setActiveDialog}
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
			{activeDialog && selectedEntry && (
				<ModMutationDialog
					entry={selectedEntry}
					isMutating={isMutationLocked}
					key={`${selectedEntry.id}-${activeDialog}`}
					mode={activeDialog}
					onClose={() => setActiveDialog(null)}
					onRename={renameMod}
					onSetPriority={setModPriority}
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

function ModMutationDialog({
	entry,
	isMutating,
	mode,
	onClose,
	onRename,
	onSetPriority,
}: ModMutationDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const [name, setName] = useState(entry.displayName);
	const [priority, setPriority] = useState(String(entry.priority.value));
	const [validationError, setValidationError] = useState("");
	const renameMode = mode === "rename";
	const title = renameMode ? "Rename mod" : "Set priority";

	useEffect(() => {
		inputRef.current?.focus();
	}, []);

	async function submit() {
		if (renameMode) {
			const error = renameValidationError(name);
			if (error) {
				setValidationError(error);
				return;
			}

			if (name === entry.displayName) {
				setValidationError("Enter a different mod name.");
				return;
			}

			if (await onRename(entry, name)) onClose();
			return;
		}

		if (priority.trim() === "") {
			setValidationError("Enter a priority.");
			return;
		}

		const value = Number(priority);
		if (!Number.isSafeInteger(value) || value < 0 || value > maximumModPriority) {
			setValidationError(`Priority must be a whole number from 0 to ${maximumModPriority}.`);
			return;
		}

		if (value === entry.priority.value) {
			setValidationError("Choose a different priority.");
			return;
		}

		if (await onSetPriority(entry, value)) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				className="mutation-dialog"
				aria-labelledby="mutation-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="mutation-dialog-title">{title}</h2>
					<p className="mutation-dialog-subtitle">{entry.displayName}</p>
				</div>
				<form
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{renameMode ? (
						<label className="mutation-dialog-field" htmlFor="rename-mod-name">
							<span>New name</span>
							<input
								id="rename-mod-name"
								ref={inputRef}
								value={name}
								onChange={(event) => {
									setName(event.target.value);
									setValidationError("");
								}}
							/>
						</label>
					) : (
						<label className="mutation-dialog-field" htmlFor="set-mod-priority">
							<span>Priority</span>
							<input
								id="set-mod-priority"
								inputMode="numeric"
								min="0"
								ref={inputRef}
								step="1"
								type="number"
								value={priority}
								onChange={(event) => {
									setPriority(event.target.value);
									setValidationError("");
								}}
							/>
						</label>
					)}
					{validationError && (
						<p className="mutation-dialog-error" role="alert">
							{validationError}
						</p>
					)}
					<div className="mutation-dialog-actions">
						<button
							type="button"
							className="quiet-button"
							disabled={isMutating}
							onClick={onClose}
						>
							Cancel
						</button>
						<button type="submit" disabled={isMutating}>
							{isMutating ? "Saving..." : renameMode ? "Rename" : "Set priority"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}

function renameValidationError(name: string): string | null {
	if (name.trim() === "") return "Enter a mod name.";
	if (name.endsWith(" ") || name.endsWith("."))
		return "A mod name cannot end with a space or period.";
	if (hasWindowsReservedCharacter(name))
		return "A mod name contains a Windows-reserved character.";

	return null;
}

function hasWindowsReservedCharacter(name: string): boolean {
	return /[<>:"/\\|?*]/.test(name) || [...name].some((character) => character.charCodeAt(0) < 32);
}

// Keeps the current selection and its available actions in one stable location.
function SelectedModPanel({
	entry,
	isMutating,
	isMutationLocked,
	onClear,
	onOpenDialog,
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
					Rename and priority actions are available for complete, unambiguous mods.
				</p>
			</section>
		);
	}

	const hasAmbiguousPrimary = entry.issues?.some((issue) => issue.code === "ambiguous-primary");
	const hasMissingSidecar = entry.issues?.some(
		(issue) => issue.code === "missing-utoc" || issue.code === "missing-ucas",
	);
	const canChangeState =
		entry.kind === "mod" && entry.primaryPath !== undefined && !hasAmbiguousPrimary;
	const canOrganize = canChangeState && !hasMissingSidecar;
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
				{canOrganize && (
					<>
						<button
							type="button"
							className="quiet-button"
							disabled={isMutationLocked}
							onClick={() => onOpenDialog("rename")}
						>
							Rename
						</button>
						<button
							type="button"
							className="quiet-button"
							disabled={isMutationLocked}
							onClick={() => onOpenDialog("priority")}
						>
							Priority
						</button>
					</>
				)}
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
