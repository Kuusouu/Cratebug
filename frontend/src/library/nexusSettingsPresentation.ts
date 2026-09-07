export const nexusApiKeyPageURL = "https://www.nexusmods.com/users/myaccount?tab=api";

export type NexusAccountView = {
	configured: boolean;
	verified: boolean;
	name?: string;
	isPremium: boolean;
	hourlyRemaining: number;
	dailyRemaining: number;
};

export type NexusProtocolView = {
	ownership: string;
	ownerName?: string;
	ownerPath?: string;
	userChoice: boolean;
	machineWide: boolean;
	canRegister: boolean;
	enabled: boolean;
};

export type NexusAccountKind = "disconnected" | "unverified" | "premium" | "free";

export function nexusAccountKind(account: NexusAccountView | null): NexusAccountKind {
	if (!account?.configured) return "disconnected";
	if (!account.verified) return "unverified";
	return account.isPremium ? "premium" : "free";
}

export function formatNexusRateLimit(account: NexusAccountView): string {
	return `${account.hourlyRemaining} hourly / ${account.dailyRemaining} daily remaining`;
}

export function formatProtocolOwner(protocol: NexusProtocolView): string {
	if (protocol.ownerName && protocol.ownerName !== ".") return protocol.ownerName;
	if (protocol.ownerPath) {
		const parts = protocol.ownerPath.split(/[/\\]/);
		return parts[parts.length - 1] ?? protocol.ownerPath;
	}
	return "another application";
}

export function protocolSwitchDisabledReason(protocol: NexusProtocolView | null): string | null {
	if (!protocol) return "The nxm:// handler status is unavailable.";
	if (!protocol.canRegister) {
		return "Dev builds do not register as the nxm:// handler.";
	}
	return null;
}
