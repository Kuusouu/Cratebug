import { ChevronRight, Folder, FolderRoot, LibraryBig } from "lucide-react";
import { memo, type MouseEvent, useCallback, useMemo, useState } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

import { type DraggedItem, isValidDropTarget } from "./libraryTypes";

import styles from "./FolderNavigation.module.css";

type FolderNode = {
	path: string;
	name: string;
	children: FolderNode[];
};

type FolderNavigationProps = {
	folders: string[];
	selectedFolder: string;
	onSelect: (folder: string) => void;
	onContextMenu: (folder: string, event: MouseEvent) => void;
	draggedItem: DraggedItem | null;
	dragDisabled: boolean;
	onDragStartFolder: (path: string) => void;
	onDragEnd: () => void;
	onDropOnFolder: (destinationFolder: string) => void;
	entryCount: number;
	rootEntryCount: number;
	folderEntryCounts: ReadonlyMap<string, number>;
};

type FolderTreeItemProps = {
	node: FolderNode;
	selectedFolder: string;
	expandedFolders: ReadonlySet<string>;
	entryCounts: ReadonlyMap<string, number>;
	draggedItem: DraggedItem | null;
	dragOverFolder: string | null;
	dragDisabled: boolean;
	onSelect: (folder: string) => void;
	onContextMenu: (folder: string, event: MouseEvent) => void;
	onToggle: (folder: string) => void;
	onDragStartFolder: (path: string) => void;
	onDragEnd: () => void;
	onDragOverTarget: (folder: string) => void;
	onDropOnFolder: (folder: string) => void;
};

// Turns flat relative paths into the expandable sidebar hierarchy.
function buildFolderTree(folders: string[]): FolderNode[] {
	const nodes = new Map<string, FolderNode>();
	const roots: FolderNode[] = [];

	for (const path of folders) {
		const name = path.split("/").at(-1) ?? path;
		nodes.set(path, { path, name, children: [] });
	}

	for (const path of folders) {
		const node = nodes.get(path);
		if (!node) {
			continue;
		}

		const parentPath = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";
		const parent = nodes.get(parentPath);
		if (parent) {
			parent.children.push(node);
		} else {
			roots.push(node);
		}
	}

	return roots;
}

// Renders the physical directory hierarchy reported by discovery.
export const FolderNavigation = memo(function FolderNavigation({
	folders,
	selectedFolder,
	onSelect,
	onContextMenu,
	draggedItem,
	dragDisabled,
	onDragStartFolder,
	onDragEnd,
	onDropOnFolder,
	entryCount,
	rootEntryCount,
	folderEntryCounts,
}: FolderNavigationProps) {
	const tree = useMemo(() => buildFolderTree(folders), [folders]);
	const [expandedFolders, setExpandedFolders] = useState<Set<string>>(() => new Set());
	const [dragOverFolder, setDragOverFolder] = useState<string | null>(null);

	const handleDragOverTarget = useCallback((folder: string) => {
		setDragOverFolder((current) => (current === folder ? current : folder));
	}, []);

	const handleDragEnd = useCallback(() => {
		setDragOverFolder(null);
		onDragEnd();
	}, [onDragEnd]);

	const handleDrop = useCallback(
		(destinationFolder: string) => {
			setDragOverFolder(null);
			onDropOnFolder(destinationFolder);
		},
		[onDropOnFolder],
	);

	const toggleFolder = useCallback((path: string) => {
		setExpandedFolders((current) => {
			const next = new Set(current);
			if (next.has(path)) {
				next.delete(path);
			} else {
				next.add(path);
			}
			return next;
		});
	}, []);

	return (
		<nav className={styles["folder-navigation"]}>
			{/* Both rows drop onto the root folder, but they track drag-over under
			    different keys ("all" vs "") so only the hovered row highlights. */}
			<button
				type="button"
				className={[
					styles["folder-row"],
					selectedFolder === "all" ? styles.selected : "",
					isValidDropTarget(draggedItem, "") && dragOverFolder === "all"
						? styles["drag-over"]
						: "",
				]
					.filter(Boolean)
					.join(" ")}
				onClick={() => onSelect("all")}
				onContextMenu={(event) => {
					event.preventDefault();
					onContextMenu("", event);
				}}
				onDragOver={(event) => {
					if (!isValidDropTarget(draggedItem, "")) return;
					event.preventDefault();
					handleDragOverTarget("all");
				}}
				onDrop={(event) => {
					if (!isValidDropTarget(draggedItem, "")) return;
					event.preventDefault();
					handleDrop("");
				}}
			>
				<LibraryBig aria-hidden="true" className={styles["folder-icon"]} />
				<span className={styles["folder-name"]}>All mods</span>
				<span className={styles["folder-count"]}>{entryCount}</span>
			</button>
			<button
				type="button"
				className={[
					styles["folder-row"],
					selectedFolder === "" ? styles.selected : "",
					isValidDropTarget(draggedItem, "") && dragOverFolder === ""
						? styles["drag-over"]
						: "",
				]
					.filter(Boolean)
					.join(" ")}
				onClick={() => onSelect("")}
				onContextMenu={(event) => {
					event.preventDefault();
					onContextMenu("", event);
				}}
				onDragOver={(event) => {
					if (!isValidDropTarget(draggedItem, "")) return;
					event.preventDefault();
					handleDragOverTarget("");
				}}
				onDrop={(event) => {
					if (!isValidDropTarget(draggedItem, "")) return;
					event.preventDefault();
					handleDrop("");
				}}
			>
				<FolderRoot aria-hidden="true" className={styles["folder-icon"]} />
				<span className={styles["folder-name"]}>Library root</span>
				<span className={styles["folder-count"]}>{rootEntryCount}</span>
			</button>
			{tree.map((node) => (
				<FolderTreeItem
					entryCounts={folderEntryCounts}
					expandedFolders={expandedFolders}
					key={node.path}
					node={node}
					draggedItem={draggedItem}
					dragOverFolder={dragOverFolder}
					dragDisabled={dragDisabled}
					onSelect={onSelect}
					onContextMenu={onContextMenu}
					onToggle={toggleFolder}
					onDragStartFolder={onDragStartFolder}
					onDragEnd={handleDragEnd}
					onDragOverTarget={handleDragOverTarget}
					onDropOnFolder={handleDrop}
					selectedFolder={selectedFolder}
				/>
			))}
		</nav>
	);
});
// Identifies the small part of the tree that must update on selection.
function selectionTouchesBranch(selectedFolder: string, folderPath: string): boolean {
	return selectedFolder === folderPath || selectedFolder.startsWith(`${folderPath}/`);
}

