import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

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
	const menuRef = useRef<HTMLDivElement>(null);
	const [position, setPosition] = useState({ left: state.x, top: state.y, ready: false });

	useLayoutEffect(() => {
		const menu = menuRef.current;
		if (!menu) return;

		const margin = 8;
		const { width, height } = menu.getBoundingClientRect();
		const left = Math.min(Math.max(margin, state.x), window.innerWidth - width - margin);
		const top = Math.min(Math.max(margin, state.y), window.innerHeight - height - margin);
		setPosition({ left, top, ready: true });
	}, [state.x, state.y]);

	useEffect(() => {
		function handlePointerDown(event: PointerEvent) {
			if (!menuRef.current?.contains(event.target as Node)) {
				onClose();
			}
		}
		function handleKeyDown(event: KeyboardEvent) {
			if (event.key === "Escape") onClose();
		}

		window.addEventListener("pointerdown", handlePointerDown, true);
		window.addEventListener("keydown", handleKeyDown);
		window.addEventListener("scroll", onClose, true);
		window.addEventListener("resize", onClose);
		return () => {
			window.removeEventListener("pointerdown", handlePointerDown, true);
			window.removeEventListener("keydown", handleKeyDown);
			window.removeEventListener("scroll", onClose, true);
			window.removeEventListener("resize", onClose);
		};
	}, [onClose]);

	return createPortal(
		<div
			className="context-menu"
			ref={menuRef}
			role="menu"
			aria-label={state.title}
			style={{
				left: position.left,
				top: position.top,
				visibility: position.ready ? "visible" : "hidden",
			}}
		>
			<p className="context-menu-title">{state.title}</p>
			{state.items.map((item) => (
				<button
					type="button"
					className={`context-menu-item${item.destructive ? " destructive" : ""}`}
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
