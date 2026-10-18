import type { discovery, metadata, modtype } from "../../wailsjs/go/models";
import { cn } from "cn";
import { entryCategoryLabel, entryCharacterLabel, entryStateLabel } from "./entryPresentation";

type SelectedModPanelProps = {
	entry: discovery.Entry | null;
	identity?: modtype.Identity | undefined;
	isClassifying?: boolean | undefined;
	assignedTags: metadata.Tag[];
};

/** Status readout for the viewed mod. Actions live on the card, context menu, and header Actions. */
export function SelectedModPanel({
	entry,
	identity,
	isClassifying,
	assignedTags,
}: SelectedModPanelProps) {
	if (!entry) {
		return (
			<section
				className={cn(
					"mt-4 flex items-center justify-between gap-4 rounded-[0.7rem] border p-[0.9rem_1rem] max-[760px]:flex-col max-[760px]:items-stretch [&_p]:m-0",
					"border-border bg-popover/84",
				)}
				aria-label="Selected mod"
			>
				<div>
					<p className="eyebrow">Selected mod</p>
					<p>Select a mod to inspect it.</p>
				</div>
				<p className="max-w-[18rem] text-right text-[0.76rem] text-muted-foreground max-[760px]:max-w-none max-[760px]:text-left">
					Right-click a mod for rename, priority, and move actions.
				</p>
			</section>
		);
	}

	const stateLabel = entryStateLabel(entry);
	const categoryLabel = entryCategoryLabel(identity);
	const characterLabel = entryCharacterLabel(identity);

	return (
		<section
			className={cn(
				"mt-4 flex items-center justify-between gap-4 rounded-[0.7rem] border p-[0.9rem_1rem] max-[760px]:flex-col max-[760px]:items-stretch [&_p]:m-0",
				"border-primary/44 bg-primary/7",
			)}
			aria-label="Selected mod"
		>
			<div>
				<p className="eyebrow">Selected mod</p>
				<h3 className="my-[0.2rem] max-w-[min(36rem,50vw)] text-[0.95rem] [overflow-wrap:anywhere] max-[760px]:max-w-none">
					{entry.displayName}
				</h3>
				<p className="text-[0.76rem] text-muted-foreground">
					{entry.relativeFolder || "Library root"} · {stateLabel}
					{categoryLabel
						? ` · ${categoryLabel}`
						: isClassifying
							? " · Classifying..."
							: ""}
					{characterLabel ? ` · ${characterLabel}` : ""}
					{" · "}Priority {entry.priority.value}
				</p>
				{assignedTags.length > 0 && (
					<ul
						className="mt-2 mb-0 flex list-none flex-wrap gap-[0.35rem] p-0"
						aria-label="Tags"
					>
						{assignedTags.map((tag) => (
							<li
								key={tag.id}
								className="rounded-full border border-primary/50 bg-primary/12 px-[0.5rem] py-[0.2rem] text-[0.7rem] font-bold text-foreground"
							>
								{tag.name}
							</li>
						))}
					</ul>
				)}
			</div>
		</section>
	);
}
