import { ChevronRight, Package, X } from "lucide-react";
import styles from "./ConflictDetailsDialog.module.css";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { conflict, discovery, modtype } from "../../wailsjs/go/models";
import { canOrganizeMod, characterHeroPortraitUrl } from "./entryPresentation";
import { maximumPriorityFor } from "./modPriority";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type ConflictDetailsDialogProps = {
	result: conflict.Result;
	entries: readonly discovery.Entry[];
	identitiesByEntryID: Record<string, modtype.Identity>;
	isMutationLocked: boolean;
	onClose: () => void;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
};

// Groups each conflict.Group by the character identity shared among its participants.
// When all participants resolve to the same characterID, the group is filed under that
// character. Mixed-character groups fall back to a "Multiple characters" bucket so no
// group is silently lost.
type CharacterBucket = {
	characterID: string;
	characterName: string;
	groups: conflict.Group[];
};

function groupByCharacter(
	groups: conflict.Group[],
	identitiesByEntryID: Record<string, modtype.Identity>,
): CharacterBucket[] {
	const buckets = new Map<string, CharacterBucket>();

	for (const group of groups ?? []) {
		const participants = group.participants ?? [];
		const characterIDs = participants.map(
			(p) => identitiesByEntryID[p.entryID]?.characterID ?? "",
		);

		// Every participant must resolve to the same non-empty character before this
		// group is filed under it; an unresolved participant (identity not loaded yet,
		// or no hero association at all) must not be silently dropped from the check,
		// or a group could be mislabeled under just one participant's character.
		const uniqueCharacters = [...new Set(characterIDs)];
		const bucketKey =
			characterIDs.every(Boolean) && uniqueCharacters.length === 1 && uniqueCharacters[0]
				? uniqueCharacters[0]
				: "__mixed__";

		const firstCharacterID = uniqueCharacters[0] ?? "";
		const repParticipantID =
			participants.find(
				(p) => identitiesByEntryID[p.entryID]?.characterID === firstCharacterID,
			)?.entryID ?? "";
		const characterName =
			bucketKey === "__mixed__"
				? "Multiple characters"
				: identitiesByEntryID[repParticipantID]?.characterName ||
					firstCharacterID ||
					"Unknown";

		if (!buckets.has(bucketKey)) {
			buckets.set(bucketKey, { characterID: bucketKey, characterName, groups: [] });
		}
		buckets.get(bucketKey)?.groups.push(group);
	}

	return [...buckets.values()].sort((a, b) => {
		if (a.characterID === "__mixed__") return 1;
		if (b.characterID === "__mixed__") return -1;
		return a.characterName.localeCompare(b.characterName);
	});
}

