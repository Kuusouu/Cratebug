import { useEffect, useLayoutEffect, useRef, useState } from "react";

type PopoverPosition = { left: number; top: number; ready: boolean };

// Shared positioning/dismissal mechanics for a popup anchored to a fixed
// point: viewport-clamped placement, and dismissal on an outside pointerdown,
// Escape, or scroll/resize. Extracted from ContextMenu so TagMenu's popover
// can reuse the same mechanics anchored to its trigger button instead of a
// click point. Callers only mount the popover element while it is open, so
// the dismissal listeners are attached and torn down with it.
export function usePositionedPopover<T extends HTMLElement>(
	x: number,
	y: number,
	onClose: () => void,
) {
	const popoverRef = useRef<T>(null);
	const [position, setPosition] = useState<PopoverPosition>({ left: x, top: y, ready: false });

	useLayoutEffect(() => {
		const popover = popoverRef.current;
		if (!popover) return;

		const margin = 8;
		const { width, height } = popover.getBoundingClientRect();
		const left = Math.min(Math.max(margin, x), window.innerWidth - width - margin);
		const top = Math.min(Math.max(margin, y), window.innerHeight - height - margin);
		setPosition({ left, top, ready: true });
	}, [x, y]);

	useEffect(() => {
		function handlePointerDown(event: PointerEvent) {
			if (!popoverRef.current?.contains(event.target as Node)) {
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

	return { popoverRef, position };
}
