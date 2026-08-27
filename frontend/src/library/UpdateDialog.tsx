import { Download, Heart, X } from "lucide-react";
import { useCallback, useEffect, useRef } from "react";
import type { update } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

export type UpdateDownloadProgress = {
	downloaded: number;
	total: number;
};

type UpdateDialogProps = {
	release: update.Release;
	// "available": a newer release exists and can be downloaded and applied.
	// "installed": the running build's own changelog, shown once right after
	// an update relaunches Cratebug -- nothing left to download or apply.
	mode: "available" | "installed";
	isDownloading: boolean;
	isReady: boolean;
	downloadProgress: UpdateDownloadProgress | null;
	onDownload: () => void;
	onApply: () => void;
	onClose: () => void;
};

const BYTES_PER_MEGABYTE = 1024 * 1024;

function formatMegabytes(bytes: number): string {
	return `${(bytes / BYTES_PER_MEGABYTE).toFixed(1)} MB`;
}

type ChangelogBlock =
	| { kind: "heading"; text: string; key: string }
	| { kind: "item"; text: string; key: string }
	| { kind: "text"; text: string; key: string };

// Splits the release body into headings, list items, and paragraphs. This
// only needs to understand Cratebug's own CHANGELOG.md convention (###
// headings, "- " list items), not arbitrary markdown.
function parseChangelog(notes: string): ChangelogBlock[] {
	return notes
		.split("\n")
		.map((line, index) => ({ line: line.trim(), key: `${index}` }))
		.filter(({ line }) => line.length > 0)
		.map(({ line, key }) => {
			if (line.startsWith("### "))
				return { kind: "heading" as const, text: line.slice(4), key };
			if (line.startsWith("- ")) return { kind: "item" as const, text: line.slice(2), key };
			return { kind: "text" as const, text: line, key };
		});
}

function ChangelogBody({ notes }: { notes: string }) {
	const blocks = parseChangelog(notes);
	if (blocks.length === 0) {
		return <p className="update-changelog-empty">No changelog details for this release.</p>;
	}

	const rendered: React.ReactNode[] = [];
	let pendingItems: ChangelogBlock[] = [];

	function flushItems() {
		const firstItem = pendingItems[0];
		if (!firstItem) return;
		rendered.push(
			<ul key={`list-${firstItem.key}`} className="update-changelog-list">
				{pendingItems.map((item) => (
					<li key={item.key}>{item.text}</li>
				))}
			</ul>,
		);
		pendingItems = [];
	}

	for (const block of blocks) {
		if (block.kind === "item") {
			pendingItems.push(block);
			continue;
		}
		flushItems();
		rendered.push(
			block.kind === "heading" ? (
				<h3 key={block.key}>{block.text}</h3>
			) : (
				<p key={block.key}>{block.text}</p>
			),
		);
	}
	flushItems();

	return <div className="update-changelog">{rendered}</div>;
}

export function UpdateDialog({
	release,
	mode,
	isDownloading,
	isReady,
	downloadProgress,
	onDownload,
	onApply,
	onClose,
}: UpdateDialogProps) {
	const closeButtonRef = useRef<HTMLButtonElement>(null);
	// Downloading is not cancellable yet, so Escape is ignored mid-download
	// rather than abandoning a transfer the user can't resume.
	const handleEscape = useCallback(() => {
		if (!isDownloading) onClose();
	}, [isDownloading, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	const percent =
		downloadProgress && downloadProgress.total > 0
			? Math.min(
					100,
					Math.round((downloadProgress.downloaded / downloadProgress.total) * 100),
				)
			: null;

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog update-dialog"
				aria-labelledby="update-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div className="conflict-dialog-header">
					<div>
						<p className="eyebrow">Cratebug</p>
						<h2 id="update-dialog-title">
							{mode === "available"
								? "What new crates are there?"
								: "What's new in this crate"}
						</h2>
						<p className="update-dialog-version">Version {release.version.tag}</p>
					</div>
					{!isDownloading && (
						<button
							ref={closeButtonRef}
							type="button"
							className="icon-button conflict-dialog-close"
							onClick={onClose}
							aria-label="Close"
						>
							<X aria-hidden="true" />
						</button>
					)}
				</div>

				<div className="update-dialog-body">
					{!isDownloading && <ChangelogBody notes={release.notes} />}

					{isDownloading && (
						<div className="update-download-progress">
							<div className="progress-bar">
								<div
									className="progress-fill"
									style={{ width: percent !== null ? `${percent}%` : "100%" }}
								/>
							</div>
							<span className="update-download-progress-text">
								{downloadProgress && percent !== null
									? `${percent}% (${formatMegabytes(downloadProgress.downloaded)} / ${formatMegabytes(downloadProgress.total)})`
									: downloadProgress
										? formatMegabytes(downloadProgress.downloaded)
										: "Starting download..."}
							</span>
						</div>
					)}

					{isReady && !isDownloading && (
						<p className="update-ready-notice">
							Downloaded and ready to install. Cratebug will restart.
						</p>
					)}
				</div>

				<div className="mutation-dialog-actions">
					{mode === "available" && !isReady && (
						<button
							type="button"
							className="quiet-button"
							onClick={() => BrowserOpenURL(release.htmlURL)}
							disabled={isDownloading}
						>
							View release
						</button>
					)}
					{mode === "available" && !isReady && (
						<button type="button" onClick={onDownload} disabled={isDownloading}>
							<Download aria-hidden="true" />
							{isDownloading ? "Downloading..." : "Download update"}
						</button>
					)}
					{mode === "available" && isReady && (
						<button type="button" onClick={onApply}>
							Install &amp; restart
						</button>
					)}
					{mode === "installed" && (
						<button type="button" onClick={onClose}>
							Nice!
						</button>
					)}
				</div>

				<p className="update-dialog-footer">
					Brought to you with <Heart className="update-heart" aria-hidden="true" /> by the
					maintainer(s) of Cratebug
				</p>
			</section>
		</div>
	);
}
