import { Check, ChevronDown, ListFilter } from "lucide-react";
import styles from "./FilterMenu.module.css";
import { useCallback, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { usePositionedPopover } from "./usePositionedPopover";

/** One single-select option inside a filter section. */
export type FilterOption = {
	value: string;
	label: string;
};

/**
 * One titled group inside the filter popover. Sections are data, not
 * components: a later filter (bundle type, enabled state, ...) adds another
 * entry to the sections array without touching this menu.
 */
export type FilterSection = {
	id: string;
	title: string;
	defaultValue: string;
	selectedValue: string;
	options: readonly FilterOption[];
	onSelect: (value: string) => void;
};

type FilterMenuProps = {
	sections: readonly FilterSection[];
};

// Catalog filtering has no single mod target, so per
// docs/decisions/0002-organize-action-pattern.md it earns its own toolbar
// control instead of living behind a per-mod context menu. This stays a
// thin popover over caller-supplied sections: new filters add data upstream,
// not branches here.
export function FilterMenu({ sections }: FilterMenuProps) {
	const triggerRef = useRef<HTMLButtonElement>(null);
	const [anchor, setAnchor] = useState<{ x: number; y: number } | null>(null);
	const open = anchor !== null;

	const close = useCallback(() => {
		setAnchor(null);
	}, []);

	const { popoverRef, position } = usePositionedPopover<HTMLDivElement>(
		anchor?.x ?? 0,
		anchor?.y ?? 0,
		close,
	);

	function toggleOpen() {
		if (open) {
			close();
			return;
		}
		const rect = triggerRef.current?.getBoundingClientRect();
		if (!rect) return;
		setAnchor({ x: rect.left, y: rect.bottom + 6 });
	}

	const activeCount = sections.filter(
		(section) => section.selectedValue !== section.defaultValue,
	).length;
	const container = triggerRef.current?.closest<HTMLElement>(".app-shell");

	return (
		<>
			<button
				type="button"
				ref={triggerRef}
				className={[styles["filter-menu-trigger"], activeCount > 0 ? styles.active : ""]
					.filter(Boolean)
					.join(" ")}
				aria-haspopup="true"
				aria-expanded={open}
				aria-label="Filter catalog"
				title="Filter catalog"
				onClick={toggleOpen}
			>
				<ListFilter aria-hidden="true" />
				{activeCount > 0 && <span>{activeCount}</span>}
				<ChevronDown aria-hidden="true" />
			</button>
			{open &&
				container &&
				createPortal(
					<div
						ref={popoverRef}
						className={styles["filter-menu-popover"]}
						role="menu"
						aria-label="Filter catalog"
						style={{
							left: position.left,
							top: position.top,
							visibility: position.ready ? "visible" : "hidden",
						}}
					>
						{sections.map((section) => (
							<section key={section.id} aria-label={section.title}>
								<h3 className={styles["filter-menu-heading"]}>{section.title}</h3>
								<ul className={styles["filter-menu-list"]}>
									{section.options.map((option) => {
										const selected = section.selectedValue === option.value;
										return (
											<li key={option.value}>
												<button
													type="button"
													className={styles["filter-menu-option"]}
													aria-pressed={selected}
													onClick={() => section.onSelect(option.value)}
												>
													{selected && <Check aria-hidden="true" />}
													<span>{option.label}</span>
												</button>
											</li>
										);
									})}
								</ul>
							</section>
						))}
					</div>,
					container,
				)}
		</>
	);
}
