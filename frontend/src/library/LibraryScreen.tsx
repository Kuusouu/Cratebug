import {
	CircleAlert,
	CircleCheckBig,
	Grid2X2,
	List,
	PanelsTopLeft,
	Settings as SettingsIcon,
	TriangleAlert,
	X,
} from "lucide-react";
import {
	useCallback,
	type CSSProperties,
	useEffect,
	useMemo,
	type MouseEvent,
	useRef,
	useState,
} from "react";
import {
	AssignModTag,
	CreateFolder,
	CreateTag,
	DeleteMod,
	DeleteTag,
	LoadMetadata,
	MoveFolder,
	MoveMod,
	RenameFolder,
	RenameMod,
	RenameTag,
	ScanLibrary,
	SetAccentColor,
	SetDefaultViewMode,
	SetModEnabled,
	SetModPriority,
	SetModRoot,
	SetTheme,
	UnassignModTag,
} from "../../wailsjs/go/main/App";
import { discovery, type metadata, type mutation } from "../../wailsjs/go/models";
import { contrastingInk, isValidHexColor } from "./accentColor";
import { ContextMenu, type ContextMenuItem, type ContextMenuState } from "./ContextMenu";
import {
	canChangeModState,
	canDeleteMod,
	canOrganizeMod,
	canTagMod,
	entryStateLabel,
	hasMissingSidecar,
} from "./entryPresentation";
import { FolderNavigation } from "./FolderNavigation";
import {
	type DraggedItem,
	isValidDropTarget,
	type LibraryState,
	type Theme,
	themes,
	type ViewMode,
	viewModeLabels,
	viewModes,
} from "./libraryTypes";
import { ModCatalog } from "./ModCatalog";
import { SettingsDialog } from "./SettingsDialog";
import { TagMenu } from "./TagMenu";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

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
	assignedTags: metadata.Tag[];
	isMutating: boolean;
	isMutationLocked: boolean;
	onClear: () => void;
	onSetEnabled: (entry: discovery.Entry) => void;
};

type MutationDialog = "priority" | "rename" | "move" | "delete" | "tags";

type ModMutationDialogProps = {
	entry: discovery.Entry;
	folders: string[];
	isMutating: boolean;
	mode: MutationDialog;
	onClose: () => void;
	onMove: (entry: discovery.Entry, destinationFolder: string) => Promise<boolean>;
	onRename: (entry: discovery.Entry, name: string) => Promise<boolean>;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
};

type FolderDialogMode = "create" | "rename" | "move";

type FolderMutationDialogProps = {
	folders: string[];
	isMutating: boolean;
	mode: FolderDialogMode;
	targetFolder: string;
	onClose: () => void;
	onCreate: (parentFolder: string, name: string) => Promise<boolean>;
	onMove: (folder: string, destinationParent: string) => Promise<boolean>;
	onRename: (folder: string, name: string) => Promise<boolean>;
};

type DeleteConfirmDialogProps = {
	entry: discovery.Entry;
	isMutating: boolean;
	onClose: () => void;
	onConfirm: (entry: discovery.Entry) => Promise<boolean>;
};

type ModTagDialogProps = {
	entry: discovery.Entry;
	catalog: metadata.Tag[];
	assignedTagIDs: ReadonlySet<string>;
	onClose: () => void;
	onCreateAndAssign: (name: string) => Promise<boolean>;
	onToggle: (tag: metadata.Tag, assign: boolean) => Promise<boolean>;
};

type MutationFeedback = {
	id: number;
	kind: "error" | "success" | "warning";
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
// Mirrors internal/mutation's Windows filename component limit (see maximumFileNameUTF16CodeUnits).
const maximumFileNameUTF16CodeUnits = 255;
// Mirrors discovery.MinimumTrailingNines, the shortest trailing-nine priority form.
const minimumTrailingNines = 7;
// SPEC.md requires a short deliberate delay before destructive confirmation.
const deleteConfirmDelaySeconds = 3;

// Converts unknown Wails failures into displayable scan errors.
function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}

// Build subtree lookups once per scan so folder navigation does not repeatedly scan the library.
// Starts from the scanner's complete folder list, not just folders containing mods, so an empty
// folder created through Cratebug still appears and stays selectable.
function indexLibrary(library: discovery.Library | null): LibraryIndex {
	const folderEntries = new Map<string, discovery.Entry[]>();
	const folders = new Set<string>(library?.folders ?? []);

	for (const entry of library?.entries ?? []) {
		const segments = entry.relativeFolder.split("/").filter(Boolean);
		for (let index = 1; index <= segments.length; index += 1) {
			const folder = segments.slice(0, index).join("/");
			folders.add(folder);
			const entriesInFolder = folderEntries.get(folder);
			if (entriesInFolder) {
				entriesInFolder.push(entry);
			} else {
				folderEntries.set(folder, [entry]);
			}
		}
	}

	return {
		folders: [...folders].sort((left, right) => left.localeCompare(right)),
		folderEntries,
		folderEntryCounts: new Map(
			[...folderEntries].map(([folder, entriesInFolder]) => [folder, entriesInFolder.length]),
		),
	};
}

// Keeps a folder selection pointed at the correct subtree after a rename or move.
function remapFolderSelection(
	selectedFolder: string,
	oldFolder: string,
	newFolder: string,
): string {
	if (selectedFolder === oldFolder) return newFolder;
	if (selectedFolder.startsWith(`${oldFolder}/`)) {
		return newFolder + selectedFolder.slice(oldFolder.length);
	}
	return selectedFolder;
}

// Mod records are keyed by a persistent identity, not the scanner ID entries
// carry, so the current entry's tags are found by matching its scanner ID
// against each record rather than by a direct lookup.
function tagIDsForScannerID(
	document: metadata.Document | null,
	scannerID: string | null,
): ReadonlySet<string> {
	if (!document?.mods || !scannerID) return new Set();

	for (const record of Object.values(document.mods)) {
		if (record.scannerID === scannerID) return new Set(record.tags ?? []);
	}
	return new Set();
}

