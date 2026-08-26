import { useEffect, useRef } from "react";

const focusableSelector =
	'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

// role="dialog" aria-modal="true" only describes the intent; nothing in the
// DOM enforces it. This keeps Tab from reaching the catalog behind the
// overlay and lets Escape close the dialog, matching what aria-modal claims.
export function useDialogFocusTrap<T extends HTMLElement>(onClose: () => void) {
	const containerRef = useRef<T>(null);

	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		// TypeScript cannot narrow `container` across this closure boundary, but the
		// effect cleanup removes this listener before containerRef could point elsewhere.
		function handleKeyDown(this: HTMLElement, event: KeyboardEvent) {
			if (event.key === "Escape") {
				event.stopPropagation();
				onClose();
				return;
			}
			if (event.key !== "Tab") return;

			const focusable = Array.from(this.querySelectorAll<HTMLElement>(focusableSelector));
			const first = focusable.at(0);
			const last = focusable.at(-1);
			if (!first || !last) return;

			const isOutside = !this.contains(document.activeElement);

			if (event.shiftKey) {
				if (isOutside || document.activeElement === first) {
					event.preventDefault();
					last.focus();
				}
			} else if (isOutside || document.activeElement === last) {
				event.preventDefault();
				first.focus();
			}
		}

		const activeContainer: HTMLElement = container;

		// Blocks wheel input from reaching the page/library behind the dialog
		function handleWheel(event: WheelEvent) {
			const target = event.target as HTMLElement | null;
			if (!target || !activeContainer.contains(target)) {
				event.preventDefault();
				return;
			}

			let el: HTMLElement | null = target;
			let isScrollable = false;
			while (el && el !== activeContainer) {
				const style = window.getComputedStyle(el);
				const overflowY = style.overflowY;
				if (
					(overflowY === "auto" || overflowY === "scroll") &&
					el.scrollHeight > el.clientHeight
				) {
					isScrollable = true;
					break;
				}
				el = el.parentElement;
			}

			if (!isScrollable) {
				event.preventDefault();
			}
		}

		container.addEventListener("keydown", handleKeyDown);
		window.addEventListener("wheel", handleWheel, { passive: false });
		return () => {
			container.removeEventListener("keydown", handleKeyDown);
			window.removeEventListener("wheel", handleWheel);
		};
	}, [onClose]);

	return containerRef;
}
