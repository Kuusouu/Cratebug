//go:build linux

package reveal

import (
	"errors"
	"strings"
	"testing"
)

func TestOpenPathCallsDBusShowItemsWhenSelectItemIsTrue(t *testing.T) {
	// Arrange
	origLookPath := lookPath
	origDbusRunner := dbusRunner
	defer func() {
		lookPath = origLookPath
		dbusRunner = origDbusRunner
	}()

	lookPath = func(file string) (string, error) {
		if file == "gdbus" {
			return "/usr/bin/gdbus", nil
		}
		return "", errors.New("not found")
	}

	var invokedCommand string
	var invokedArgs []string
	dbusRunner = func(name string, args ...string) error {
		invokedCommand = name
		invokedArgs = args
		return nil
	}

	target := Target{
		Path:       "/home/user/mods/test.pak",
		SelectItem: true,
	}

	// Act
	err := openPath(target)

	// Assert
	if err != nil {
		t.Fatalf("openPath unexpected error: %v", err)
	}
	if invokedCommand != "/usr/bin/gdbus" {
		t.Errorf("invoked command = %q, want /usr/bin/gdbus", invokedCommand)
	}
	argsStr := strings.Join(invokedArgs, " ")
	if !strings.Contains(argsStr, "ShowItems") {
		t.Errorf("expected ShowItems in args, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "file:///home/user/mods/test.pak") {
		t.Errorf("expected file URI in args, got: %s", argsStr)
	}
}

func TestOpenPathFallsBackToXdgOpenWhenDBusFails(t *testing.T) {
	// Arrange
	origLookPath := lookPath
	origDbusRunner := dbusRunner
	origOpenRunner := openRunner
	defer func() {
		lookPath = origLookPath
		dbusRunner = origDbusRunner
		openRunner = origOpenRunner
	}()

	lookPath = func(file string) (string, error) {
		return "", errors.New("tool not found")
	}

	var invokedCommand string
	var invokedArgs []string
	openRunner = func(name string, args ...string) error {
		invokedCommand = name
		invokedArgs = args
		return nil
	}

	target := Target{
		Path:       "/home/user/mods/test.pak",
		SelectItem: true,
	}

	// Act
	err := openPath(target)

	// Assert
	if err != nil {
		t.Fatalf("openPath unexpected error: %v", err)
	}
	if invokedCommand != "xdg-open" {
		t.Errorf("invoked command = %q, want xdg-open", invokedCommand)
	}
	if len(invokedArgs) != 1 || invokedArgs[0] != "/home/user/mods" {
		t.Errorf("invoked args = %v, want [/home/user/mods]", invokedArgs)
	}
}

func TestOpenPathOpensDirectoryDirectlyWhenSelectItemIsFalse(t *testing.T) {
	// Arrange
	origOpenRunner := openRunner
	defer func() {
		openRunner = origOpenRunner
	}()

	var invokedCommand string
	var invokedArgs []string
	openRunner = func(name string, args ...string) error {
		invokedCommand = name
		invokedArgs = args
		return nil
	}

	target := Target{
		Path:       "/home/user/mods/Characters",
		SelectItem: false,
	}

	// Act
	err := openPath(target)

	// Assert
	if err != nil {
		t.Fatalf("openPath unexpected error: %v", err)
	}
	if invokedCommand != "xdg-open" {
		t.Errorf("invoked command = %q, want xdg-open", invokedCommand)
	}
	if len(invokedArgs) != 1 || invokedArgs[0] != "/home/user/mods/Characters" {
		t.Errorf("invoked args = %v, want [/home/user/mods/Characters]", invokedArgs)
	}
}

func TestOpenPathEscapesSingleQuotesInDBusShowItems(t *testing.T) {
	// Arrange
	origLookPath := lookPath
	origDbusRunner := dbusRunner
	defer func() {
		lookPath = origLookPath
		dbusRunner = origDbusRunner
	}()

	lookPath = func(file string) (string, error) {
		if file == "gdbus" {
			return "/usr/bin/gdbus", nil
		}
		return "", errors.New("not found")
	}

	var invokedArgs []string
	dbusRunner = func(name string, args ...string) error {
		invokedArgs = args
		return nil
	}

	target := Target{
		Path:       "/home/user/mods/Deadpool's Mask.pak",
		SelectItem: true,
	}

	// Act
	err := openPath(target)

	// Assert
	if err != nil {
		t.Fatalf("openPath unexpected error: %v", err)
	}
	argsStr := strings.Join(invokedArgs, " ")
	if !strings.Contains(argsStr, `Deadpool\'s Mask.pak`) {
		t.Errorf("expected escaped single quote in args, got: %s", argsStr)
	}
}

func TestOpenPathSkipsDbusSendWithCommaInPath(t *testing.T) {
	// Arrange
	origLookPath := lookPath
	origOpenRunner := openRunner
	defer func() {
		lookPath = origLookPath
		openRunner = origOpenRunner
	}()

	lookPath = func(file string) (string, error) {
		if file == "dbus-send" {
			return "/usr/bin/dbus-send", nil
		}
		return "", errors.New("not found")
	}

	var openCommand string
	var openArgs []string
	openRunner = func(name string, args ...string) error {
		openCommand = name
		openArgs = args
		return nil
	}

	target := Target{
		Path:       "/home/user/mods/IronMan,Mark7.pak",
		SelectItem: true,
	}

	// Act
	err := openPath(target)

	// Assert
	if err != nil {
		t.Fatalf("openPath unexpected error: %v", err)
	}
	if openCommand != "xdg-open" {
		t.Errorf("openCommand = %q, want xdg-open", openCommand)
	}
	if len(openArgs) != 1 || openArgs[0] != "/home/user/mods" {
		t.Errorf("openArgs = %v, want [/home/user/mods]", openArgs)
	}
}