// Presents conflict scan results grouped by resolved character, with hero thumbnails,
// relationship labels, overlapping path counts, and per-participant priority switchers
// so users can adjust load order without leaving the view.
export function ConflictDetailsDialog({
	result,
	entries,
	identitiesByEntryID,
	isMutationLocked,
	onClose,
	onSetPriority,
}: ConflictDetailsDialogProps) {
	const closeButtonRef = useRef<HTMLButtonElement>(null);
	const handleEscape = useCallback(() => onClose(), [onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	const entriesByID = useMemo(() => new Map(entries.map((e) => [e.id, e])), [entries]);
	const groups = result.groups ?? [];
	const unavailable = result.unavailable ?? [];

	const characterBuckets = useMemo(
		() => groupByCharacter(groups, identitiesByEntryID),
		[groups, identitiesByEntryID],
	);

	const samePriorityCount = groups.filter((g) => g.relationship === "same_priority").length;
	const crossPriorityCount = groups.length - samePriorityCount;

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className={["mutation-dialog", styles["conflict-details-dialog"]].join(" ")}
				aria-labelledby="conflict-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div className="conflict-dialog-header">
					<div>
						<p className="eyebrow">Conflict report</p>
						<h2 id="conflict-dialog-title">Asset conflicts</h2>
						<p
							className={[
								"mutation-dialog-subtitle",
								styles["conflict-summary-pills"],
							].join(" ")}
						>
							{samePriorityCount > 0 && (
								<span
									className={[
										styles["conflict-summary-pill"],
										styles["same-priority"],
									].join(" ")}
								>
									{samePriorityCount} duplicate priority
								</span>
							)}
							{crossPriorityCount > 0 && (
								<span
									className={[
										styles["conflict-summary-pill"],
										styles["cross-priority"],
									].join(" ")}
								>
									{crossPriorityCount} cross-priority
								</span>
							)}
							{unavailable.length > 0 && (
								<span
									className={[
										styles["conflict-summary-pill"],
										styles.unavailable,
									].join(" ")}
								>
									{unavailable.length} unavailable
								</span>
							)}
						</p>
					</div>
					<button
						ref={closeButtonRef}
						type="button"
						className="icon-button conflict-dialog-close"
						onClick={onClose}
						aria-label="Close conflict details"
					>
						<X aria-hidden="true" />
					</button>
				</div>

				{unavailable.length > 0 && (
					<p className={styles["conflict-unavailable-notice"]} role="note">
						{unavailable.length === 1
							? "1 enabled mod could not be scanned (encrypted or unreadable) and is excluded from these results."
							: `${unavailable.length} enabled mods could not be scanned (encrypted or unreadable) and are excluded from these results.`}
					</p>
				)}

				<div className={[styles["conflict-groups-list"], "scroll-y"].join(" ")}>
					{characterBuckets.map((bucket) => (
						<section
							key={bucket.characterID}
							className={styles["conflict-character-section"]}
						>
							<ConflictCharacterHeading
								characterID={bucket.characterID}
								characterName={bucket.characterName}
							/>
							{bucket.groups.map((group) => (
								<ConflictGroupCard
									key={(group.participants ?? []).map((p) => p.entryID).join(",")}
									group={group}
									entriesByID={entriesByID}
									isMutationLocked={isMutationLocked}
									onSetPriority={onSetPriority}
								/>
							))}
						</section>
					))}
				</div>

				<div className="mutation-dialog-actions">
					<button type="button" className="quiet-button" onClick={onClose}>
						Close
					</button>
				</div>
			</section>
		</div>
	);
}

type ConflictCharacterHeadingProps = {
	characterID: string;
	characterName: string;
};

// Renders a character section heading with default hero avatar when available.
function ConflictCharacterHeading({ characterID, characterName }: ConflictCharacterHeadingProps) {
	const portraitUrl = characterHeroPortraitUrl(characterID);

	return (
		<div className={styles["conflict-character-heading"]}>
			<div className={styles["conflict-character-thumbnail"]}>
				{portraitUrl ? (
					<img src={portraitUrl} alt="" className="mod-thumbnail-hero" />
				) : (
					<Package aria-hidden="true" />
				)}
			</div>
			<h3>{characterName}</h3>
		</div>
	);
}

type ConflictGroupCardProps = {
	group: conflict.Group;
	entriesByID: ReadonlyMap<string, discovery.Entry>;
	isMutationLocked: boolean;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
};

// One conflict group: relationship label, total overlapping path count, and each
// participant with its priority switcher and the specific paths it contributes.
function ConflictGroupCard({
	group,
	entriesByID,
	isMutationLocked,
	onSetPriority,
}: ConflictGroupCardProps) {
	const isSamePriority = group.relationship === "same_priority";
	return (
		<div
			className={[
				styles["conflict-group-card"],
				isSamePriority ? styles["same-priority"] : styles["cross-priority"],
			].join(" ")}
		>
			<div className={styles["conflict-group-meta"]}>
				<span
					className={[
						styles["conflict-relationship-badge"],
						isSamePriority ? styles["same-priority"] : styles["cross-priority"],
					].join(" ")}
				>
					{isSamePriority ? "Duplicate priority" : "Cross-priority"}
				</span>
				<span className={styles["conflict-path-count"]}>
					{group.pathCount} overlapping {group.pathCount === 1 ? "path" : "paths"}
				</span>
			</div>
			<ul className={styles["conflict-participants"]}>
				{(group.participants ?? []).map((participant) => {
					const entry = entriesByID.get(participant.entryID) ?? null;
					return (
						<ConflictParticipantRow
							key={participant.entryID}
							participant={participant}
							entry={entry}
							isMutationLocked={isMutationLocked}
							onSetPriority={onSetPriority}
						/>
					);
				})}
			</ul>
		</div>
	);
}

type ConflictParticipantRowProps = {
	participant: conflict.Participant;
	entry: discovery.Entry | null;
	isMutationLocked: boolean;
	onSetPriority: (entry: discovery.Entry, priority: number) => Promise<boolean>;
};

// One participant within a conflict group: mod name, priority value with +/- buttons
// to adjust load order in place, and the specific overlapping paths it contributes.
function ConflictParticipantRow({
	participant,
	entry,
	isMutationLocked,
	onSetPriority,
}: ConflictParticipantRowProps) {
	const [isBusy, setIsBusy] = useState(false);
	const [pathsExpanded, setPathsExpanded] = useState(false);
	const canAdjust = entry !== null && canOrganizeMod(entry) && !isMutationLocked && !isBusy;
	const maxPriority = entry ? maximumPriorityFor(entry) : 0;
	const currentPriority = entry?.priority?.value ?? participant.priority.value;
	const overlappingPaths = participant.overlappingPaths ?? [];

	async function adjustPriority(delta: number) {
		if (!entry || !canAdjust) return;
		const next = Math.max(0, Math.min(maxPriority, currentPriority + delta));
		if (next === currentPriority) return;
		setIsBusy(true);
		try {
			await onSetPriority(entry, next);
		} finally {
			setIsBusy(false);
		}
	}

	return (
		<li className={styles["conflict-participant"]}>
			<div className={styles["conflict-participant-header"]}>
				<span className={styles["conflict-participant-name"]}>
					{participant.displayName}
				</span>
				<div className={styles["conflict-priority-control"]}>
					<button
						type="button"
						className={styles["conflict-priority-step"]}
						aria-label={`Decrease priority of ${participant.displayName}`}
						disabled={!canAdjust || currentPriority <= 0}
						onClick={() => void adjustPriority(-1)}
					>
						−
					</button>
					<span className={styles["conflict-priority-value"]} aria-hidden="true">
						{currentPriority}
					</span>
					<button
						type="button"
						className={styles["conflict-priority-step"]}
						aria-label={`Increase priority of ${participant.displayName}`}
						disabled={!canAdjust || currentPriority >= maxPriority}
						onClick={() => void adjustPriority(1)}
					>
						+
					</button>
				</div>
			</div>
			{overlappingPaths.length > 0 && (
				<>
					<button
						type="button"
						className={styles["conflict-paths-toggle"]}
						aria-expanded={pathsExpanded}
						onClick={() => setPathsExpanded((expanded) => !expanded)}
					>
						<ChevronRight
							aria-hidden="true"
							className={`chevron-icon${pathsExpanded ? " expanded" : ""}`}
						/>
						{overlappingPaths.length} overlapping{" "}
						{overlappingPaths.length === 1 ? "file" : "files"}
					</button>
					{pathsExpanded && (
						<ul className={styles["conflict-paths"]}>
							{overlappingPaths.map((path) => (
								<li key={path} className={styles["conflict-path"]} title={path}>
									{path.split("/").pop() ?? path}
								</li>
							))}
						</ul>
					)}
				</>
			)}
		</li>
	);
}
