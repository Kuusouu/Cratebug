import { cn } from "cn";
import { ChevronDown, ListFilter } from "lucide-react";

import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuLabel,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

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
	const activeCount = sections.filter(
		(section) => section.selectedValue !== section.defaultValue,
	).length;

	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				render={
					<button
						type="button"
						className={cn(
							"flex items-center gap-[0.4rem] rounded-[0.55rem] border border-border bg-popover px-[0.8rem] py-[0.65rem] font-bold text-foreground hover:border-primary/48 hover:filter-none [&_svg]:size-4",
							activeCount > 0 && "border-primary/60 bg-primary/12 text-primary",
						)}
						aria-label="Filter catalog"
						title="Filter catalog"
					/>
				}
			>
				<ListFilter aria-hidden="true" />
				{activeCount > 0 && <span>{activeCount}</span>}
				<ChevronDown aria-hidden="true" />
			</DropdownMenuTrigger>
			<DropdownMenuContent
				sideOffset={6}
				// Wide enough for the longest option label regardless of trigger width.
				className="w-64"
			>
				{sections.map((section) => (
					<DropdownMenuGroup key={section.id}>
						<DropdownMenuLabel>{section.title}</DropdownMenuLabel>
						{/* Picking an option is a filter change, not a menu action: the
						    radio group and menu stay open while the caller updates. */}
						<DropdownMenuRadioGroup value={section.selectedValue}>
							{section.options.map((option) => (
								<DropdownMenuRadioItem
									key={option.value}
									value={option.value}
									onClick={() => section.onSelect(option.value)}
								>
									{option.label}
								</DropdownMenuRadioItem>
							))}
						</DropdownMenuRadioGroup>
					</DropdownMenuGroup>
				))}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
