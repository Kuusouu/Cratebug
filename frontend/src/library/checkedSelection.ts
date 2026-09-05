/** Modifier keys from a card click that updates the checked set. */
export type CheckClickModifiers = {
	shiftKey: boolean;
	ctrlKey: boolean;
	metaKey: boolean;
};

/** Next checked set plus the anchor used for a later Shift-click range. */
export type CheckClickResult = {
	next: Set<string>;
	anchorID: string | null;
};

/**
 * Applies Ctrl / Shift rules to the current filtered list.
 * A plain click replaces the set with the clicked id, or empties the set when
 * that id was already the only member. Ctrl+click toggles. Shift+click adds
 * the range from the last anchor. Shift+Ctrl removes it.
 */
export function nextCheckedIDs(
	current: ReadonlySet<string>,
	visibleIDs: readonly string[],
	clickedID: string,
	lastAnchorID: string | null,
	modifiers: CheckClickModifiers,
): CheckClickResult {
	const ctrl = modifiers.ctrlKey || modifiers.metaKey;
	if (modifiers.shiftKey && lastAnchorID) {
		const start = visibleIDs.indexOf(lastAnchorID);
		const end = visibleIDs.indexOf(clickedID);
		if (start !== -1 && end !== -1) {
			const low = Math.min(start, end);
			const high = Math.max(start, end);
			const next = new Set(current);
			for (let index = low; index <= high; index += 1) {
				const id = visibleIDs[index];
				if (!id) continue;
				if (ctrl) {
					next.delete(id);
				} else {
					next.add(id);
				}
			}
			return { next, anchorID: lastAnchorID };
		}
	}

	if (ctrl) {
		const next = new Set(current);
		if (next.has(clickedID)) {
			next.delete(clickedID);
		} else {
			next.add(clickedID);
		}
		return { next, anchorID: clickedID };
	}

	if (current.size === 1 && current.has(clickedID)) {
		return { next: new Set(), anchorID: null };
	}
	return { next: new Set([clickedID]), anchorID: clickedID };
}

/** Replaces a scanner ID in the checked set after rename, priority, or move. */
export function remapCheckedID(
	current: ReadonlySet<string>,
	previousID: string | undefined,
	nextID: string,
): Set<string> {
	const next = new Set(current);
	if (previousID && next.has(previousID)) {
		next.delete(previousID);
		next.add(nextID);
	}
	return next;
}

/** Drops checked IDs that are no longer in the scanned library. */
export function retainCheckedIDs(
	current: ReadonlySet<string>,
	presentIDs: ReadonlySet<string>,
): Set<string> {
	const next = new Set<string>();
	for (const id of current) {
		if (presentIDs.has(id)) next.add(id);
	}
	return next;
}
