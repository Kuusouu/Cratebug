import {
	Grid2X2,
	Link,
	List,
	PanelsTopLeft,
	PackagePlus,
	Settings as SettingsIcon,
	ShieldAlert,
} from "lucide-react";
import styles from "./LibraryScreen.module.css";
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
	ApplyUpdate,
	AssignModTag,
	CheckForUpdate,
	CheckWhatsNew,
	ClassifyLibrary,
	CreateFolder,
	CreateLibrary,
	CreateTag,
	DeleteFolder,
	DeleteMod,
	DeleteTag,
	DetectConflicts,
	DetectLibrary,
	DownloadUpdate,
	GetAppVersion,
	LoadMetadata,
	MoveFolder,
	MoveMod,
	RenameFolder,
	RenameMod,
	RenameTag,
	ScanLibrary,
	SelectFilesForInstall,
	SetAccentColor,
	SetDefaultViewMode,
	SetLibraryProvider,
	SetModEnabled,
	SetModPriority,
	SetModRoot,
	SetTheme,
	UnassignModTag,
} from "../../wailsjs/go/main/App";
import {
	conflict,
	discovery,
	type gamedetect,
	type main,
	type metadata,
	type modtype,
	type mutation,
} from "../../wailsjs/go/models";
import { EventsOn, OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { contrastingInk, isValidHexColor } from "./accentColor";
import { ConflictDetailsDialog } from "./ConflictDetailsDialog";
import { ContextMenu, type ContextMenuItem, type ContextMenuState } from "./ContextMenu";
import { DeleteConfirmDialog } from "./DeleteConfirmDialog";
import { DetectLibraryDialog } from "./DetectLibraryDialog";
import { canDeleteMod, canOrganizeMod, canTagMod } from "./entryPresentation";
import { FolderDeleteConfirmDialog } from "./FolderDeleteConfirmDialog";
import { FolderMutationDialog } from "./FolderMutationDialog";
import { FolderNavigation } from "./FolderNavigation";
import { InstallFromUrlDialog } from "./InstallFromUrlDialog";
import { type InstallSource, InstallPreviewDialog } from "./InstallPreviewDialog";
import { formatWailsError as errorMessage } from "./installPresentation";
import { detectionOutcome } from "./libraryDetection";
import {
	type DraggedItem,
	isValidDropTarget,
	type LibraryProvider,
	type LibraryState,
	libraryProviderLabels,
	libraryProviders,
	type Theme,
	themes,
	type ViewMode,
	viewModeLabels,
	viewModes,
} from "./libraryTypes";
import { ModCatalog } from "./ModCatalog";
import { ModMutationDialog } from "./ModMutationDialog";
import { ModTagDialog } from "./ModTagDialog";
import { type MutationFeedback, MutationToast } from "./MutationToast";
import { SelectedModPanel } from "./SelectedModPanel";
import { SettingsDialog } from "./SettingsDialog";
import { providerLogos } from "./StoreLogos";
import { TagMenu } from "./TagMenu";
import { type UpdateDownloadProgress, UpdateDialog } from "./UpdateDialog";

type LibraryIndex = {
	folders: string[];
	folderEntries: ReadonlyMap<string, discovery.Entry[]>;
	folderEntryCounts: ReadonlyMap<string, number>;
	rootEntryCount: number;
};

type ViewModeButtonProps = {
	active: boolean;
	mode: ViewMode;
	onSelect: (mode: ViewMode) => void;
};

type MutationDialog = "priority" | "rename" | "move" | "delete" | "tags";

type FolderDialogMode = "create" | "rename" | "move" | "delete";

const viewModeIcons = {
	compact: Grid2X2,
	large: PanelsTopLeft,
	list: List,
} satisfies Record<ViewMode, typeof Grid2X2>;

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
		rootEntryCount: (library?.entries ?? []).filter((entry) => entry.relativeFolder === "")
			.length,
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

// Counts tagged mod records whose scanner ID matches nothing in the current
// scan. The record itself is never deleted (see
// internal/metadata/identity.go's OrphanedMods), so this exists to tell the
// user rather than let a tag silently vanish from a card with no
// explanation — for example when a stale metadata backup is restored after
// a mod has already moved on disk.
function orphanedTaggedModCount(
	document: metadata.Document | null,
	liveEntries: readonly discovery.Entry[],
): number {
	if (!document?.mods) return 0;

	const liveScannerIDs = new Set(liveEntries.map((entry) => entry.id));
	let count = 0;
	for (const record of Object.values(document.mods)) {
		if ((record.tags?.length ?? 0) > 0 && !liveScannerIDs.has(record.scannerID)) {
			count++;
		}
	}
	return count;
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
			const scope =
				selectedFolder === "all"
					? "library"
					: selectedFolder === ""
						? "library root"
						: selectedFolder;
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
	const [identitiesByEntryID, setIdentitiesByEntryID] = useState<
		Record<string, modtype.Identity>
	>({});
	const [isClassifying, setIsClassifying] = useState(false);
	const [conflictResult, setConflictResult] = useState<conflict.Result | null>(null);
	const [isCheckingConflicts, setIsCheckingConflicts] = useState(false);
	const [conflictDetailsOpen, setConflictDetailsOpen] = useState(false);
	const [updateCheckResult, setUpdateCheckResult] = useState<main.UpdateCheckResult | null>(null);
	const [isCheckingForUpdate, setIsCheckingForUpdate] = useState(false);
	const [whatsNewResult, setWhatsNewResult] = useState<main.UpdateCheckResult | null>(null);
	const [updateDialogMode, setUpdateDialogMode] = useState<"available" | "installed" | null>(
		null,
	);
	const [isDownloadingUpdate, setIsDownloadingUpdate] = useState(false);
	const [isUpdateReady, setIsUpdateReady] = useState(false);
	const [updateDownloadProgress, setUpdateDownloadProgress] =
		useState<UpdateDownloadProgress | null>(null);
	const [downloadedInstallerPath, setDownloadedInstallerPath] = useState<string | null>(null);
	const [appVersion, setAppVersion] = useState("dev");
	const [libraryProvider, setLibraryProvider] = useState<LibraryProvider>("steam");
	const [isDetectingLibrary, setIsDetectingLibrary] = useState(false);
	const [isCreatingLibrary, setIsCreatingLibrary] = useState(false);
	const [detectionDialog, setDetectionDialog] = useState<{
		mode: "apply" | "create";
		detection: gamedetect.Detection;
	} | null>(null);
	const [draggedItem, setDraggedItem] = useState<DraggedItem | null>(null);
	const [installSource, setInstallSource] = useState<InstallSource | null>(null);
	const [installFromUrlOpen, setInstallFromUrlOpen] = useState(false);
	const [isDraggingExternalFiles, setIsDraggingExternalFiles] = useState(false);
	const externalDragDepthRef = useRef(0);
	const activeLibraryRootRef = useRef<string | null>(null);
	const classificationRequestIDRef = useRef(0);
	const mutatingEntryIDsRef = useRef(new Set<string>());
	const isFolderMutatingRef = useRef(false);
	const nextMutationFeedbackIDRef = useRef(0);
	const hasLoadedInitialMetadataRef = useRef(false);
	const hasCheckedWhatsNewRef = useRef(false);
	const lastOrphanedTagNoticeCountRef = useRef(0);
	const accentColorSaveTimeoutRef = useRef<number | undefined>(undefined);
	const libraryRoot = library?.root;
	const ActiveProviderLogo = providerLogos[libraryProvider];

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
	const conflictedEntryIDs = useMemo(
		() =>
			new Set(
				conflictResult?.groups?.flatMap((group) =>
					(group.participants ?? []).map((participant) => participant.entryID),
				) ?? [],
			),
		[conflictResult],
	);

	const displayedEntries = useMemo(() => {
		const normalizedSearch = search.trim().toLocaleLowerCase();
		const libraryEntries = library?.entries ?? [];
		const scopedEntries =
			selectedFolder === "all"
				? libraryEntries
				: selectedFolder === ""
					? libraryEntries.filter((entry) => entry.relativeFolder === "")
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
	const startInstallFiles = useCallback(async () => {
		try {
			const files = await SelectFilesForInstall();
			if (files && files.length > 0) {
				setInstallSource({ kind: "files", paths: files });
			}
		} catch (error) {
			showMutationFeedback("error", `Could not open file selector: ${errorMessage(error)}`);
		}
	}, [showMutationFeedback]);
	const submitInstallFromUrl = useCallback((url: string) => {
		setInstallFromUrlOpen(false);
		setInstallSource({ kind: "url", url });
	}, []);
	const handleDroppedFiles = useCallback(
		(_x: number, _y: number, paths: string[]) => {
			if (paths.length === 0) return;
			if (!libraryRoot) {
				showMutationFeedback("error", "Set a mod library folder before installing.");
				return;
			}
			setInstallSource({ kind: "files", paths });
		},
		[libraryRoot, showMutationFeedback],
	);

	// Wails resolves real filesystem paths for dropped files outside the DOM's
	// dataTransfer, which never exposes host paths for security reasons.
	useEffect(() => {
		OnFileDrop(handleDroppedFiles, false);
		return () => OnFileDropOff();
	}, [handleDroppedFiles]);

	useEffect(() => {
		return EventsOn("update:downloadProgress", (progress: UpdateDownloadProgress) => {
			setUpdateDownloadProgress(progress);
		});
	}, []);

	// Best-effort welcome notice: a failure here should not surface as an
	// error toast, since there is nothing the user needs to act on.
	useEffect(() => {
		if (hasCheckedWhatsNewRef.current) return;
		hasCheckedWhatsNewRef.current = true;

		void (async () => {
			try {
				const result = await CheckWhatsNew();
				if (result.available) {
					setWhatsNewResult(result);
					setUpdateDialogMode((current) => current ?? "installed");
				}
			} catch {
				// Intentionally silent; see comment above.
			}
		})();

		GetAppVersion()
			.then(setAppVersion)
			.catch(() => {
				// Settings keeps its "dev" default; not worth a toast.
			});
	}, []);

	// Tracks external OS file drags separately from the app's own internal drag-to-organize
	// system (which carries "text/plain" data, not "Files") to show a drop-target overlay.
	// dragenter/dragleave bubble from every element the drag crosses, so a depth counter
	// avoids the overlay flickering as the pointer moves between nested elements.
	useEffect(() => {
		function isExternalFileDrag(event: DragEvent) {
			return Array.from(event.dataTransfer?.types ?? []).includes("Files");
		}

		function handleDragEnter(event: DragEvent) {
			if (!isExternalFileDrag(event)) return;
			event.preventDefault();
			externalDragDepthRef.current += 1;
			setIsDraggingExternalFiles(true);
		}

		function handleDragOver(event: DragEvent) {
			if (!isExternalFileDrag(event)) return;
			event.preventDefault();
		}

		function handleDragLeave(event: DragEvent) {
			if (!isExternalFileDrag(event)) return;
			externalDragDepthRef.current = Math.max(0, externalDragDepthRef.current - 1);
			if (externalDragDepthRef.current === 0) {
				setIsDraggingExternalFiles(false);
			}
		}

		function handleDrop(event: DragEvent) {
			if (!isExternalFileDrag(event)) return;
			event.preventDefault();
			externalDragDepthRef.current = 0;
			setIsDraggingExternalFiles(false);
		}

		window.addEventListener("dragenter", handleDragEnter);
		window.addEventListener("dragover", handleDragOver);
		window.addEventListener("dragleave", handleDragLeave);
		window.addEventListener("drop", handleDrop);
		return () => {
			window.removeEventListener("dragenter", handleDragEnter);
			window.removeEventListener("dragover", handleDragOver);
			window.removeEventListener("dragleave", handleDragLeave);
			window.removeEventListener("drop", handleDrop);
		};
	}, []);

	const classify = useCallback(async (root: string, entries: discovery.Entry[]) => {
		classificationRequestIDRef.current += 1;
		const requestID = classificationRequestIDRef.current;
		if (entries.length === 0) {
			setIsClassifying(false);
			return;
		}
		setIsClassifying(true);
		try {
			const results = await ClassifyLibrary(root, entries);
			if (
				activeLibraryRootRef.current !== root ||
				classificationRequestIDRef.current !== requestID
			) {
				return;
			}
			setIdentitiesByEntryID((prev) => ({ ...prev, ...results }));
		} catch {
			// Non-blocking background classification
		} finally {
			if (classificationRequestIDRef.current === requestID) {
				setIsClassifying(false);
			}
		}
	}, []);
	// User-initiated only: never runs automatically after a scan, so it never
	// surprises someone mid-organize the way a background check would.
	const checkConflicts = useCallback(async () => {
		if (!libraryRoot || !library) return;

		const requestRoot = libraryRoot;
		setIsCheckingConflicts(true);
		try {
			const result = await DetectConflicts(requestRoot, library.entries);
			if (activeLibraryRootRef.current !== requestRoot) return;

			setConflictResult(result);
			if ((result?.groups?.length ?? 0) === 0) {
				showMutationFeedback("success", "No asset conflicts found.");
			} else {
				setConflictDetailsOpen(true);
			}
		} catch (error) {
			if (activeLibraryRootRef.current !== requestRoot) return;
			showMutationFeedback("error", `Could not check for conflicts: ${errorMessage(error)}`);
		} finally {
			if (activeLibraryRootRef.current === requestRoot) {
				setIsCheckingConflicts(false);
			}
		}
	}, [libraryRoot, library, showMutationFeedback]);
	// User-initiated only, same as checkConflicts: nothing polls GitHub in the
	// background unasked.
	// Closes Settings when an update turns up so the update dialog does not
	// stack on top of it -- dialogs in this app assume exclusive focus.
	const checkForUpdate = useCallback(async () => {
		setIsCheckingForUpdate(true);
		try {
			const result = await CheckForUpdate();
			setUpdateCheckResult(result.available ? result : null);
			if (result.available) {
				setSettingsOpen(false);
				setUpdateDialogMode("available");
			} else {
				showMutationFeedback("success", "Cratebug is up to date.");
			}
		} catch (error) {
			showMutationFeedback("error", `Could not check for updates: ${errorMessage(error)}`);
		} finally {
			setIsCheckingForUpdate(false);
		}
	}, [showMutationFeedback]);
	const downloadUpdate = useCallback(async () => {
		if (!updateCheckResult?.release) return;

		setIsDownloadingUpdate(true);
		setUpdateDownloadProgress(null);
		try {
			const path = await DownloadUpdate(updateCheckResult.release);
			setDownloadedInstallerPath(path);
			setIsUpdateReady(true);
		} catch (error) {
			showMutationFeedback("error", `Could not download the update: ${errorMessage(error)}`);
		} finally {
			setIsDownloadingUpdate(false);
		}
	}, [updateCheckResult, showMutationFeedback]);
	// A successful call quits Cratebug from the Go side (see App.ApplyUpdate),
	// so there is deliberately no further state update on success here: the
	// detached helper takes over from this point, not the running frontend.
	const applyUpdate = useCallback(async () => {
		if (!downloadedInstallerPath) return;
		try {
			await ApplyUpdate(downloadedInstallerPath);
		} catch (error) {
			showMutationFeedback("error", `Could not apply the update: ${errorMessage(error)}`);
		}
	}, [downloadedInstallerPath, showMutationFeedback]);
	const closeUpdateDialog = useCallback(() => {
		setUpdateDialogMode(null);
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
			if (result.previousID && result.previousID !== result.id) {
				setIdentitiesByEntryID((prev) => {
					if (!result.previousID) return prev;
					const prevIdentity = prev[result.previousID];
					if (!prevIdentity) return prev;
					const next = { ...prev };
					next[result.id] = prevIdentity;
					delete next[result.previousID];
					return next;
				});
			}
			setConflictResult((currentConflictResult) => {
				if (!currentConflictResult) return currentConflictResult;
				let hasAnyChange = false;
				const updatedGroups = currentConflictResult.groups.map((group) => {
					let groupChanged = false;
					const updatedParticipants = group.participants.map((p) => {
						if (
							(result.previousID && p.entryID === result.previousID) ||
							p.entryID === result.id
						) {
							groupChanged = true;
							hasAnyChange = true;
							return new conflict.Participant({
								...p,
								entryID: result.id,
								displayName: changes.displayName ?? p.displayName,
								priority: changes.priority ?? p.priority,
							});
						}
						return p;
					});
					if (!groupChanged) return group;

					const firstPriority = updatedParticipants[0]?.priority;
					const isSamePriority = updatedParticipants.every(
						(p) =>
							p.priority.kind === firstPriority?.kind &&
							p.priority.value === firstPriority?.value,
					);
					return new conflict.Group({
						...group,
						participants: updatedParticipants,
						relationship: isSamePriority ? "same_priority" : "cross_priority",
					});
				});
				if (!hasAnyChange) return currentConflictResult;
				return new conflict.Result({
					...currentConflictResult,
					groups: updatedGroups,
				});
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
			// The rescanned set of mods may no longer match a conflict report taken
			// before this mutation (a moved, deleted, or newly-installed mod can
			// change what overlaps), so a stale report can't be trusted anymore.
			setConflictResult(null);
			classify(requestRoot, result.entries);
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
	}, [libraryRoot, showMutationFeedback, classify]);

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

	// Switches the store provider auto-detect targets, with the same
	// optimistic-then-revert pattern as the theme so the selector feels
	// instant but never lies if the save fails.
	const selectLibraryProvider = useCallback(
		async (provider: LibraryProvider) => {
			if (provider === libraryProvider) return;

			const previousProvider = libraryProvider;
			setLibraryProvider(provider);
			try {
				await SetLibraryProvider(provider);
			} catch (error) {
				setLibraryProvider(previousProvider);
				showMutationFeedback(
					"error",
					`Could not save the library provider: ${errorMessage(error)}`,
				);
			}
		},
		[libraryProvider, showMutationFeedback],
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
				// Enabled state determines conflict eligibility directly, so a report
				// taken before this toggle can no longer be trusted.
				setConflictResult(null);
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

	const deleteFolder = useCallback(
		async (folder: string): Promise<boolean> => {
			if (!libraryRoot || mutatingEntryIDsRef.current.size > 0 || isFolderMutatingRef.current)
				return false;

			isFolderMutatingRef.current = true;
			setIsFolderMutating(true);
			try {
				await DeleteFolder(libraryRoot, folder, true);
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				await reloadLibrary();
				if (activeLibraryRootRef.current !== libraryRoot) return false;

				// Deletion removes the whole subtree, so a selection inside it is
				// dropped rather than remapped to a path that no longer exists.
				setSelectedFolder((current) =>
					current === folder || current.startsWith(`${folder}/`) ? "all" : current,
				);
				setSelectedEntryID(null);
				showMutationFeedback("success", `Sent folder ${folder} to the Recycle Bin.`);
				return true;
			} catch (error) {
				if (activeLibraryRootRef.current === libraryRoot) {
					showMutationFeedback(
						"error",
						`Could not delete folder: ${errorMessage(error)}`,
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
				{
					label: "Delete folder...",
					onSelect: () => {
						setFolderDialogTarget(folder);
						setActiveFolderDialog("delete");
					},
					destructive: true,
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

	// Runs one auto-detection attempt against the active provider and routes
	// its three-state outcome: a found library applies directly when nothing
	// is configured (or confirms a switch when something is), a missing
	// library offers the one-folder creation, and no installation reports
	// through the regular feedback toast.
	async function detectLibrary() {
		if (isDetectingLibrary || isMutationLocked) return;

		setIsDetectingLibrary(true);
		try {
			const detection = await DetectLibrary(libraryProvider);
			const outcome = detectionOutcome(detection, modRoot);
			switch (outcome.kind) {
				case "apply":
					setDetectionDialog({ mode: "apply", detection });
					break;
				case "create":
					setDetectionDialog({ mode: "create", detection });
					break;
				case "same-library": {
					const libraryPath = detection.libraryPath ?? "";
					setModRoot(libraryPath);
					await scan(libraryPath);
					showMutationFeedback("success", "This library is already active.");
					break;
				}
				case "not-found":
					showMutationFeedback(
						"error",
						`Could not find a Marvel Rivals installation in your ${libraryProviderLabels[libraryProvider]} libraries.`,
					);
					break;
			}
		} catch (error) {
			showMutationFeedback("error", `Could not detect a mod library: ${errorMessage(error)}`);
		} finally {
			setIsDetectingLibrary(false);
		}
	}

	// Applies a detected library through the normal scan flow, which also
	// persists it as the mod root.
	async function applyDetectedLibrary(libraryPath: string) {
		setDetectionDialog(null);
		setModRoot(libraryPath);
		await scan(libraryPath);
	}

	// Creates the missing library folder behind the user's confirmation.
	// Cratebug re-detects on the Go side, so this can only ever create the
	// folder of an installation the provider just verified.
	async function createDetectedLibrary() {
		setIsCreatingLibrary(true);
		try {
			const libraryPath = await CreateLibrary(libraryProvider);
			setDetectionDialog(null);
			setModRoot(libraryPath);
			await scan(libraryPath);
			showMutationFeedback("success", "Mod library folder created.");
		} catch (error) {
			setDetectionDialog(null);
			showMutationFeedback(
				"error",
				`Could not create the mod library folder: ${errorMessage(error)}`,
			);
		} finally {
			setIsCreatingLibrary(false);
		}
	}

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
			setIdentitiesByEntryID({});
			setConflictResult(null);
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
			setIdentitiesByEntryID({});
			setConflictResult(null);
			classify(root, result.entries);
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
				const persistedProvider = state.document.settings.libraryProvider;
				if (
					persistedProvider &&
					libraryProviders.includes(persistedProvider as LibraryProvider)
				) {
					setLibraryProvider(persistedProvider as LibraryProvider);
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

	// Surfaces orphaned tag records once the scan and the metadata that might
	// reference it are both loaded. Runs on every library or metadata change
	// (not just the initial load), since a scan can outlive a metadata
	// recovery or vice versa; the ref only reshows the warning when the count
	// actually changes, so routine metadata reloads (assigning a tag, for
	// example) do not repeat it.
	useEffect(() => {
		if (!library) {
			lastOrphanedTagNoticeCountRef.current = 0;
			return;
		}

		const count = orphanedTaggedModCount(metadataDocument, library.entries);
		if (count === lastOrphanedTagNoticeCountRef.current) return;
		lastOrphanedTagNoticeCountRef.current = count;
		if (count === 0) return;

		showMutationFeedback(
			"warning",
			count === 1
				? "1 tagged mod record no longer matches anything in this scan. Its tags are kept in case the mod reappears."
				: `${count} tagged mod records no longer match anything in this scan. Their tags are kept in case the mods reappear.`,
		);
	}, [library, metadataDocument, showMutationFeedback]);

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
				<div className={styles.brand}>
					<div className={styles["brand-mark"]} aria-hidden="true">
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
						<p className={styles["brand-kicker"]}>Marvel Rivals mod manager</p>
						<h1 className={styles.wordmark}>Cratebug</h1>
					</div>
				</div>
				<div className={styles["header-controls"]}>
					<button
						type="button"
						className="icon-button"
						onClick={() => void startInstallFiles()}
						disabled={!libraryRoot || libraryState === "loading"}
						aria-label="Install mod"
						title="Install mod from archive or pak file"
					>
						<PackagePlus aria-hidden="true" />
					</button>
					<button
						type="button"
						className="icon-button"
						onClick={() => setInstallFromUrlOpen(true)}
						disabled={!libraryRoot || libraryState === "loading"}
						aria-label="Install from URL"
						title="Download and install a mod from a direct URL"
					>
						<Link aria-hidden="true" />
					</button>
					<button
						type="button"
						className="icon-button"
						onClick={() => {
							if ((conflictResult?.groups?.length ?? 0) > 0) {
								setConflictDetailsOpen(true);
							} else {
								void checkConflicts();
							}
						}}
						disabled={!libraryRoot || libraryState === "loading" || isCheckingConflicts}
						aria-label={
							(conflictResult?.groups?.length ?? 0) > 0
								? "View conflict details"
								: "Check for conflicts"
						}
						title={
							(conflictResult?.groups?.length ?? 0) > 0
								? "View conflict details"
								: "Scan enabled mods for overlapping asset paths"
						}
					>
						<ShieldAlert aria-hidden="true" />
					</button>
					<button
						type="button"
						className={
							updateCheckResult?.available ? "icon-button has-update" : "icon-button"
						}
						onClick={() => setSettingsOpen(true)}
						aria-label={
							updateCheckResult?.available
								? "Settings (update available)"
								: "Settings"
						}
						title={
							updateCheckResult?.available
								? "Settings -- a Cratebug update is available"
								: "Settings"
						}
					>
						<SettingsIcon aria-hidden="true" />
					</button>
				</div>
			</header>

			<section className="library-toolbar" aria-label="Library scan controls">
				<form
					className={styles["root-form"]}
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
				<button
					type="button"
					className="quiet-button detect-button"
					onClick={() => void detectLibrary()}
					disabled={isDetectingLibrary || isMutationLocked}
					title={`Automatically detect the Marvel Rivals mod library in your ${libraryProviderLabels[libraryProvider]} installation`}
				>
					<ActiveProviderLogo className="detect-button-logo" />
					{isDetectingLibrary
						? "Detecting..."
						: `Detect ${libraryProviderLabels[libraryProvider]} library`}
				</button>
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
				<aside
					className={[styles["library-sidebar"], "scroll-y"].join(" ")}
					aria-label="Library folders"
				>
					<div className={styles["sidebar-heading"]}>
						<span>Folders</span>
						{library && (
							<div className={styles["sidebar-heading-actions"]}>
								<button
									type="button"
									className={[
										"quiet-button",
										styles["sidebar-heading-action"],
									].join(" ")}
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
						rootEntryCount={libraryIndex.rootEntryCount}
						folderEntryCounts={libraryIndex.folderEntryCounts}
					/>
				</aside>

				<section className={styles["catalog-panel"]} aria-label="Discovered mods">
					<div className={styles["catalog-header"]}>
						<div>
							<p className="eyebrow">Mod library</p>
							<h2>
								{selectedFolder === "all"
									? "All mods"
									: selectedFolder === ""
										? "Library root"
										: selectedFolder}
							</h2>
						</div>
						<div className={styles["catalog-controls"]}>
							<label className={styles["search-control"]}>
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
						<fieldset className={styles["view-mode-controls"]}>
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
						identity={selectedEntry ? identitiesByEntryID[selectedEntry.id] : undefined}
						isClassifying={isClassifying}
						assignedTags={assignedTagsForSelection}
						isMutating={selectedEntry ? mutatingEntryIDs.has(selectedEntry.id) : false}
						isMutationLocked={isMutationLocked}
						onClear={() => {
							setSelectedEntryID(null);
							setActiveDialog(null);
						}}
						onSetEnabled={setModEnabled}
						onDelete={() => setActiveDialog("delete")}
					/>
					<ModCatalog
						entries={displayedEntries}
						state={libraryState}
						scanError={scanError}
						hasLibrary={library !== null}
						initialStateAction={
							<button
								type="button"
								className="quiet-button detect-button"
								onClick={() => void detectLibrary()}
								disabled={isDetectingLibrary || isMutationLocked}
								title={`Automatically detect the Marvel Rivals mod library in your ${libraryProviderLabels[libraryProvider]} installation`}
							>
								<ActiveProviderLogo className="detect-button-logo" />
								{isDetectingLibrary
									? "Detecting..."
									: `Detect ${libraryProviderLabels[libraryProvider]} library`}
							</button>
						}
						mutatingEntryIDs={mutatingEntryIDs}
						isMutationLocked={isMutationLocked}
						tagsByEntryID={entryTags}
						identitiesByEntryID={identitiesByEntryID}
						isClassifying={isClassifying}
						conflictedEntryIDs={conflictedEntryIDs}
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
			{activeFolderDialog && activeFolderDialog !== "delete" && library && (
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
			{activeFolderDialog === "delete" && library && (
				<FolderDeleteConfirmDialog
					folder={folderDialogTarget}
					libraryRoot={libraryRoot ?? ""}
					isMutating={isMutationLocked}
					key={folderDialogTarget}
					onClose={() => setActiveFolderDialog(null)}
					onConfirm={deleteFolder}
				/>
			)}
			{contextMenu && (
				<ContextMenu state={contextMenu} onClose={() => setContextMenu(null)} />
			)}
			{settingsOpen && (
				<SettingsDialog
					theme={theme}
					accentColor={accentColor}
					appVersion={appVersion}
					isCheckingForUpdate={isCheckingForUpdate}
					libraryProvider={libraryProvider}
					onClose={() => setSettingsOpen(false)}
					onSelectTheme={selectTheme}
					onSelectAccentColor={selectAccentColor}
					onSelectLibraryProvider={(provider) => void selectLibraryProvider(provider)}
					onCheckForUpdate={() => void checkForUpdate()}
				/>
			)}
			{detectionDialog && (
				<DetectLibraryDialog
					provider={libraryProvider}
					mode={detectionDialog.mode}
					detection={detectionDialog.detection}
					isWorking={isCreatingLibrary}
					onApply={() =>
						void applyDetectedLibrary(detectionDialog.detection.libraryPath ?? "")
					}
					onCreate={() => void createDetectedLibrary()}
					onClose={() => setDetectionDialog(null)}
				/>
			)}
			{conflictDetailsOpen && conflictResult && library && (
				<ConflictDetailsDialog
					result={conflictResult}
					entries={library.entries}
					identitiesByEntryID={identitiesByEntryID}
					isMutationLocked={isMutationLocked}
					onClose={() => setConflictDetailsOpen(false)}
					onSetPriority={setModPriority}
				/>
			)}
			{updateDialogMode &&
				(() => {
					const release =
						updateDialogMode === "available"
							? updateCheckResult?.release
							: whatsNewResult?.release;
					if (!release) return null;
					return (
						<UpdateDialog
							release={release}
							mode={updateDialogMode}
							isDownloading={isDownloadingUpdate}
							isReady={isUpdateReady}
							downloadProgress={updateDownloadProgress}
							onDownload={() => void downloadUpdate()}
							onApply={() => void applyUpdate()}
							onClose={closeUpdateDialog}
						/>
					);
				})()}
			{installFromUrlOpen && (
				<InstallFromUrlDialog
					onSubmit={submitInstallFromUrl}
					onCancel={() => setInstallFromUrlOpen(false)}
				/>
			)}
			{installSource && libraryRoot && (
				<InstallPreviewDialog
					modRoot={libraryRoot}
					source={installSource}
					defaultFolder={selectedFolder === "all" ? "" : selectedFolder}
					folders={libraryIndex.folders}
					libraryEntries={library.entries}
					onDone={(result) => {
						setInstallSource(null);
						setLibrary(result.reconciledLibrary);
						setLibraryState(
							result.reconciledLibrary.entries.length === 0 ? "empty" : "populated",
						);
						classify(libraryRoot, result.reconciledLibrary.entries);
						showMutationFeedback(
							"success",
							result.installedEntryIDs.length === 1
								? "Installed 1 mod."
								: `Installed ${result.installedEntryIDs.length} mods.`,
						);
						const firstInstalledID = result.installedEntryIDs[0];
						if (firstInstalledID) {
							setSelectedEntryID(firstInstalledID);
						}
					}}
					onCancel={() => setInstallSource(null)}
				/>
			)}
			{isDraggingExternalFiles &&
				!activeDialog &&
				!activeFolderDialog &&
				!settingsOpen &&
				!conflictDetailsOpen &&
				!updateDialogMode &&
				!installFromUrlOpen &&
				!detectionDialog &&
				!installSource && (
					<div className={styles["drop-overlay"]} aria-hidden="true">
						<div className={styles["drop-overlay-card"]}>
							<PackagePlus aria-hidden="true" />
							<p>Drop mod files or archives to install</p>
						</div>
					</div>
				)}
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
			className={active ? styles.selected : undefined}
			onClick={() => onSelect(mode)}
			title={`${label} view`}
		>
			<span className="visually-hidden">{label} view</span>
			<Icon aria-hidden="true" />
		</button>
	);
}
