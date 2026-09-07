package urlscheme

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandExecutablePath(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "quoted with spaces",
			command: `"C:\Program Files\Cratebug\Cratebug.exe" "%1"`,
			want:    `C:\Program Files\Cratebug\Cratebug.exe`,
		},
		{
			name:    "bare",
			command: `C:\Cratebug\Cratebug.exe "%1"`,
			want:    `C:\Cratebug\Cratebug.exe`,
		},
		{
			name:    "quoted with flags",
			command: `"C:\Program Files\Vortex\Vortex.exe" --download "%1"`,
			want:    `C:\Program Files\Vortex\Vortex.exe`,
		},
		{
			name:    "empty",
			command: "",
			want:    "",
		},
		{
			name:    "unbalanced quote",
			command: `"C:\Program Files\Cratebug\Cratebug.exe %1`,
			want:    "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Act
			got := commandExecutablePath(test.command)

			// Assert
			if got != test.want {
				t.Errorf("commandExecutablePath(%q) = %q, want %q", test.command, got, test.want)
			}
		})
	}
}

func TestStatusOwnershipCases(t *testing.T) {
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`

	tests := []struct {
		name        string
		seedUser    *Snapshot
		seedMachine bool
		userChoice  bool
		existing    map[string]bool
		want        Ownership
		wantMachine bool
		wantChoice  bool
	}{
		{name: "none", want: OwnershipNone},
		{
			name:     "self",
			seedUser: ownHandler(selfExe, "cratebug-test-unit"),
			existing: map[string]bool{selfExe: true},
			want:     OwnershipSelf,
		},
		{
			name:     "selfStale",
			seedUser: ownHandler(selfExe, "cratebug-test-unit"),
			existing: map[string]bool{},
			want:     OwnershipSelfStale,
		},
		{
			name:     "other",
			seedUser: ownHandler(otherExe, "cratebug-test-unit"),
			existing: map[string]bool{otherExe: true},
			want:     OwnershipOther,
		},
		{
			name:       "other with UserChoice",
			seedUser:   ownHandler(otherExe, "cratebug-test-unit"),
			userChoice: true,
			existing:   map[string]bool{otherExe: true},
			want:       OwnershipOther,
			wantChoice: true,
		},
		{
			name:        "machine-wide only",
			seedMachine: true,
			want:        OwnershipNone,
			wantMachine: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			r, user, machine := testRegistrar(t, selfExe)
			if test.seedUser != nil {
				if err := user.write(r.Scheme, *test.seedUser); err != nil {
					t.Fatal(err)
				}
			}
			if test.seedMachine {
				if err := machine.write(r.Scheme, *ownHandler(otherExe, r.Scheme)); err != nil {
					t.Fatal(err)
				}
			}
			r.userChoice = func(string) (bool, error) { return test.userChoice, nil }
			r.fileExists = func(path string) bool { return test.existing[path] }

			// Act
			status, err := r.Status()

			// Assert
			if err != nil {
				t.Fatalf("Status() = %v", err)
			}
			if status.Ownership != test.want {
				t.Errorf("Ownership = %q, want %q", status.Ownership, test.want)
			}
			if status.MachineWide != test.wantMachine {
				t.Errorf("MachineWide = %v, want %v", status.MachineWide, test.wantMachine)
			}
			if status.UserChoice != test.wantChoice {
				t.Errorf("UserChoice = %v, want %v", status.UserChoice, test.wantChoice)
			}
		})
	}
}

func TestRegisterRecordsPreviousOwnerWithExtraSubkeys(t *testing.T) {
	// Arrange
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`
	r, user, _ := testRegistrar(t, selfExe)
	previous := Snapshot{
		Command:     handlerCommand(otherExe),
		Icon:        handlerIcon(otherExe),
		Description: "URL:cratebug-test-unit Protocol",
		Values: []Value{
			{Data: "URL:cratebug-test-unit Protocol"},
			{Name: urlProtocolValue},
			{SubKey: defaultIconSubKey, Data: handlerIcon(otherExe)},
			{SubKey: commandSubKey, Data: handlerCommand(otherExe)},
			{SubKey: `custom\extra`, Name: "Note", Data: "keep-me"},
		},
	}
	if err := user.write(r.Scheme, previous); err != nil {
		t.Fatal(err)
	}
	r.fileExists = func(path string) bool { return path == otherExe || path == selfExe }

	// Act
	displaced, err := r.Register(true)

	// Assert
	if err != nil {
		t.Fatalf("Register() = %v", err)
	}
	if !containsValue(displaced.Values, Value{SubKey: `custom\extra`, Name: "Note", Data: "keep-me"}) {
		t.Fatalf("displaced snapshot missing extra subkey: %+v", displaced.Values)
	}
	if displaced.Command != previous.Command {
		t.Errorf("displaced.Command = %q, want %q", displaced.Command, previous.Command)
	}
	status, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Ownership != OwnershipSelf {
		t.Errorf("Ownership after take-over = %q, want self", status.Ownership)
	}
}

