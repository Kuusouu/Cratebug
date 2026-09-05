export function renameValidationError(
	name: string,
	subject: "mod" | "folder" = "mod",
): string | null {
	if (name.trim() === "") return `Enter a ${subject} name.`;
	if (name.endsWith(" ") || name.endsWith("."))
		return `A ${subject} name cannot end with a space or period.`;
	if (hasWindowsReservedCharacter(name))
		return `A ${subject} name contains a Windows-reserved character.`;

	return null;
}

function hasWindowsReservedCharacter(name: string): boolean {
	return /[<>:"/\\|?*]/.test(name) || [...name].some((character) => character.charCodeAt(0) < 32);
}
