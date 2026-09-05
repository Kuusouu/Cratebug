import type { discovery, metadata, modtype } from "../../wailsjs/go/models";
import styles from "./SelectedModPanel.module.css";
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
				className={[styles["selected-mod-panel"], styles.empty].join(" ")}
				aria-label="Selected mod"
			>
				<div>
					<p className="eyebrow">Selected mod</p>
					<p>Select a mod to inspect it.</p>
				</div>
				<p className={styles["selected-mod-hint"]}>
					Right-click a mod for rename, priority, and move actions.
				</p>
			</section>
		);
	}

	const stateLabel = entryStateLabel(entry);
	const categoryLabel = entryCategoryLabel(identity);
	const characterLabel = entryCharacterLabel(identity);

	return (
		<section className={styles["selected-mod-panel"]} aria-label="Selected mod">
			<div className={styles["selected-mod-details"]}>
				<p className="eyebrow">Selected mod</p>
				<h3>{entry.displayName}</h3>
				<p>
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
					<ul className={styles["selected-mod-tags"]} aria-label="Tags">
						{assignedTags.map((tag) => (
							<li key={tag.id}>{tag.name}</li>
						))}
					</ul>
				)}
			</div>
		</section>
	);
}
