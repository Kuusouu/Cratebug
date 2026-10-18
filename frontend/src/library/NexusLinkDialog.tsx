import { Button } from "@/components/ui/button";
import { useEffect, useRef } from "react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";
import { formatNexusFileSize, formatNexusLinkTitle, nexusModPageURL } from "./nexusPresentation";

type NexusLinkDialogProps = {
	link: main.NexusLink;
	libraryReady: boolean;
	onConfirm: () => void;
	onCancel: () => void;
	onOpenSettings: () => void;
};

export function NexusLinkDialog({
	link,
	libraryReady,
	onConfirm,
	onCancel,
	onOpenSettings,
}: NexusLinkDialogProps) {
	const confirmRef = useRef<HTMLButtonElement>(null);
	const title = formatNexusLinkTitle(link);
	const hasFile = (link.fileId ?? 0) > 0;
	const needsAccount = !link.modName && !link.fileName;
	const canInstall = libraryReady && hasFile && !link.needsWebsite && !needsAccount;

	useEffect(() => {
		confirmRef.current?.focus();
	}, []);

	const sizeLabel = link.sizeKb && link.sizeKb > 0 ? formatNexusFileSize(link.sizeKb) : null;
	const details = [link.fileName, link.version, sizeLabel, link.author]
		.filter((part): part is string => Boolean(part && part.trim() !== ""))
		.join(" · ");

	return (
		<Dialog
			open
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				if (!nextOpen) onCancel();
			}}
		>
			<DialogContent>
				<DialogHeader>
					<p className="text-[0.7rem] font-extrabold tracking-[0.12em] text-primary uppercase">
						Nexus Mods
					</p>
					<DialogTitle>Install this download?</DialogTitle>
				</DialogHeader>
				<div className="grid gap-[0.35rem]">
					<DialogDescription className="text-[0.95rem] font-bold text-foreground">
						{title}
					</DialogDescription>
					{details ? (
						<p className="text-[0.82rem] leading-[1.45] text-muted-foreground">
							{details}
						</p>
					) : null}
				</div>
				{needsAccount ? (
					<p className="text-[0.85rem] leading-normal text-muted-foreground">
						Connect a Nexus Mods API key in Settings to confirm this download.
					</p>
				) : null}
				{link.needsWebsite ? (
					<p className="text-[0.85rem] leading-normal text-muted-foreground">
						This link still needs a Mod Manager Download click on the Nexus website.
					</p>
				) : null}
				{!libraryReady ? (
					<p className="text-[0.85rem] leading-normal text-muted-foreground">
						Set a mod library folder before installing.
					</p>
				) : null}
				{!hasFile ? (
					<p className="text-[0.8rem] font-semibold text-destructive" role="alert">
						That Nexus link is missing a file.
					</p>
				) : null}
				<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
					<Button size="action" type="button" variant="secondary" onClick={onCancel}>
						Cancel
					</Button>
					{needsAccount ? (
						<Button size="action" type="button" onClick={onOpenSettings}>
							Open Settings
						</Button>
					) : null}
					{link.needsWebsite && link.modId ? (
						<Button
							size="action"
							type="button"
							onClick={() =>
								BrowserOpenURL(nexusModPageURL(link.modId ?? 0, link.fileId))
							}
						>
							Open Nexus page
						</Button>
					) : null}
					<Button
						size="action"
						ref={confirmRef}
						type="button"
						disabled={!canInstall}
						onClick={onConfirm}
					>
						Download &amp; install
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
