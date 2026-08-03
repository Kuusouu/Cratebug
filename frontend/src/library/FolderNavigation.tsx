import { useMemo, useState } from "react";
import { AllModsIcon, ChevronIcon, FolderIcon } from "./LibraryIcons";

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

// FolderNavigation renders the physical directory hierarchy reported by discovery.
export function FolderNavigation({
	folders,
	selectedFolder,
	onSelect,
	entryCount,
	folderEntryCounts,
}: FolderNavigationProps) {
	const tree = useMemo(() => buildFolderTree(folders), [folders]);
	const [expandedFolders, setExpandedFolders] = useState<Set<string>>(() => new Set());

	function toggleFolder(path: string) {
		setExpandedFolders((current) => {
			const next = new Set(current);
			if (next.has(path)) {
				next.delete(path);
			} else {
				next.add(path);
			}
			return next;
		});
	}

	return (
		<nav className="folder-navigation">
			<button
				type="button"
				className={`folder-row${selectedFolder === "all" ? " selected" : ""}`}
				onClick={() => onSelect("all")}
			>
				<AllModsIcon className="folder-icon" />
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
					onToggle={toggleFolder}
					selectedFolder={selectedFolder}
				/>
			))}
		</nav>
	);
}

function FolderTreeItem({
	node,
	selectedFolder,
	expandedFolders,
	entryCounts,
	onSelect,
	onToggle,
}: {
	node: FolderNode;
	selectedFolder: string;
	expandedFolders: ReadonlySet<string>;
	entryCounts: ReadonlyMap<string, number>;
	onSelect: (folder: string) => void;
	onToggle: (folder: string) => void;
}) {
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
						<ChevronIcon className="chevron-icon" expanded={expanded} />
					</button>
				) : (
					<span className="folder-toggle-placeholder" aria-hidden="true" />
				)}
				<button className="folder-select" type="button" onClick={() => onSelect(node.path)}>
					<FolderIcon className="folder-icon" />
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
							onToggle={onToggle}
							selectedFolder={selectedFolder}
						/>
					))}
				</div>
			)}
		</div>
	);
}
