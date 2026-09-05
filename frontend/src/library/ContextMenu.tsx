import { createPortal } from "react-dom";
import styles from "./ContextMenu.module.css";
import { usePositionedPopover } from "./usePositionedPopover";

export type ContextMenuItem = {
	label: string;
	onSelect: () => void;
	disabled?: boolean;
	destructive?: boolean;
};

export type ContextMenuState = {
	x: number;
	y: number;
	container: HTMLElement;
	title: string;
	items: ContextMenuItem[];
};

type ContextMenuProps = {
	state: ContextMenuState;
	onClose: () => void;
};

// Renders a positioned popup menu, clamped to the viewport and dismissed by an
// outside click, Escape, or scroll/resize the way the folder tooltip already is.
export function ContextMenu({ state, onClose }: ContextMenuProps) {
	const { popoverRef, position } = usePositionedPopover<HTMLDivElement>(
		state.x,
		state.y,
		onClose,
	);

	return createPortal(
		<div
			className={styles["context-menu"]}
			ref={popoverRef}
			role="menu"
			aria-label={state.title}
			style={{
				left: position.left,
				top: position.top,
				visibility: position.ready ? "visible" : "hidden",
			}}
		>
			<p className={styles["context-menu-title"]}>{state.title}</p>
			{state.items.map((item) => (
				<button
					type="button"
					className={[
						styles["context-menu-item"],
						item.destructive ? styles.destructive : "",
					]
						.filter(Boolean)
						.join(" ")}
					disabled={item.disabled}
					key={item.label}
					role="menuitem"
					onClick={() => {
						onClose();
						item.onSelect();
					}}
				>
					{item.label}
				</button>
			))}
		</div>,
		state.container,
	);
}