function withinSubtree(path: string | null, root: string): boolean {
	return path !== null && (path === root || path.startsWith(`${root}/`));
}

// A FolderTreeItem's memo check must ask "did anything in MY SUBTREE change",
// not just "did my own row change" — it also renders its own children, so a
// "no" here freezes them too. A child-only change (e.g. the drag-over target
// moved to a grandchild) must still return true for every ancestor on the
// path down to it, or React never re-renders far enough for that grandchild
// to receive its new props. This is the same cascade selectionTouchesBranch
// already uses for the selected-folder highlight; checking only the exact
// node's own path (this function's first version) silently broke drag-over
// highlighting and drop-target validity for every subfolder.
function hasRelevantDragStateChanged(
	previousItem: DraggedItem | null,
	nextItem: DraggedItem | null,
	previousDragOver: string | null,
	nextDragOver: string | null,
	subtreeRoot: string,
): boolean {
	const previousDraggedPath = previousItem?.type === "folder" ? previousItem.path : null;
	const nextDraggedPath = nextItem?.type === "folder" ? nextItem.path : null;
	const draggedFolderInSubtreeBefore = withinSubtree(previousDraggedPath, subtreeRoot)
		? previousDraggedPath
		: null;
	const draggedFolderInSubtreeAfter = withinSubtree(nextDraggedPath, subtreeRoot)
		? nextDraggedPath
		: null;
	if (draggedFolderInSubtreeBefore !== draggedFolderInSubtreeAfter) return true;

	const dragOverInSubtreeBefore = withinSubtree(previousDragOver, subtreeRoot)
		? previousDragOver
		: null;
	const dragOverInSubtreeAfter = withinSubtree(nextDragOver, subtreeRoot) ? nextDragOver : null;
	return dragOverInSubtreeBefore !== dragOverInSubtreeAfter;
}

