import { describe, expect, it } from "bun:test";
import {
	formatNexusRateLimit,
	formatProtocolOwner,
	nexusAccountKind,
	protocolSwitchDisabledReason,
} from "./nexusSettingsPresentation";

describe("nexusAccountKind", () => {
	it("returns disconnected when there is no account", () => {
		expect(nexusAccountKind(null)).toBe("disconnected");
	});

	it("returns unverified when a key file exists but validate failed", () => {
		expect(
			nexusAccountKind({
				configured: true,
				verified: false,
				isPremium: false,
				hourlyRemaining: 0,
				dailyRemaining: 0,
			}),
		).toBe("unverified");
	});

	it("distinguishes premium from free once verified", () => {
		expect(
			nexusAccountKind({
				configured: true,
				verified: true,
				name: "Omar",
				isPremium: true,
				hourlyRemaining: 10,
				dailyRemaining: 100,
			}),
		).toBe("premium");
		expect(
			nexusAccountKind({
				configured: true,
				verified: true,
				name: "Omar",
				isPremium: false,
				hourlyRemaining: 10,
				dailyRemaining: 100,
			}),
		).toBe("free");
	});
});

describe("formatNexusRateLimit", () => {
	it("names both remaining buckets", () => {
		expect(
			formatNexusRateLimit({
				configured: true,
				verified: true,
				isPremium: true,
				hourlyRemaining: 250,
				dailyRemaining: 2500,
			}),
		).toBe("250 hourly / 2500 daily remaining");
	});
});

describe("formatProtocolOwner", () => {
	it("prefers the basename, then the path, then a generic label", () => {
		expect(
			formatProtocolOwner({
				ownership: "other",
				ownerName: "Vortex.exe",
				ownerPath: "C:\\Apps\\Vortex\\Vortex.exe",
				userChoice: false,
				machineWide: false,
				canRegister: true,
				enabled: false,
			}),
		).toBe("Vortex.exe");
		expect(
			formatProtocolOwner({
				ownership: "other",
				ownerPath: "C:\\Apps\\Vortex\\Vortex.exe",
				userChoice: false,
				machineWide: false,
				canRegister: true,
				enabled: false,
			}),
		).toBe("Vortex.exe");
		expect(
			formatProtocolOwner({
				ownership: "other",
				userChoice: false,
				machineWide: false,
				canRegister: true,
				enabled: false,
			}),
		).toBe("another application");
		expect(
			formatProtocolOwner({
				ownership: "other",
				ownerName: ".",
				userChoice: false,
				machineWide: false,
				canRegister: true,
				enabled: false,
			}),
		).toBe("another application");
	});
});

describe("protocolSwitchDisabledReason", () => {
	it("explains a missing status and a dev build", () => {
		expect(protocolSwitchDisabledReason(null)).toBe(
			"The nxm:// handler status is unavailable.",
		);
		expect(
			protocolSwitchDisabledReason({
				ownership: "none",
				userChoice: false,
				machineWide: false,
				canRegister: false,
				enabled: false,
			}),
		).toBe("Dev builds do not register as the nxm:// handler.");
		expect(
			protocolSwitchDisabledReason({
				ownership: "none",
				userChoice: false,
				machineWide: false,
				canRegister: true,
				enabled: false,
			}),
		).toBeNull();
	});
});
