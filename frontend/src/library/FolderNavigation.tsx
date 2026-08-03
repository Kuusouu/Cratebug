import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronRight, Folder, LibraryBig } from "lucide-react";

type FolderNode = {
	path: string;
	name: string;
	children: FolderNode[];
};

type FolderNavigationProps = {
	folders: string[];
	selectedFolder: string;
	onSelect: (folder: string) => void;
	entryCount: number;
	folderEntryCounts: ReadonlyMap<string, number>;
};

type TooltipState = {
	content: string;
	container: HTMLElement;
	x: number;
	y: number;
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
		if (!node) continue;

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
	entryCount,
	folderEntryCounts,
}: FolderNavigationProps) {
	const tree = useMemo(() => buildFolderTree(folders), [folders]);
	const [expandedFolders, setExpandedFolders] = useState<Set<string>>(() => new Set());
	const [tooltip, setTooltip] = useState<TooltipState | null>(null);

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

	const showTooltip = useCallback((content: string, target: HTMLElement) => {
		const container = target.closest<HTMLElement>(".app-shell");
		if (!container) return;

		const { right, top, height } = target.getBoundingClientRect();
		setTooltip({ content, container, x: right, y: top + height / 2 });
	}, []);

	const hideTooltip = useCallback(() => setTooltip(null), []);

	// Fixed-position tooltips must disappear when their source row moves.
	useEffect(() => {
		if (!tooltip) return;

		window.addEventListener("scroll", hideTooltip, true);
		window.addEventListener("resize", hideTooltip);
		return () => {
			window.removeEventListener("scroll", hideTooltip, true);
			window.removeEventListener("resize", hideTooltip);
		};
	}, [hideTooltip, tooltip]);

	return (
		<nav className="folder-navigation">
			<button
				type="button"
				className={`folder-row${selectedFolder === "all" ? " selected" : ""}`}
				onClick={() => onSelect("all")}
			>
				<LibraryBig aria-hidden="true" className="folder-icon" />
				<span className="folder-name">All mods</span>
				<span className="folder-count">{entryCount}</span>
			</button>
			{tree.map((node) => (
				<FolderTreeItem
					entryCounts={folderEntryCounts}
					expandedFolders={expandedFolders}
					key={node.path}
					node={node}
					onSelect={onSelect}
					onHideTooltip={hideTooltip}
					onShowTooltip={showTooltip}
					onToggle={toggleFolder}
					selectedFolder={selectedFolder}
				/>
			))}
			{tooltip &&
				createPortal(
					<div
						aria-hidden="true"
						className="app-tooltip"
						style={{ left: tooltip.x, top: tooltip.y }}
					>
						{tooltip.content}
					</div>,
					tooltip.container,
				)}
		</nav>
	);
});

type FolderTreeItemProps = {
	node: FolderNode;
	selectedFolder: string;
	expandedFolders: ReadonlySet<string>;
	entryCounts: ReadonlyMap<string, number>;
	onSelect: (folder: string) => void;
	onHideTooltip: () => void;
	onShowTooltip: (content: string, target: HTMLElement) => void;
	onToggle: (folder: string) => void;
};

// Identifies the small part of the tree that must update on selection.
function selectionTouchesBranch(selectedFolder: string, folderPath: string): boolean {
	return selectedFolder === folderPath || selectedFolder.startsWith(`${folderPath}/`);
}

// Limits selection updates to the old and new folder branches.
function sameFolderTreeItemProps(
	previous: FolderTreeItemProps,
	next: FolderTreeItemProps,
): boolean {
	if (
		previous.node !== next.node ||
		previous.expandedFolders !== next.expandedFolders ||
		previous.entryCounts !== next.entryCounts ||
		previous.onSelect !== next.onSelect ||
		previous.onHideTooltip !== next.onHideTooltip ||
		previous.onShowTooltip !== next.onShowTooltip ||
		previous.onToggle !== next.onToggle
	)
		return false;

	if (previous.selectedFolder === next.selectedFolder) return true;

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
	onSelect,
	onHideTooltip,
	onShowTooltip,
	onToggle,
}: FolderTreeItemProps) {
	const hasChildren = node.children.length > 0;
	const expanded = expandedFolders.has(node.path);

	return (
		<div className="folder-tree-item">
			<div className={`folder-row${selectedFolder === node.path ? " selected" : ""}`}>
				{hasChildren ? (
					<button
						aria-expanded={expanded}
						aria-label={`${expanded ? "Collapse" : "Expand"} ${node.name}`}
						className="folder-toggle"
						type="button"
						onClick={() => onToggle(node.path)}
					>
						<ChevronRight
							aria-hidden="true"
							className={`chevron-icon${expanded ? " expanded" : ""}`}
						/>
					</button>
				) : (
					<span className="folder-toggle-placeholder" aria-hidden="true" />
				)}
				<button
					className="folder-select"
					type="button"
					onBlur={onHideTooltip}
					onFocus={(event) => onShowTooltip(node.name, event.currentTarget)}
					onClick={() => onSelect(node.path)}
					onMouseEnter={(event) => onShowTooltip(node.name, event.currentTarget)}
					onMouseLeave={onHideTooltip}
				>
					<Folder aria-hidden="true" className="folder-icon" />
					<span className="folder-name">{node.name}</span>
					<span className="folder-count">{entryCounts.get(node.path) ?? 0}</span>
				</button>
			</div>
			{hasChildren && expanded && (
				<div className="folder-children">
					{node.children.map((child) => (
						<FolderTreeItem
							entryCounts={entryCounts}
							expandedFolders={expandedFolders}
							key={child.path}
							node={child}
							onSelect={onSelect}
							onHideTooltip={onHideTooltip}
							onShowTooltip={onShowTooltip}
							onToggle={onToggle}
							selectedFolder={selectedFolder}
						/>
					))}
				</div>
			)}
		</div>
	);
}, sameFolderTreeItemProps);
