package mutation

import "strings"

const marvelRivalsExecutable = "marvel-win64-shipping.exe"

// Keeps the established executable identity easy to test without a live process.
func isMarvelRivalsProcess(processName string) bool {
	return strings.EqualFold(processName, marvelRivalsExecutable)
}