func TestUnregisterLeavesAForeignOwnerInPlace(t *testing.T) {
	// Arrange
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`
	r, user, _ := testRegistrar(t, selfExe)
	foreign := *ownHandler(otherExe, r.Scheme)
	if err := user.write(r.Scheme, foreign); err != nil {
		t.Fatal(err)
	}
	r.fileExists = func(path string) bool { return path == otherExe }

	// Act
	if err := r.Unregister(Snapshot{}); err != nil {
		t.Fatalf("Unregister() = %v", err)
	}

	// Assert
	status, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Ownership != OwnershipOther {
		t.Fatalf("Ownership = %q, want other", status.Ownership)
	}
	if status.Snapshot.Command != foreign.Command {
		t.Errorf("Command = %q, want the foreign handler left in place", status.Snapshot.Command)
	}
}

func TestUnregisterRestoresValueForValue(t *testing.T) {
	// Arrange
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`
	r, _, _ := testRegistrar(t, selfExe)
	r.fileExists = func(path string) bool { return path == selfExe || path == otherExe }
	previous := Snapshot{
		Command:     handlerCommand(otherExe),
		Icon:        handlerIcon(otherExe),
		Description: "URL:cratebug-test-unit Protocol",
		Values: []Value{
			{Data: "URL:cratebug-test-unit Protocol"},
			{Name: urlProtocolValue},
			{SubKey: defaultIconSubKey, Data: handlerIcon(otherExe)},
			{SubKey: commandSubKey, Data: handlerCommand(otherExe)},
			{SubKey: `custom\extra`, Name: "Note", Data: "keep-me"},
		},
	}
	if _, err := r.Register(false); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := r.Unregister(previous); err != nil {
		t.Fatalf("Unregister() = %v", err)
	}

	// Assert
	status, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Ownership != OwnershipOther {
		t.Fatalf("Ownership = %q, want other after restore", status.Ownership)
	}
	if !containsValue(status.Snapshot.Values, Value{SubKey: `custom\extra`, Name: "Note", Data: "keep-me"}) {
		t.Fatalf("restored snapshot missing extra value: %+v", status.Snapshot.Values)
	}
	if status.Snapshot.Command != previous.Command {
		t.Errorf("Command = %q, want %q", status.Snapshot.Command, previous.Command)
	}
}

func TestRegisterWritesQuotedCommandForPathWithSpace(t *testing.T) {
	// Arrange
	exe := `C:\Program Files\Cratebug\Cratebug.exe`
	r, _, _ := testRegistrar(t, exe)
	r.fileExists = func(path string) bool { return path == exe }

	// Act
	if _, err := r.Register(false); err != nil {
		t.Fatalf("Register() = %v", err)
	}

	// Assert
	status, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := `"C:\Program Files\Cratebug\Cratebug.exe" "%1"`
	if status.Snapshot.Command != want {
		t.Errorf("Command = %q, want %q", status.Snapshot.Command, want)
	}
}

func TestRegisterWithoutTakeOverLeavesTheOtherOwner(t *testing.T) {
	// Arrange
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`
	r, user, _ := testRegistrar(t, selfExe)
	foreign := *ownHandler(otherExe, r.Scheme)
	if err := user.write(r.Scheme, foreign); err != nil {
		t.Fatal(err)
	}
	r.fileExists = func(path string) bool { return path == otherExe }

	// Act
	_, err := r.Register(false)

	// Assert
	if !errors.Is(err, ErrTaken) {
		t.Fatalf("Register() error = %v, want ErrTaken", err)
	}
	status, statusErr := r.Status()
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Snapshot.Command != foreign.Command {
		t.Errorf("Command = %q, want the foreign handler left in place", status.Snapshot.Command)
	}
}

func TestDeleteKeyTreeVisitsDeepestFirst(t *testing.T) {
	// Arrange
	children := map[string][]string{
		"root":     {"a", "b"},
		`root\a`:   {"c"},
		`root\a\c`: nil,
		`root\b`:   nil,
	}
	var order []string

	// Act
	err := deleteKeyTree(
		func(path string) ([]string, error) { return children[path], nil },
		func(path string) error {
			order = append(order, path)
			return nil
		},
		"root",
	)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !isDeepestFirst(order, children) {
		t.Fatalf("delete order %v is not deepest-first", order)
	}
	if order[len(order)-1] != "root" {
		t.Fatalf("root was not deleted last: %v", order)
	}
}

func testRegistrar(t *testing.T, exe string) (*Registrar, *memoryHive, *memoryHive) {
	t.Helper()
	if strings.EqualFold(filepath.Base(exe), SchemeNXM) {
		t.Fatal("test executable basename must not be nxm")
	}
	user := newMemoryHive()
	machine := newMemoryHive()
	r := &Registrar{
		Scheme:     "cratebug-test-unit",
		Exe:        exe,
		user:       user,
		machine:    machine,
		userChoice: func(string) (bool, error) { return false, nil },
		fileExists: func(string) bool { return false },
		notify:     func() {},
	}
	if r.Scheme == SchemeNXM {
		t.Fatal("unit tests must not use the nxm scheme")
	}
	return r, user, machine
}

func ownHandler(exe, scheme string) *Snapshot {
	snapshot := (&Registrar{Scheme: scheme, Exe: exe}).ownSnapshot()
	return &snapshot
}

func containsValue(values []Value, want Value) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func isDeepestFirst(order []string, children map[string][]string) bool {
	index := make(map[string]int, len(order))
	for i, path := range order {
		index[path] = i
	}
	for parent, kids := range children {
		parentIndex, ok := index[parent]
		if !ok {
			return false
		}
		for _, kid := range kids {
			child := parent + `\` + kid
			childIndex, ok := index[child]
			if !ok || childIndex > parentIndex {
				return false
			}
		}
	}
	return true
}
