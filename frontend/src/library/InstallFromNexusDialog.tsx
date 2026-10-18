import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";

import { NexusAccount, ResolveNexusModPage } from "../../wailsjs/go/main/NexusService";
import type { main } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import {
	defaultNexusFileId,
	formatNexusFileSize,
	formatNexusInstallError,
	groupNexusFiles,
	nexusAdultContentBlockedMessage,
	nexusContentBlockingURL,
	nexusInstallStep,
	nexusModPageURL,
	parseNexusModPageInput,
} from "./nexusPresentation";

import styles from "./InstallFromNexusDialog.module.css";

type InstallFromNexusDialogProps = {
	onReady: (modId: number, fileId: number) => void;
	onOpenSettings: () => void;
	onCancel: () => void;
};

function continueNexusFile(
	account: main.NexusAccountState,
	link: main.NexusLink,
	fileId: number,
): "ready" | "needs-website" {
	if (account.isPremium) return "ready";
	if (link.fileId === fileId && !link.needsWebsite) return "ready";
	return "needs-website";
}

export function InstallFromNexusDialog({
	onReady,
	onOpenSettings,
	onCancel,
}: InstallFromNexusDialogProps) {
	const [account, setAccount] = useState<main.NexusAccountState | null>(null);
	const [url, setUrl] = useState("");
	const [link, setLink] = useState<main.NexusLink | null>(null);
	const [selectedFileId, setSelectedFileId] = useState<number | null>(null);
	const [waitingPage, setWaitingPage] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const [errorMessage, setErrorMessage] = useState("");
	const inputRef = useRef<HTMLInputElement>(null);

	useEffect(() => {
		let cancelled = false;
		void NexusAccount()
			.then((next) => {
				if (!cancelled) setAccount(next);
			})
			.catch((error) => {
				if (!cancelled) setErrorMessage(formatNexusInstallError(error));
			});
		return () => {
			cancelled = true;
		};
	}, []);

	useEffect(() => {
		if (account?.verified) {
			inputRef.current?.focus();
		}
	}, [account]);

	const step = waitingPage
		? "needs-website"
		: account === null
			? "loading"
			: nexusInstallStep(account, link);

	function openWaiting(modId: number, fileId: number) {
		const page = nexusModPageURL(modId, fileId);
		setWaitingPage(page);
		BrowserOpenURL(page);
	}

	function proceed(nextLink: main.NexusLink, fileId: number) {
		if (!account || !nextLink.modId) return;
		if (continueNexusFile(account, nextLink, fileId) === "needs-website") {
			openWaiting(nextLink.modId, fileId);
			return;
		}
		onReady(nextLink.modId, fileId);
	}

	async function resolvePage() {
		if (busy) return;
		const parsed = parseNexusModPageInput(url);
		if (!parsed.ok) {
			setErrorMessage(parsed.error);
			return;
		}

		setBusy(true);
		setErrorMessage("");
		try {
			const next = await ResolveNexusModPage(parsed.url);
			setLink(next);
			const files = next.files ?? [];
			const fileId = next.fileId && next.fileId > 0 ? next.fileId : defaultNexusFileId(files);
			setSelectedFileId(fileId);
			const nextStep = nexusInstallStep(account, next);
			if (nextStep === "ready" && next.modId && fileId) {
				onReady(next.modId, fileId);
				return;
			}
			if (nextStep === "needs-website" && next.modId && fileId) {
				openWaiting(next.modId, fileId);
				return;
			}
			if (nextStep === "no-files") {
				setErrorMessage("This mod has no installable MAIN or OPTIONAL files.");
			}
		} catch (error) {
			setErrorMessage(formatNexusInstallError(error));
		} finally {
			setBusy(false);
		}
	}

	function confirmPickedFile() {
		if (!link?.modId || selectedFileId === null) {
			setErrorMessage("Select a file.");
			return;
		}
		proceed(link, selectedFileId);
	}

	const files = link?.files ?? [];
	const grouped = groupNexusFiles(files);
	const adultBlocked = errorMessage === nexusAdultContentBlockedMessage;

	return (
		<Dialog
			open
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				if (!nextOpen && !busy) onCancel();
			}}
		>
			<DialogContent>
				<div>
					<p className="eyebrow">Cratebug</p>
					<DialogTitle>Install from Nexus Mods</DialogTitle>
				</div>

				{step === "loading" && !errorMessage ? (
					<p className={styles["nexus-install-copy"]}>Checking Nexus account...</p>
				) : null}

				{step === "connect" ? (
					<p className={styles["nexus-install-copy"]}>
						Connect a Nexus Mods API key in Settings before installing from Nexus.
					</p>
				) : null}

				{step === "paste" || (step === "pick-file" && !waitingPage) ? (
					<form
						className="grid gap-5"
						onSubmit={(event) => {
							event.preventDefault();
							if (step === "pick-file") {
								confirmPickedFile();
								return;
							}
							void resolvePage();
						}}
					>
						{step === "paste" ? (
							<label
								className="grid gap-1.5 text-xs font-bold text-muted-foreground"
								htmlFor="install-nexus-url"
							>
								<span>Nexus Mods page URL</span>
								<input
									ref={inputRef}
									id="install-nexus-url"
									type="text"
									className="min-w-0 px-3 py-2.5"
									value={url}
									placeholder="https://www.nexusmods.com/marvelrivals/mods/..."
									spellCheck={false}
									disabled={busy}
									onChange={(event) => {
										setUrl(event.target.value);
										setErrorMessage("");
									}}
								/>
							</label>
						) : null}

						{step === "pick-file" ? (
							<div
								className={styles["nexus-file-groups"]}
								role="radiogroup"
								aria-label="Nexus files"
							>
								{grouped.main.length > 0 ? (
									<div className={styles["nexus-file-group"]}>
										<h3>Main</h3>
										{grouped.main.map((file) => (
											<label
												key={file.fileId}
												className={styles["nexus-file-option"]}
											>
												<input
													type="radio"
													name="nexus-file"
													checked={selectedFileId === file.fileId}
													onChange={() => setSelectedFileId(file.fileId)}
												/>
												<span>
													<span className={styles["nexus-file-name"]}>
														{file.name}
													</span>
													<span className={styles["nexus-file-meta"]}>
														{file.version ? `${file.version} · ` : ""}
														{formatNexusFileSize(file.sizeKb)}
													</span>
												</span>
											</label>
										))}
									</div>
								) : null}
								{grouped.optional.length > 0 ? (
									<div className={styles["nexus-file-group"]}>
										<h3>Optional</h3>
										{grouped.optional.map((file) => (
											<label
												key={file.fileId}
												className={styles["nexus-file-option"]}
											>
												<input
													type="radio"
													name="nexus-file"
													checked={selectedFileId === file.fileId}
													onChange={() => setSelectedFileId(file.fileId)}
												/>
												<span>
													<span className={styles["nexus-file-name"]}>
														{file.name}
													</span>
													<span className={styles["nexus-file-meta"]}>
														{file.version ? `${file.version} · ` : ""}
														{formatNexusFileSize(file.sizeKb)}
													</span>
												</span>
											</label>
										))}
									</div>
								) : null}
							</div>
						) : null}

						{errorMessage ? (
							<p
								className="text-[0.8rem] leading-snug font-semibold text-destructive"
								role="alert"
							>
								{errorMessage}
							</p>
						) : null}

						<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
							<Button
								size="action"
								type="button"
								variant="secondary"
								onClick={onCancel}
								disabled={busy}
							>
								Cancel
							</Button>
							{adultBlocked ? (
								<Button
									size="action"
									type="button"
									onClick={() => BrowserOpenURL(nexusContentBlockingURL)}
								>
									Open Content Blocking
								</Button>
							) : null}
							<Button
								size="action"
								type="submit"
								disabled={busy || (step === "pick-file" && selectedFileId === null)}
							>
								{busy ? (
									<>
										<Loader2 className="spinning-loader" aria-hidden="true" />
										<span>Looking up mod...</span>
									</>
								) : step === "pick-file" ? (
									"Continue"
								) : (
									"Look up mod"
								)}
							</Button>
						</div>
					</form>
				) : null}

				{step === "needs-website" ? (
					<div className={styles["nexus-waiting"]}>
						<p className={styles["nexus-install-copy"]}>
							Free Nexus accounts have to start the download on the website. Click{" "}
							<strong>Mod Manager Download</strong> for this file. Cratebug will pick
							up the link.
						</p>
						{waitingPage ? (
							<button
								type="button"
								className={styles["nexus-link-button"]}
								onClick={() => BrowserOpenURL(waitingPage)}
							>
								Open the Nexus page again
							</button>
						) : null}
						{errorMessage ? (
							<p
								className="text-[0.8rem] leading-snug font-semibold text-destructive"
								role="alert"
							>
								{errorMessage}
							</p>
						) : null}
						<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
							<Button
								size="action"
								type="button"
								variant="secondary"
								onClick={onCancel}
							>
								Cancel
							</Button>
						</div>
					</div>
				) : null}

				{step === "connect" || (step === "loading" && errorMessage) ? (
					<>
						{errorMessage ? (
							<p
								className="text-[0.8rem] leading-snug font-semibold text-destructive"
								role="alert"
							>
								{errorMessage}
							</p>
						) : null}
						<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
							<Button
								size="action"
								type="button"
								variant="secondary"
								onClick={onCancel}
							>
								Close
							</Button>
							<Button size="action" type="button" onClick={onOpenSettings}>
								Open Settings
							</Button>
						</div>
					</>
				) : null}

				{step === "no-files" ? (
					<>
						{errorMessage ? (
							<p
								className="text-[0.8rem] leading-snug font-semibold text-destructive"
								role="alert"
							>
								{errorMessage}
							</p>
						) : (
							<p className={styles["nexus-install-copy"]}>
								This mod has no installable MAIN or OPTIONAL files.
							</p>
						)}
						<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
							<Button
								size="action"
								type="button"
								variant="secondary"
								onClick={onCancel}
							>
								Close
							</Button>
						</div>
					</>
				) : null}
			</DialogContent>
		</Dialog>
	);
}