// Resolves every mod's assigned tags against the catalog once per metadata
// change, keyed by scanner ID for a direct per-card lookup. This is a
// separate structure from tagIDsForScannerID, not a shared helper: that
// helper scans every mod record for one selected entry, which is fine for
// the single-selection panel but would become O(entries x mods) if reused
// per card on every render.
function tagsByEntryID(document: metadata.Document | null): ReadonlyMap<string, metadata.Tag[]> {
	const map = new Map<string, metadata.Tag[]>();
	if (!document) return map;

	const catalogByID = new Map(document.tags?.map((tag) => [tag.id, tag]) ?? []);
	for (const record of Object.values(document.mods ?? {})) {
		const tags = (record.tags ?? [])
			.map((tagID) => catalogByID.get(tagID))
			.filter((tag): tag is metadata.Tag => tag !== undefined);
		if (tags.length > 0) {
			map.set(record.scannerID, tags);
		}
	}
	return map;
}

function folderParent(folder: string): string {
	const separatorIndex = folder.lastIndexOf("/");
	return separatorIndex === -1 ? "" : folder.slice(0, separatorIndex);
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
	const [activeFolderDialog, setActiveFolderDialog] = useState<FolderDialogMode | null>(null);
	const [folderDialogTarget, setFolderDialogTarget] = useState("");
	const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
	const [isFolderMutating, setIsFolderMutating] = useState(false);
	const [theme, setTheme] = useState<Theme>("system");
	const [viewMode, setViewMode] = useState<ViewMode>("compact");
	const [settingsOpen, setSettingsOpen] = useState(false);
	const [metadataDocument, setMetadataDocument] = useState<metadata.Document | null>(null);
	const [tagFilterIDs, setTagFilterIDs] = useState<ReadonlySet<string>>(new Set());
	const [accentColor, setAccentColor] = useState("");
	const [draggedItem, setDraggedItem] = useState<DraggedItem | null>(null);
	const activeLibraryRootRef = useRef<string | null>(null);
	const mutatingEntryIDsRef = useRef(new Set<string>());
	const isFolderMutatingRef = useRef(false);
	const nextMutationFeedbackIDRef = useRef(0);
	const hasLoadedInitialMetadataRef = useRef(false);
	const accentColorSaveTimeoutRef = useRef<number | undefined>(undefined);
	const libraryRoot = library?.root;

	const libraryIndex = useMemo(() => indexLibrary(library), [library]);
	const assignedTagIDsForSelection = useMemo(
		() => tagIDsForScannerID(metadataDocument, selectedEntryID),
		[metadataDocument, selectedEntryID],
	);
	const tagCatalog = metadataDocument?.tags ?? [];
	const assignedTagsForSelection = useMemo(
		() => tagCatalog.filter((tag) => assignedTagIDsForSelection.has(tag.id)),
		[tagCatalog, assignedTagIDsForSelection],
	);
	const entryTags = useMemo(() => tagsByEntryID(metadataDocument), [metadataDocument]);

	const displayedEntries = useMemo(() => {
		const normalizedSearch = search.trim().toLocaleLowerCase();
		const scopedEntries =
			selectedFolder === "all"
				? (library?.entries ?? [])
				: (libraryIndex.folderEntries.get(selectedFolder) ?? []);

		return scopedEntries.filter((entry) => {
			if (normalizedSearch !== "") {
				const matchesSearch =
					entry.displayName.toLocaleLowerCase().includes(normalizedSearch) ||
					entry.primaryPath?.toLocaleLowerCase().includes(normalizedSearch) ||
					entry.relativeFolder.toLocaleLowerCase().includes(normalizedSearch);
				if (!matchesSearch) return false;
			}

			if (tagFilterIDs.size > 0) {
				const tags = entryTags.get(entry.id) ?? [];
				if (!tags.some((tag) => tagFilterIDs.has(tag.id))) return false;
			}

			return true;
		});
	}, [library, libraryIndex, search, selectedFolder, tagFilterIDs, entryTags]);
	const statusMessage = libraryStatusMessage(
		libraryState,
		scanError,
		displayedEntries.length,
		selectedFolder,
		search,
		viewMode,
	);
	const selectedEntry = library?.entries.find((entry) => entry.id === selectedEntryID) ?? null;
	const isMutationLocked = mutatingEntryIDs.size > 0 || isFolderMutating;
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

	// Refreshes the catalog after a folder-level or cross-folder mutation without
	// resetting search, folder selection, or view mode the way a user-initiated scan does.
	const reloadLibrary = useCallback(async (): Promise<discovery.Library | null> => {
		if (!libraryRoot) return null;

		const requestRoot = libraryRoot;
		try {
			const result = await ScanLibrary(requestRoot);
			if (activeLibraryRootRef.current !== requestRoot) return null;

			setLibrary(result);
			setLibraryState(result.entries.length === 0 ? "empty" : "populated");
			return result;
		} catch (error) {
			if (activeLibraryRootRef.current === requestRoot) {
				showMutationFeedback(
					"error",
					`Could not refresh the library: ${errorMessage(error)}`,
				);
			}
			return null;
		}
	}, [libraryRoot, showMutationFeedback]);

	// Tags live in Cratebug's own persisted metadata, not the scanned library, so
	// refreshing after a change re-reads that store instead of rescanning the mod
	// root. Every mod-level mutation that can change a mod's scanner ID (rename,
	// priority, move) must also call this: the backend already re-points that
	// mod's tags at its new scanner ID, but the frontend's cached metadata
	// document still has the old one until this refetches it.
	const refreshMetadata = useCallback(async () => {
		const state = await LoadMetadata();
		setMetadataDocument(state.document);
	}, []);

	// Applies immediately, matching every other single-click preference in the
	// app: the icon shows the new theme right away, and only reverts if the
	// backend save actually fails.
	const selectTheme = useCallback(
		async (nextTheme: Theme) => {
			const previousTheme = theme;
			setTheme(nextTheme);
			try {
				await SetTheme(nextTheme);
			} catch (error) {
				setTheme(previousTheme);
				showMutationFeedback("error", `Could not save theme: ${errorMessage(error)}`);
			}
		},
		[theme, showMutationFeedback],
	);

	// The view mode a user leaves the catalog in is remembered automatically;
	// there is no separate "default view" setting to configure.
	const selectViewMode = useCallback(
		(nextViewMode: ViewMode) => {
			setViewMode(nextViewMode);
			SetDefaultViewMode(nextViewMode).catch((error) => {
				showMutationFeedback(
					"warning",
					`Could not remember this view for next launch: ${errorMessage(error)}`,
				);
			});
		},
		[showMutationFeedback],
	);

	// Applies locally on every keystroke for instant feedback, but debounces
	// the actual save so typing a hex value doesn't write to disk on every
	// character.
	const selectAccentColor = useCallback(
		(hex: string) => {
			setAccentColor(hex);
			window.clearTimeout(accentColorSaveTimeoutRef.current);
			accentColorSaveTimeoutRef.current = window.setTimeout(() => {
				SetAccentColor(hex).catch((error) => {
					showMutationFeedback(
						"warning",
						`Could not save accent color: ${errorMessage(error)}`,
					);
				});
			}, 400);
		},
		[showMutationFeedback],
	);

	const setModEnabled = useCallback(
		async (entry: discovery.Entry) => {
			if (!libraryRoot) return;

			if (
				!entry.primaryPath ||
				mutatingEntryIDsRef.current.size > 0 ||
				isFolderMutatingRef.current
			)
				return;

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
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				const result = await RenameMod(libraryRoot, entry.id, name);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				updateMutatedEntry(result, { displayName: name });
				await refreshMetadata();
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
		[libraryRoot, refreshMetadata, showMutationFeedback, updateMutatedEntry],
	);

	const setModPriority = useCallback(
		async (entry: discovery.Entry, priority: number): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				const result = await SetModPriority(libraryRoot, entry.id, priority);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				updateMutatedEntry(result, {
					priority: new discovery.Priority({ ...entry.priority, value: priority }),
				});
				await refreshMetadata();
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
		[libraryRoot, refreshMetadata, showMutationFeedback, updateMutatedEntry],
	);

	// A folder move changes the destination's scanner identity for every entry it
	// carries, not just the one primary being moved, so this reconciles by rescanning
	// rather than patching the single entry the way rename and priority do.
	const moveModToFolder = useCallback(
		async (entry: discovery.Entry, destinationFolder: string): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				const result = await MoveMod(libraryRoot, entry.id, destinationFolder);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await reloadLibrary();
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await refreshMetadata();
				setSelectedEntryID(result.id);
				showMutationFeedback(
					"success",
					`Moved ${entry.displayName} to ${destinationFolder || "Library root"}.`,
				);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not move ${entry.displayName}: ${errorMessage(error)}`,
					);
				}
				return false;
			} finally {
				mutatingEntryIDsRef.current.delete(entry.id);
				setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));
			}
		},
		[libraryRoot, reloadLibrary, refreshMetadata, showMutationFeedback],
	);

	// The UI-level confirmation delay lives in DeleteConfirmDialog; the backend
	// only requires a `confirmed` flag, not its own timing policy.
	const deleteMod = useCallback(
		async (entry: discovery.Entry): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			mutatingEntryIDsRef.current.add(entry.id);
			setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));

			try {
				await DeleteMod(libraryRoot, entry.id, true);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await reloadLibrary();
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				setSelectedEntryID(null);
				showMutationFeedback("success", `Sent ${entry.displayName} to the Recycle Bin.`);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not delete ${entry.displayName}: ${errorMessage(error)}`,
					);
				}
				return false;
			} finally {
				mutatingEntryIDsRef.current.delete(entry.id);
				setMutatingEntryIDs(new Set(mutatingEntryIDsRef.current));
			}
		},
		[libraryRoot, reloadLibrary, showMutationFeedback],
	);

	// Tag requests use their own dialog-local busy state rather than the
	// mod/folder mutation lock: unlike rename, move, or delete, they never touch
	// mod files and cannot race a filesystem operation.
	const createAndAssignTag = useCallback(
		async (entry: discovery.Entry, name: string): Promise<boolean> => {
			try {
				const tag = await CreateTag(name);
				await AssignModTag(entry.id, tag.id);
				await refreshMetadata();
				showMutationFeedback("success", `Created and assigned tag "${tag.name}".`);
				return true;
			} catch (error) {
				showMutationFeedback("error", `Could not create tag: ${errorMessage(error)}`);
				return false;
			}
		},
		[refreshMetadata, showMutationFeedback],
	);

	const toggleModTag = useCallback(
		async (entry: discovery.Entry, tag: metadata.Tag, assign: boolean): Promise<boolean> => {
			try {
				if (assign) {
					await AssignModTag(entry.id, tag.id);
				} else {
					await UnassignModTag(entry.id, tag.id);
				}
				await refreshMetadata();
				showMutationFeedback(
					"success",
					assign
						? `Added tag "${tag.name}" to ${entry.displayName}.`
						: `Removed tag "${tag.name}" from ${entry.displayName}.`,
				);
				return true;
			} catch (error) {
				showMutationFeedback("error", `Could not update tags: ${errorMessage(error)}`);
				return false;
			}
		},
		[refreshMetadata, showMutationFeedback],
	);

	const toggleTagFilter = useCallback((tagID: string) => {
		setTagFilterIDs((current) => {
			const next = new Set(current);
			if (next.has(tagID)) {
				next.delete(tagID);
			} else {
				next.add(tagID);
			}
			return next;
		});
	}, []);

	const createTag = useCallback(
		async (name: string): Promise<boolean> => {
			try {
				const tag = await CreateTag(name);
				await refreshMetadata();
				showMutationFeedback("success", `Created tag "${tag.name}".`);
				return true;
			} catch (error) {
				showMutationFeedback("error", `Could not create tag: ${errorMessage(error)}`);
				return false;
			}
		},
		[refreshMetadata, showMutationFeedback],
	);

	const renameTag = useCallback(
		async (tag: metadata.Tag, name: string): Promise<boolean> => {
			try {
				await RenameTag(tag.id, name);
				await refreshMetadata();
				showMutationFeedback("success", `Renamed tag "${tag.name}" to "${name}".`);
				return true;
			} catch (error) {
				showMutationFeedback("error", `Could not rename tag: ${errorMessage(error)}`);
				return false;
			}
		},
		[refreshMetadata, showMutationFeedback],
	);

	// Also drops the tag from the active filter so a stale, now-nonexistent
	// tag ID cannot keep the catalog silently filtered down.
	const deleteTag = useCallback(
		async (tag: metadata.Tag): Promise<boolean> => {
			try {
				await DeleteTag(tag.id);
				await refreshMetadata();
				setTagFilterIDs((current) => {
					if (!current.has(tag.id)) return current;
					const next = new Set(current);
					next.delete(tag.id);
					return next;
				});
				showMutationFeedback("success", `Deleted tag "${tag.name}".`);
				return true;
			} catch (error) {
				showMutationFeedback("error", `Could not delete tag: ${errorMessage(error)}`);
				return false;
			}
		},
		[refreshMetadata, showMutationFeedback],
	);

	// Fire-and-forget, matching the plan's "immediate, not confirmed" removal:
	// unassigning only edits Cratebug's own metadata, never mod files, so it
	// doesn't need the busy-state gating a filesystem mutation would.
	const removeModTagFromCard = useCallback(
		(entry: discovery.Entry, tagID: string) => {
			const tag = tagCatalog.find((candidate) => candidate.id === tagID);
			if (!tag) return;
			void toggleModTag(entry, tag, false);
		},
		[tagCatalog, toggleModTag],
	);

	const createFolder = useCallback(
		async (parentFolder: string, name: string): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			isFolderMutatingRef.current = true;
			setIsFolderMutating(true);
			try {
				const result = await CreateFolder(libraryRoot, parentFolder, name);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await reloadLibrary();
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				const createdFolderPath = result.folderPath ?? name;
				setSelectedFolder(result.folderPath ?? "all");
				showMutationFeedback("success", `Created folder ${createdFolderPath}.`);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not create folder: ${errorMessage(error)}`,
					);
				}
				return false;
			} finally {
				isFolderMutatingRef.current = false;
				setIsFolderMutating(false);
			}
		},
		[libraryRoot, reloadLibrary, showMutationFeedback],
	);

	// Folder renames and moves change the scanner identity of every mod they carry,
	// so the previous selection is cleared rather than remapped to a stale entry ID.
	const renameFolder = useCallback(
		async (folder: string, name: string): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			isFolderMutatingRef.current = true;
			setIsFolderMutating(true);
			try {
				const result = await RenameFolder(libraryRoot, folder, name);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await reloadLibrary();
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				const destination = result.folderPath ?? folder;
				setSelectedFolder((current) => remapFolderSelection(current, folder, destination));
				setSelectedEntryID(null);
				setActiveDialog(null);
				showMutationFeedback("success", `Renamed folder to ${destination}.`);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not rename folder: ${errorMessage(error)}`,
					);
				}
				return false;
			} finally {
				isFolderMutatingRef.current = false;
				setIsFolderMutating(false);
			}
		},
		[libraryRoot, reloadLibrary, showMutationFeedback],
	);

	const moveFolder = useCallback(
		async (folder: string, destinationParent: string): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			isFolderMutatingRef.current = true;
			setIsFolderMutating(true);
			try {
				const result = await MoveFolder(libraryRoot, folder, destinationParent);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await reloadLibrary();
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				const destination = result.folderPath ?? folder;
				setSelectedFolder((current) => remapFolderSelection(current, folder, destination));
				setSelectedEntryID(null);
				setActiveDialog(null);
				showMutationFeedback("success", `Moved folder to ${destination}.`);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback("error", `Could not move folder: ${errorMessage(error)}`);
				}
				return false;
			} finally {
				isFolderMutatingRef.current = false;
				setIsFolderMutating(false);
			}
		},
		[libraryRoot, reloadLibrary, showMutationFeedback],
	);

	// Reuses isValidDropTarget rather than re-deriving the cycle check here,
	// matching the same validity FolderNavigation already used to decide
	// whether to show the drag-over highlight before this ever fires.
	const handleDropOnFolder = useCallback(
		(destinationFolder: string) => {
			const item = draggedItem;
			setDraggedItem(null);
			if (!isValidDropTarget(item, destinationFolder)) return;
			if (item?.type === "folder") {
				void moveFolder(item.path, destinationFolder);
			} else if (item?.type === "mod") {
				void moveModToFolder(item.entry, destinationFolder);
			}
		},
		[draggedItem, moveFolder, moveModToFolder],
	);

	const startDragFolder = useCallback((path: string) => {
		setDraggedItem({ type: "folder", path });
	}, []);

	const startDragMod = useCallback((entry: discovery.Entry) => {
		setDraggedItem({ type: "mod", entry });
	}, []);

	const endDrag = useCallback(() => setDraggedItem(null), []);

	// Lets a folder's actions reach it without first navigating into it.
	const openFolderContextMenu = useCallback((folder: string, event: MouseEvent) => {
		const container = (event.target as HTMLElement).closest<HTMLElement>(".app-shell");
		if (!container) return;

		const isRoot = folder === "";
		const items: ContextMenuItem[] = [
			{
				label: "New folder",
				onSelect: () => {
					setFolderDialogTarget(folder);
					setActiveFolderDialog("create");
				},
			},
		];

		if (!isRoot) {
			items.push(
				{
					label: "Rename folder",
					onSelect: () => {
						setFolderDialogTarget(folder);
						setActiveFolderDialog("rename");
					},
				},
				{
					label: "Move to...",
					onSelect: () => {
						setFolderDialogTarget(folder);
						setActiveFolderDialog("move");
					},
				},
			);
		}

		setContextMenu({
			x: event.clientX,
			y: event.clientY,
			container,
			title: isRoot ? "Library root" : folder,
			items,
		});
	}, []);

	// Selects the mod under the pointer so its actions and the panel agree on the target.
	const openModContextMenu = useCallback((entry: discovery.Entry, event: MouseEvent) => {
		const organizable = canOrganizeMod(entry);
		const deletable = canDeleteMod(entry);
		const taggable = canTagMod(entry);
		if (!organizable && !deletable && !taggable) return;

		const container = (event.target as HTMLElement).closest<HTMLElement>(".app-shell");
		if (!container) return;

		const items: ContextMenuItem[] = [];
		if (organizable) {
			items.push(
				{ label: "Rename", onSelect: () => setActiveDialog("rename") },
				{ label: "Priority", onSelect: () => setActiveDialog("priority") },
				{ label: "Move to...", onSelect: () => setActiveDialog("move") },
			);
		}
		if (taggable) {
			items.push({ label: "Tags...", onSelect: () => setActiveDialog("tags") });
		}
		if (deletable) {
			items.push({
				label: "Delete...",
				onSelect: () => setActiveDialog("delete"),
				destructive: true,
			});
		}

		setSelectedEntryID(entry.id);
		setContextMenu({
			x: event.clientX,
			y: event.clientY,
			container,
			title: entry.displayName,
			items,
		});
	}, []);

	// Replaces the catalog only after a scan finishes successfully. Accepts an
	// explicit root so the initial-launch scan of a persisted mod root does not
	// have to wait a render cycle for the modRoot input's state to catch up.
	async function scan(overrideRoot?: string) {
		if (isMutationLocked) return;

		const root = (overrideRoot ?? modRoot).trim();
		if (!root) {
			activeLibraryRootRef.current = null;
			setLibrary(null);
			setScanError("");
			setMutationFeedback(null);
			setSearch("");
			setSelectedFolder("all");
			setSelectedEntryID(null);
			setActiveDialog(null);
			setActiveFolderDialog(null);
			setContextMenu(null);
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
			setActiveFolderDialog(null);
			setContextMenu(null);
			setLibraryState(result.entries.length === 0 ? "empty" : "populated");
			// Best-effort: a failed save here does not affect the library that
			// just loaded successfully, so it only surfaces as its own notice.
			SetModRoot(root).catch((error) => {
				if (activeLibraryRootRef.current !== root) return;
				showMutationFeedback(
					"warning",
					`Could not remember this library folder for next launch: ${errorMessage(error)}`,
				);
			});
		} catch (error) {
			if (activeLibraryRootRef.current !== root) return;

			setScanError(errorMessage(error));
			setLibraryState("error");
		}
	}

	// Runs once, even under StrictMode's double-invoked effects: restores the
	// persisted mod root and scans it automatically, and surfaces a corrupt or
	// recovered metadata file as a non-crashing warning instead of losing track
	// of it. Fires after scan() so its own feedback reset cannot clear this
	// warning immediately after it is shown.
	// biome-ignore lint/correctness/useExhaustiveDependencies: scan is a plain function recreated every render, not a useCallback; depending on it would rerun this effect on every render instead of once on mount.
	useEffect(() => {
		if (hasLoadedInitialMetadataRef.current) return;
		hasLoadedInitialMetadataRef.current = true;

		void (async () => {
			try {
				const state = await LoadMetadata();
				setMetadataDocument(state.document);

				const persistedTheme = state.document.settings.theme;
				if (persistedTheme && themes.includes(persistedTheme as Theme)) {
					setTheme(persistedTheme as Theme);
				}
				const persistedViewMode = state.document.settings.defaultViewMode;
				if (persistedViewMode && viewModes.includes(persistedViewMode as ViewMode)) {
					setViewMode(persistedViewMode as ViewMode);
				}
				const persistedAccentColor = state.document.settings.accentColor;
				if (persistedAccentColor && isValidHexColor(persistedAccentColor)) {
					setAccentColor(persistedAccentColor);
				}

				const persistedRoot = state.document.settings.modRoot?.trim();
				if (persistedRoot) {
					setModRoot(persistedRoot);
					await scan(persistedRoot);
				}

				if (state.recovered) {
					showMutationFeedback(
						"warning",
						`Cratebug found a problem with your saved settings and recovered them from a backup: ${state.recoveryReason ?? "unknown cause"}.`,
					);
				}
			} catch (error) {
				showMutationFeedback(
					"warning",
					`Could not load saved settings: ${errorMessage(error)}`,
				);
			}
		})();
	}, [showMutationFeedback]);

	return (
		<main
			className="app-shell"
			data-theme={theme}
			style={
				accentColor
					? ({
							"--accent": accentColor,
							"--accent-ink": contrastingInk(accentColor),
						} as CSSProperties)
					: undefined
			}
		>
			<header className="app-header">
				<div className="brand">
					<div className="brand-mark" aria-hidden="true">
						<svg
							aria-hidden="true"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="1.8"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<rect x="3" y="5" width="18" height="14" rx="1.2" />
							<line x1="3" y1="10" x2="21" y2="10" />
							<line x1="3" y1="10" x2="21" y2="19" />
							<line x1="21" y1="10" x2="3" y2="19" />
						</svg>
					</div>
					<div>
						<p className="brand-kicker">Marvel Rivals mod manager</p>
						<h1 className="wordmark">Cratebug</h1>
					</div>
				</div>
				<div className="header-controls">
					<button
						type="button"
						className="icon-button"
						onClick={() => setSettingsOpen(true)}
						aria-label="Settings"
						title="Settings"
					>
						<SettingsIcon aria-hidden="true" />
					</button>
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
						{library && (
							<div className="sidebar-heading-actions">
								<span>{library.entries?.length ?? 0}</span>
								<button
									type="button"
									className="quiet-button sidebar-heading-action"
									disabled={isMutationLocked}
									onClick={() => {
										setFolderDialogTarget(
											selectedFolder === "all" ? "" : selectedFolder,
										);
										setActiveFolderDialog("create");
									}}
								>
									New folder
								</button>
							</div>
						)}
					</div>
					<FolderNavigation
						folders={libraryIndex.folders}
						selectedFolder={selectedFolder}
						onSelect={setSelectedFolder}
						onContextMenu={openFolderContextMenu}
						draggedItem={draggedItem}
						dragDisabled={isMutationLocked}
						onDragStartFolder={startDragFolder}
						onDragEnd={endDrag}
						onDropOnFolder={handleDropOnFolder}
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
						<div className="catalog-controls">
							<label className="search-control">
								<span className="visually-hidden">Search mods</span>
								<input
									value={search}
									onChange={(event) => setSearch(event.target.value)}
									placeholder="Search mods"
									type="search"
								/>
							</label>
							<TagMenu
								catalog={tagCatalog}
								filterIDs={tagFilterIDs}
								onToggleFilter={toggleTagFilter}
								onCreateTag={createTag}
								onRenameTag={renameTag}
								onDeleteTag={deleteTag}
							/>
						</div>
						<fieldset className="view-mode-controls">
							<legend className="visually-hidden">Catalog view</legend>
							{viewModes.map((mode) => (
								<ViewModeButton
									active={viewMode === mode}
									key={mode}
									mode={mode}
									onSelect={selectViewMode}
								/>
							))}
						</fieldset>
					</div>
					<SelectedModPanel
						entry={selectedEntry}
						assignedTags={assignedTagsForSelection}
						isMutating={selectedEntry ? mutatingEntryIDs.has(selectedEntry.id) : false}
						isMutationLocked={isMutationLocked}
						onClear={() => {
							setSelectedEntryID(null);
							setActiveDialog(null);
						}}
						onSetEnabled={setModEnabled}
					/>
					<ModCatalog
						entries={displayedEntries}
						state={libraryState}
						scanError={scanError}
						hasLibrary={library !== null}
						mutatingEntryIDs={mutatingEntryIDs}
						isMutationLocked={isMutationLocked}
						tagsByEntryID={entryTags}
						draggedEntryID={draggedItem?.type === "mod" ? draggedItem.entry.id : null}
						onSetEnabled={setModEnabled}
						onSelect={(entry) =>
							setSelectedEntryID((currentEntryID) =>
								currentEntryID === entry.id ? null : entry.id,
							)
						}
						onContextMenu={openModContextMenu}
						onRemoveTag={removeModTagFromCard}
						onDragStartMod={startDragMod}
						onDragEndMod={endDrag}
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
			{activeDialog &&
				activeDialog !== "delete" &&
				activeDialog !== "tags" &&
				selectedEntry && (
					<ModMutationDialog
						entry={selectedEntry}
						folders={libraryIndex.folders}
						isMutating={isMutationLocked}
						key={`${selectedEntry.id}-${activeDialog}`}
						mode={activeDialog}
						onClose={() => setActiveDialog(null)}
						onMove={moveModToFolder}
						onRename={renameMod}
						onSetPriority={setModPriority}
					/>
				)}
			{activeDialog === "delete" && selectedEntry && (
				<DeleteConfirmDialog
					entry={selectedEntry}
					isMutating={isMutationLocked}
					key={selectedEntry.id}
					onClose={() => setActiveDialog(null)}
					onConfirm={deleteMod}
				/>
			)}
			{activeDialog === "tags" && selectedEntry && (
				<ModTagDialog
					entry={selectedEntry}
					catalog={tagCatalog}
					assignedTagIDs={assignedTagIDsForSelection}
					key={selectedEntry.id}
					onClose={() => setActiveDialog(null)}
					onCreateAndAssign={(name) => createAndAssignTag(selectedEntry, name)}
					onToggle={(tag, assign) => toggleModTag(selectedEntry, tag, assign)}
				/>
			)}
			{activeFolderDialog && library && (
				<FolderMutationDialog
					folders={libraryIndex.folders}
					isMutating={isMutationLocked}
					key={`${folderDialogTarget}-${activeFolderDialog}`}
					mode={activeFolderDialog}
					targetFolder={folderDialogTarget}
					onClose={() => setActiveFolderDialog(null)}
					onCreate={createFolder}
					onMove={moveFolder}
					onRename={renameFolder}
				/>
			)}
			{contextMenu && (
				<ContextMenu state={contextMenu} onClose={() => setContextMenu(null)} />
			)}
			{settingsOpen && (
				<SettingsDialog
					theme={theme}
					accentColor={accentColor}
					onClose={() => setSettingsOpen(false)}
					onSelectTheme={selectTheme}
					onSelectAccentColor={selectAccentColor}
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
	const Icon =
		feedback.kind === "success"
			? CircleCheckBig
			: feedback.kind === "warning"
				? TriangleAlert
				: CircleAlert;

	useEffect(() => {
		const timeout = window.setTimeout(onDismiss, duration);
		return () => window.clearTimeout(timeout);
	}, [duration, onDismiss]);

	return (
		<div
			className={`mutation-toast ${feedback.kind}`}
			role={feedback.kind === "success" ? "status" : "alert"}
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
	folders,
	isMutating,
	mode,
	onClose,
	onMove,
	onRename,
	onSetPriority,
}: ModMutationDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const selectRef = useRef<HTMLSelectElement>(null);
	const [name, setName] = useState(entry.displayName);
	const [priority, setPriority] = useState(String(entry.priority.value));
	const [destinationFolder, setDestinationFolder] = useState(entry.relativeFolder);
	const [validationError, setValidationError] = useState("");
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const renameMode = mode === "rename";
	const priorityMode = mode === "priority";
	const moveMode = mode === "move";
	const title = renameMode ? "Rename mod" : priorityMode ? "Set priority" : "Move mod";

	useEffect(() => {
		if (moveMode) {
			selectRef.current?.focus();
		} else {
			inputRef.current?.focus();
		}
	}, [moveMode]);

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

		if (moveMode) {
			if (destinationFolder === entry.relativeFolder) {
				setValidationError("Choose a different folder.");
				return;
			}

			if (await onMove(entry, destinationFolder)) onClose();
			return;
		}

		if (priority.trim() === "") {
			setValidationError("Enter a priority.");
			return;
		}

		const value = Number(priority);
		const maximumPriority = maximumPriorityFor(entry);
		if (!Number.isSafeInteger(value) || value < 0 || value > maximumPriority) {
			setValidationError(`Priority must be a whole number from 0 to ${maximumPriority}.`);
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
				ref={dialogRef}
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
					) : moveMode ? (
						<label className="mutation-dialog-field" htmlFor="move-mod-folder">
							<span>Destination folder</span>
							<select
								id="move-mod-folder"
								ref={selectRef}
								value={destinationFolder}
								onChange={(event) => {
									setDestinationFolder(event.target.value);
									setValidationError("");
								}}
							>
								<option value="">Library root</option>
								{folders.map((folder) => (
									<option key={folder} value={folder}>
										{folder}
									</option>
								))}
							</select>
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
							{isMutating
								? "Saving..."
								: renameMode
									? "Rename"
									: priorityMode
										? "Set priority"
										: "Move"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}

function FolderMutationDialog({
	folders,
	isMutating,
	mode,
	targetFolder,
	onClose,
	onCreate,
	onMove,
	onRename,
}: FolderMutationDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const selectRef = useRef<HTMLSelectElement>(null);
	const createMode = mode === "create";
	const renameMode = mode === "rename";
	const moveMode = mode === "move";
	const currentName = targetFolder.split("/").at(-1) ?? targetFolder;
	const currentParent = folderParent(targetFolder);
	const [name, setName] = useState(renameMode ? currentName : "");
	const [destinationParent, setDestinationParent] = useState(currentParent);
	const [validationError, setValidationError] = useState("");
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const title = createMode ? "New folder" : renameMode ? "Rename folder" : "Move folder";
	const subtitle = createMode
		? targetFolder
			? `Inside ${targetFolder}`
			: "Inside the library root"
		: targetFolder;

	useEffect(() => {
		if (moveMode) {
			selectRef.current?.focus();
		} else {
			inputRef.current?.focus();
		}
	}, [moveMode]);

	// Excludes the folder itself and its descendants: a folder cannot move into itself or a child.
	const moveDestinations = useMemo(
		() =>
			folders.filter(
				(folder) => folder !== targetFolder && !folder.startsWith(`${targetFolder}/`),
			),
		[folders, targetFolder],
	);

	async function submit() {
		if (moveMode) {
			if (destinationParent === currentParent) {
				setValidationError("Choose a different folder.");
				return;
			}

			if (await onMove(targetFolder, destinationParent)) onClose();
			return;
		}

		const error = renameValidationError(name, "folder");
		if (error) {
			setValidationError(error);
			return;
		}

		if (createMode) {
			if (await onCreate(targetFolder, name)) onClose();
			return;
		}

		if (name === currentName) {
			setValidationError("Enter a different folder name.");
			return;
		}

		if (await onRename(targetFolder, name)) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="folder-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Folder action</p>
					<h2 id="folder-dialog-title">{title}</h2>
					<p className="mutation-dialog-subtitle">{subtitle}</p>
				</div>
				<form
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{moveMode ? (
						<label className="mutation-dialog-field" htmlFor="move-folder-destination">
							<span>Destination folder</span>
							<select
								id="move-folder-destination"
								ref={selectRef}
								value={destinationParent}
								onChange={(event) => {
									setDestinationParent(event.target.value);
									setValidationError("");
								}}
							>
								<option value="">Library root</option>
								{moveDestinations.map((folder) => (
									<option key={folder} value={folder}>
										{folder}
									</option>
								))}
							</select>
						</label>
					) : (
						<label className="mutation-dialog-field" htmlFor="folder-name">
							<span>{createMode ? "Folder name" : "New name"}</span>
							<input
								id="folder-name"
								ref={inputRef}
								value={name}
								onChange={(event) => {
									setName(event.target.value);
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
							{isMutating
								? "Saving..."
								: createMode
									? "Create"
									: renameMode
										? "Rename"
										: "Move"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}

// Sends a scanner-recognized bundle to the Recycle Bin after a short delay
// gates the confirm button, matching SPEC.md's UI safeguard requirement.
// The backend enforces the actual safety checks; this dialog cannot bypass them.
function DeleteConfirmDialog({ entry, isMutating, onClose, onConfirm }: DeleteConfirmDialogProps) {
	const [secondsRemaining, setSecondsRemaining] = useState(deleteConfirmDelaySeconds);
	const ready = secondsRemaining <= 0;
	const cancelRef = useRef<HTMLButtonElement>(null);

	useEffect(() => {
		if (secondsRemaining <= 0) return;
		const timeout = window.setTimeout(
			() => setSecondsRemaining((current) => current - 1),
			1000,
		);
		return () => window.clearTimeout(timeout);
	}, [secondsRemaining]);

	// Cancel, not the destructive action, gets initial focus. This also puts
	// focus inside the dialog so the shared focus trap's Escape/Tab handling
	// (which listens on the dialog element and relies on the keydown bubbling
	// from whatever currently has focus) actually has something to bubble from.
	useEffect(() => {
		cancelRef.current?.focus();
	}, []);

	const missingSidecar = hasMissingSidecar(entry);
	const bundleFiles = [entry.primaryPath, entry.sidecars.utoc, entry.sidecars.ucas]
		.filter((path): path is string => Boolean(path))
		.map((path) => path.split("/").pop() ?? path);
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	async function handleConfirm() {
		if (await onConfirm(entry)) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="delete-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="delete-dialog-title">Delete mod</h2>
					<p className="mutation-dialog-subtitle">{entry.displayName}</p>
				</div>
				<p className="delete-confirm-summary">
					Sends {bundleFiles.join(", ")} to the Recycle Bin. You can restore it from there
					until the Recycle Bin is emptied.
				</p>
				{missingSidecar && (
					<p className="delete-confirm-warning" role="alert">
						This bundle is missing a recognized file. Only the files listed above will
						be removed.
					</p>
				)}
				<div className="mutation-dialog-actions">
					<button
						ref={cancelRef}
						type="button"
						className="quiet-button"
						disabled={isMutating}
						onClick={onClose}
					>
						Cancel
					</button>
					<button
						type="button"
						className="destructive-button"
						disabled={!ready || isMutating}
						onClick={() => void handleConfirm()}
					>
						{isMutating
							? "Deleting..."
							: ready
								? "Delete"
								: `Delete (${secondsRemaining})`}
					</button>
				</div>
			</section>
		</div>
	);
}

// Applies each tag toggle and the create-and-assign action immediately
// rather than staging changes behind a Save button, since every checkbox is
// already its own independent, atomic backend call.
function ModTagDialog({
	entry,
	catalog,
	assignedTagIDs,
	onClose,
	onCreateAndAssign,
	onToggle,
}: ModTagDialogProps) {
	const inputRef = useRef<HTMLInputElement>(null);
	const [newTagName, setNewTagName] = useState("");
	const [validationError, setValidationError] = useState("");
	const [togglingTagID, setTogglingTagID] = useState<string | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const isBusy = isCreating || togglingTagID !== null;
	const handleEscape = useCallback(() => {
		if (!isBusy) onClose();
	}, [isBusy, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		inputRef.current?.focus();
	}, []);

	async function handleToggle(tag: metadata.Tag, assign: boolean) {
		setTogglingTagID(tag.id);
		try {
			await onToggle(tag, assign);
		} finally {
			setTogglingTagID(null);
		}
	}

	async function submitNewTag() {
		const name = newTagName.trim();
		if (name === "") {
			setValidationError("Enter a tag name.");
			return;
		}

		setIsCreating(true);
		try {
			if (await onCreateAndAssign(name)) {
				setNewTagName("");
				setValidationError("");
			}
		} finally {
			setIsCreating(false);
		}
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="tag-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="tag-dialog-title">Tags</h2>
					<p className="mutation-dialog-subtitle">{entry.displayName}</p>
				</div>
				{catalog.length > 0 ? (
					<ul className="tag-checklist">
						{catalog.map((tag) => {
							const assigned = assignedTagIDs.has(tag.id);
							return (
								<li key={tag.id}>
									<label>
										<input
											type="checkbox"
											checked={assigned}
											disabled={isBusy}
											onChange={() => void handleToggle(tag, !assigned)}
										/>
										<span>{tag.name}</span>
									</label>
								</li>
							);
						})}
					</ul>
				) : (
					<p className="mutation-dialog-subtitle">No tags yet. Create one below.</p>
				)}
				<form
					onSubmit={(event) => {
						event.preventDefault();
						void submitNewTag();
					}}
				>
					<label className="mutation-dialog-field" htmlFor="new-tag-name">
						<span>New tag</span>
						<input
							id="new-tag-name"
							ref={inputRef}
							value={newTagName}
							disabled={isBusy}
							onChange={(event) => {
								setNewTagName(event.target.value);
								setValidationError("");
							}}
						/>
					</label>
					{validationError && (
						<p className="mutation-dialog-error" role="alert">
							{validationError}
						</p>
					)}
					<div className="mutation-dialog-actions">
						<button
							type="button"
							className="quiet-button"
							disabled={isBusy}
							onClick={onClose}
						>
							Close
						</button>
						<button type="submit" disabled={isBusy}>
							{isCreating ? "Adding..." : "Add tag"}
						</button>
					</div>
				</form>
			</section>
		</div>
	);
}

function renameValidationError(name: string, subject: "mod" | "folder" = "mod"): string | null {
	if (name.trim() === "") return `Enter a ${subject} name.`;
	if (name.endsWith(" ") || name.endsWith("."))
		return `A ${subject} name cannot end with a space or period.`;
	if (hasWindowsReservedCharacter(name))
		return `A ${subject} name contains a Windows-reserved character.`;

	return null;
}

function hasWindowsReservedCharacter(name: string): boolean {
	return /[<>:"/\\|?*]/.test(name) || [...name].some((character) => character.charCodeAt(0) < 32);
}

// The trailing-nine priority filename grows with the priority value itself
// (more nines), so the true ceiling depends on the mod's own name length and
// its longest present file suffix, not a single constant shared by every mod.
function maximumPriorityFor(entry: discovery.Entry): number {
	const currentStemLength = entry.priority.raw.length;
	const suffixLengths = [entry.primaryPath, entry.sidecars.utoc, entry.sidecars.ucas]
		.filter((path): path is string => path !== undefined)
		.map((path) => basename(path).length - currentStemLength);
	const longestSuffixLength = Math.max(0, ...suffixLengths);

	// Stem length for a positive priority is name + "_" + nines + "_P".
	const fixedStemOverhead = entry.displayName.length + 1 + (minimumTrailingNines - 1) + 2;
	const ceiling = maximumFileNameUTF16CodeUnits - longestSuffixLength - fixedStemOverhead;
	return Math.max(0, Math.min(maximumFileNameUTF16CodeUnits, ceiling));
}

function basename(path: string): string {
	return path.split(/[/\\]/).pop() ?? path;
}

// Keeps the current selection and its available actions in one stable location.
function SelectedModPanel({
	entry,
	assignedTags,
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
					Right-click a mod for rename, priority, and move actions.
				</p>
			</section>
		);
	}

	const canChangeState = canChangeModState(entry);
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
				{assignedTags.length > 0 && (
					<ul className="selected-mod-tags" aria-label="Tags">
						{assignedTags.map((tag) => (
							<li key={tag.id}>{tag.name}</li>
						))}
					</ul>
				)}
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
