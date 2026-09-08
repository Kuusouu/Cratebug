import { useEffect, useState } from "react";

// SPEC.md requires a short deliberate delay before destructive confirmation.
const confirmDelaySeconds = 3;

/**
 * Counts the shared destructive-confirmation delay down to zero and reports
 * whether the confirm button may be enabled yet. When skip is true the delay
 * is never armed, matching the Settings opt-out: the countdown is only a UI
 * safeguard, and the backend safety checks run either way.
 */
export function useConfirmDelay(skip: boolean) {
	const [secondsRemaining, setSecondsRemaining] = useState(skip ? 0 : confirmDelaySeconds);

	useEffect(() => {
		if (secondsRemaining <= 0) return;
		const timeout = window.setTimeout(
			() => setSecondsRemaining((current) => current - 1),
			1000,
		);
		return () => window.clearTimeout(timeout);
	}, [secondsRemaining]);

	return { secondsRemaining, ready: secondsRemaining <= 0 };
}
