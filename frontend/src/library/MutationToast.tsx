import { CircleAlert, CircleCheckBig, TriangleAlert, X } from "lucide-react";
import styles from "./MutationToast.module.css";
import { useEffect } from "react";

export type MutationFeedback = {
	id: number;
	kind: "error" | "success" | "warning";
	message: string;
};

type MutationToastProps = {
	feedback: MutationFeedback;
	onDismiss: () => void;
};

const successToastDurationMilliseconds = 5000;
// Errors stay visible longer so people can read and dismiss actionable recovery guidance.
const errorToastDurationMilliseconds = 8000;

// Keeps mutation feedback out of the catalog layout while still allowing it to be dismissed.
export function MutationToast({ feedback, onDismiss }: MutationToastProps) {
	const duration =
		feedback.kind === "success"
			? successToastDurationMilliseconds
			: errorToastDurationMilliseconds;
	const Icon =
		feedback.kind === "success"
			? CircleCheckBig
			: feedback.kind === "warning"
				? TriangleAlert
				: CircleAlert;

	useEffect(() => {
		const timeout = window.setTimeout(onDismiss, duration);
		return () => window.clearTimeout(timeout);
	}, [duration, onDismiss]);

	return (
		<div
			className={[styles["mutation-toast"], styles[feedback.kind]].join(" ")}
			role={feedback.kind === "success" ? "status" : "alert"}
		>
			<Icon aria-hidden="true" />
			<p>{feedback.message}</p>
			<button
				type="button"
				className={styles["mutation-toast-close"]}
				onClick={onDismiss}
				aria-label="Dismiss"
			>
				<X aria-hidden="true" />
			</button>
		</div>
	);
}
