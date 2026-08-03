type FolderNavigationProps = {
	folders: string[];
	selectedFolder: string;
	onSelect: (folder: string) => void;
	entryCount: number;
};

// FolderNavigation renders the physical directory hierarchy reported by discovery.
export function FolderNavigation({
	folders,
	selectedFolder,
	onSelect,
	entryCount,
}: FolderNavigationProps) {
	return (
		<nav className="folder-navigation">
			<button
				type="button"
				className={selectedFolder === "all" ? "selected" : ""}
				onClick={() => onSelect("all")}
			>
				<span>All mods</span>
				<span>{entryCount}</span>
			</button>
			{folders.map((folder) => (
				<button
					key={folder}
					type="button"
					className={selectedFolder === folder ? "selected" : ""}
					onClick={() => onSelect(folder)}
					style={{ paddingLeft: `${1 + folder.split("/").length * 0.75}rem` }}
				>
					<span>{folder.split("/").at(-1)}</span>
				</button>
			))}
		</nav>
	);
}
