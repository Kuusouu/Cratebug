import type { discovery, metadata, modtype } from "../../wailsjs/go/models";
import styles from "./SelectedModPanel.module.css";
import {
	canChangeModState,
	canDeleteMod,
	entryCategoryLabel,
	entryCharacterLabel,
	entryStateLabel,
} from "./entryPresentation";

type SelectedModPanelProps = {
	entry: discovery.Entry | null;
	identity?: modtype.Identity | undefined;
	isClassifying?: boolean | undefined;
	assignedTags: metadata.Tag[];
	isMutating: boolean;
	isMutationLocked: boolean;
	onClear: () => void;
	onSetEnabled: (entry: discovery.Entry) => void;
	onDelete: () => void;
};

// Keeps the current selection and its available actions in one stable location.
export function SelectedModPanel({
	entry,
	identity,
	isClassifying,
	assignedTags,
	isMutating,
	isMutationLocked,
	onClear,
	onSetEnabled,
	onDelete,
}: SelectedModPanelProps) {
	if (!entry) {
		return (
			<section
				className={[styles["selected-mod-panel"], styles.empty].join(" ")}
				aria-label="Mod actions"
			>
				<div>
					<p className="eyebrow">Mod actions</p>
					<p>Select a mod to organize it.</p>
				</div>
				<p className={styles["selected-mod-hint"]}>
					Right-click a mod for rename, priority, and move actions.
				</p>
			</section>
		);
	}

	const canChangeState = canChangeModState(entry);
	const enabled = entry.state === "enabled";
	const stateLabel = entryStateLabel(entry);
	const categoryLabel = entryCategoryLabel(identity);
	const characterLabel = entryCharacterLabel(identity);

	return (
		<section className={styles["selected-mod-panel"]} aria-label="Selected mod actions">
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
			<div className={styles["selected-mod-actions"]}>
				{canChangeState && (
					<button
						type="button"
						className={styles["mod-action"]}
						disabled={isMutationLocked}
						onClick={() => onSetEnabled(entry)}
					>
						{isMutating
							? enabled
								? "Disabling..."
								: "Enabling..."
							: isMutationLocked
								? "Working..."
								: enabled
									? "Disable"
									: "Enable"}
					</button>
				)}
				{canDeleteMod(entry) && (
					<button
						type="button"
						className="destructive-button"
						disabled={isMutationLocked}
						onClick={onDelete}
					>
						Delete
					</button>
				)}
				<button
					type="button"
					className="quiet-button"
					disabled={isMutationLocked}
					onClick={onClear}
				>
					Clear selection
				</button>
			</div>
		</section>
	);
}
