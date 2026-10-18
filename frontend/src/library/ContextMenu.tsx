// oxlint-disable-next-line no-restricted-imports -- The pointer-anchored menu needs a virtual anchor and shell portal container that the shared wrapper doesn't expose.
import { Menu } from "@base-ui/react/menu";

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

// Renders the right-click actions for one mod or folder at the pointer
// coordinates, portaled into the shell so it stacks above the grid.
export function ContextMenu({ state, onClose }: ContextMenuProps) {
	return (
		<Menu.Root
			open
			onOpenChange={(nextOpen) => {
				if (!nextOpen) onClose();
			}}
		>
			<Menu.Portal container={state.container}>
				<Menu.Positioner
					// Floating UI accepts a virtual element, so the click point anchors
					// the popup without a trigger element behind it.
					anchor={{ getBoundingClientRect: () => new DOMRect(state.x, state.y, 0, 0) }}
					align="start"
					side="bottom"
					sideOffset={0}
					className="isolate z-50 outline-none"
				>
					<Menu.Popup
						aria-label={state.title}
						className="z-50 max-h-(--available-height) min-w-44 overflow-x-hidden overflow-y-auto rounded-lg bg-popover p-1 text-popover-foreground shadow-md ring-1 ring-foreground/10 duration-100 outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95"
					>
						<div className="px-1.5 py-1 text-xs font-medium text-muted-foreground">
							{state.title}
						</div>
						{state.items.map((item) => (
							<Menu.Item
								key={item.label}
								disabled={item.disabled}
								data-variant={item.destructive ? "destructive" : "default"}
								onClick={item.onSelect}
								className="relative flex cursor-default items-center gap-1.5 rounded-md px-1.5 py-1 text-sm outline-hidden select-none focus:bg-accent focus:text-accent-foreground data-[variant=destructive]:text-destructive data-[variant=destructive]:focus:bg-destructive/10 data-[variant=destructive]:focus:text-destructive data-disabled:pointer-events-none data-disabled:opacity-50"
							>
								{item.label}
							</Menu.Item>
						))}
					</Menu.Popup>
				</Menu.Positioner>
			</Menu.Portal>
		</Menu.Root>
	);
}