// Limits selection and drag-state updates to the branches they actually touch.
function sameFolderTreeItemProps(
	previous: FolderTreeItemProps,
	next: FolderTreeItemProps,
): boolean {
	if (
		previous.node !== next.node ||
		previous.expandedFolders !== next.expandedFolders ||
		previous.entryCounts !== next.entryCounts ||
		previous.dragDisabled !== next.dragDisabled ||
		previous.onSelect !== next.onSelect ||
		previous.onContextMenu !== next.onContextMenu ||
		previous.onToggle !== next.onToggle ||
		previous.onDragStartFolder !== next.onDragStartFolder ||
		previous.onDragEnd !== next.onDragEnd ||
		previous.onDragOverTarget !== next.onDragOverTarget ||
		previous.onDropOnFolder !== next.onDropOnFolder
	)
		return false;

	if (
		hasRelevantDragStateChanged(
			previous.draggedItem,
			next.draggedItem,
			previous.dragOverFolder,
			next.dragOverFolder,
			next.node.path,
		)
	) {
		return false;
	}

	if (previous.selectedFolder === next.selectedFolder) {
		return true;
	}

	return (
		!selectionTouchesBranch(previous.selectedFolder, previous.node.path) &&
		!selectionTouchesBranch(next.selectedFolder, next.node.path)
	);
}

// Renders one expandable folder branch while preserving memoized siblings.
const FolderTreeItem = memo(function FolderTreeItem({
	node,
	selectedFolder,
	expandedFolders,
	entryCounts,
	draggedItem,
	dragOverFolder,
	dragDisabled,
	onSelect,
	onContextMenu,
	onToggle,
	onDragStartFolder,
	onDragEnd,
	onDragOverTarget,
	onDropOnFolder,
}: FolderTreeItemProps) {
	const hasChildren = node.children.length > 0;
	const expanded = expandedFolders.has(node.path);
	const isDragging = draggedItem?.type === "folder" && draggedItem.path === node.path;
	const canDropHere = isValidDropTarget(draggedItem, node.path);
	const isDragOver = canDropHere && dragOverFolder === node.path;

	return (
		<div className={styles["folder-tree-item"]}>
			<div
				className={[
					styles["folder-row"],
					selectedFolder === node.path ? styles.selected : "",
					isDragging ? styles.dragging : "",
					isDragOver ? styles["drag-over"] : "",
				]
					.filter(Boolean)
					.join(" ")}
			>
				{hasChildren ? (
					<button
						aria-expanded={expanded}
						aria-label={`${expanded ? "Collapse" : "Expand"} ${node.name}`}
						className={styles["folder-toggle"]}
						type="button"
						onClick={() => onToggle(node.path)}
					>
						<ChevronRight
							aria-hidden="true"
							className={`chevron-icon${expanded ? " expanded" : ""}`}
						/>
					</button>
				) : (
					<span className={styles["folder-toggle-placeholder"]} aria-hidden="true" />
				)}
				{/* Row clicks select the folder, so Base UI's default dismiss-on-click
				    would change when this tooltip is visible; keep hover/focus-only. */}
				<Tooltip>
					<TooltipTrigger
						closeOnClick={false}
						render={
							<button
								className={styles["folder-select"]}
								type="button"
								draggable={!dragDisabled}
								onClick={() => onSelect(node.path)}
								onContextMenu={(event) => {
									event.preventDefault();
									onContextMenu(node.path, event);
								}}
								onDragStart={(event) => {
									event.dataTransfer.effectAllowed = "move";
									event.dataTransfer.setData("text/plain", node.path);
									onDragStartFolder(node.path);
								}}
								onDragEnd={onDragEnd}
								onDragOver={(event) => {
									if (!canDropHere) return;
									event.preventDefault();
									event.dataTransfer.dropEffect = "move";
									onDragOverTarget(node.path);
								}}
								onDrop={(event) => {
									if (!canDropHere) return;
									event.preventDefault();
									onDropOnFolder(node.path);
								}}
							>
								<Folder aria-hidden="true" className={styles["folder-icon"]} />
								<span className={styles["folder-name"]}>{node.name}</span>
								<span className={styles["folder-count"]}>
									{entryCounts.get(node.path) ?? 0}
								</span>
							</button>
						}
					/>
					<TooltipContent side="right" align="center">
						{node.name}
					</TooltipContent>
				</Tooltip>
			</div>
			{hasChildren && expanded && (
				<div className={styles["folder-children"]}>
					{node.children.map((child) => (
						<FolderTreeItem
							entryCounts={entryCounts}
							expandedFolders={expandedFolders}
							key={child.path}
							node={child}
							draggedItem={draggedItem}
							dragOverFolder={dragOverFolder}
							dragDisabled={dragDisabled}
							onSelect={onSelect}
							onContextMenu={onContextMenu}
							onToggle={onToggle}
							onDragStartFolder={onDragStartFolder}
							onDragEnd={onDragEnd}
							onDragOverTarget={onDragOverTarget}
							onDropOnFolder={onDropOnFolder}
							selectedFolder={selectedFolder}
						/>
					))}
				</div>
			)}
		</div>
	);
}, sameFolderTreeItemProps);
