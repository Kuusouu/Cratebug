import { cn } from "cn";
import { CircleAlert, CircleCheckBig, TriangleAlert, X } from "lucide-react";
import { useEffect } from "react";

import { Button } from "@/components/ui/button";

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

	// Dialog content and backdrops use z-50, so feedback stays above them.
	return (
		<div
			className={cn(
				"fixed bottom-6 left-1/2 z-60 flex w-[min(24rem,calc(100vw-2.5rem))] -translate-x-1/2 animate-in items-start gap-[0.65rem] rounded-[0.7rem] border bg-popover/82 px-[0.8rem] py-3 shadow-toast backdrop-blur-[1rem] backdrop-saturate-110 duration-160 ease-out fade-in slide-in-from-bottom-2 motion-reduce:animate-none",
				feedback.kind === "success" && "border-good/58 [&>svg]:text-good",
				feedback.kind === "error" && "border-destructive/62 [&>svg]:text-destructive",
				feedback.kind === "warning" && "border-warn/62 [&>svg]:text-warn",
			)}
			role={feedback.kind === "success" ? "status" : "alert"}
		>
			<Icon className="mt-[0.05rem] size-[1.1rem] shrink-0" aria-hidden="true" />
			<p className="m-0 min-w-0 flex-1 text-[0.8rem] leading-[1.4] font-[650] [overflow-wrap:anywhere]">
				{feedback.message}
			</p>
			<Button
				type="button"
				variant="ghost"
				size="icon"
				className="size-[1.65rem] rounded-[0.4rem] border-transparent bg-transparent text-muted-foreground hover:border-border hover:bg-muted-foreground/10 hover:text-foreground hover:filter-none dark:hover:bg-muted-foreground/10"
				onClick={onDismiss}
				aria-label="Dismiss"
			>
				<X className="size-[0.9rem]" aria-hidden="true" />
			</Button>
		</div>
	);
}
