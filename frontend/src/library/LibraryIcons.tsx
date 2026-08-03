import type { ViewMode } from "./libraryTypes";

type IconProps = {
	className?: string;
};

export function AllModsIcon({ className }: IconProps) {
	return (
		<svg aria-hidden="true" className={className} viewBox="0 0 24 24">
			<path d="M5 4.5 8.5 3l3 14.5L8 19Zm5.8-.9L14.3 3l3.5 14.5-3.5 1.5Zm5.8-.6L20 4.5 23.5 19 20 20.5Z" />
		</svg>
	);
}

export function ChevronIcon({ className, expanded }: IconProps & { expanded: boolean }) {
	return (
		<svg
			aria-hidden="true"
			className={className}
			viewBox="0 0 24 24"
			style={{ transform: expanded ? "rotate(90deg)" : undefined }}
		>
			<path d="m9 5 7 7-7 7" />
		</svg>
	);
}

export function FolderIcon({ className }: IconProps) {
	return (
		<svg aria-hidden="true" className={className} viewBox="0 0 24 24">
			<path d="M3 6.5h6l1.7 2H21v9.75c0 .97-.78 1.75-1.75 1.75H4.75A1.75 1.75 0 0 1 3 18.25Z" />
		</svg>
	);
}

export function ViewModeIcon({ mode }: { mode: ViewMode }) {
	if (mode === "compact") {
		return (
			<svg aria-hidden="true" viewBox="0 0 24 24">
				<rect height="6" width="6" x="3" y="3" />
				<rect height="6" width="6" x="15" y="3" />
				<rect height="6" width="6" x="3" y="15" />
				<rect height="6" width="6" x="15" y="15" />
			</svg>
		);
	}

	if (mode === "large") {
		return (
			<svg aria-hidden="true" viewBox="0 0 24 24">
				<rect height="7" width="7" x="2" y="2" />
				<rect height="7" width="7" x="15" y="2" />
				<rect height="7" width="7" x="2" y="15" />
				<rect height="7" width="7" x="15" y="15" />
			</svg>
		);
	}

	return (
		<svg aria-hidden="true" viewBox="0 0 24 24">
			<rect height="3" width="5" x="3" y="4" />
			<rect height="3" width="11" x="10" y="4" />
			<rect height="3" width="5" x="3" y="10.5" />
			<rect height="3" width="11" x="10" y="10.5" />
			<rect height="3" width="5" x="3" y="17" />
			<rect height="3" width="11" x="10" y="17" />
		</svg>
	);
}
