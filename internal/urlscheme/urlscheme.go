// Package urlscheme registers and restores a per-user URL protocol handler.
package urlscheme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The Nexus Mods download scheme. Production code uses this; tests use a
// disposable cratebug-test-* name and must never write nxm.
const SchemeNXM = "nxm"

var (
	// Another application owns the scheme and takeOver was false.
	ErrTaken = errors.New("urlscheme: another application owns this protocol")

	errEmptyScheme   = errors.New("urlscheme: empty scheme")
	errInvalidScheme = errors.New("urlscheme: scheme must be a single name")
)

// Who currently owns the per-user scheme key.
type Ownership string

const (
	OwnershipNone      Ownership = "none"
	OwnershipSelf      Ownership = "self"
	OwnershipSelfStale Ownership = "selfStale"
	OwnershipOther     Ownership = "other"
)

// One string value under the scheme key, identified by a relative subkey
// and value name. The empty name is the key's default value.
type Value struct {
	SubKey string
	Name   string
	Data   string
}

// The values that make up a protocol handler, including extras so
// unregister can restore a previous owner value-for-value.
type Snapshot struct {
	Command     string
	Icon        string
	Description string
	Values      []Value
}

// Reports who owns a scheme and what is currently written.
type Status struct {
	Ownership   Ownership
	Snapshot    Snapshot
	OwnerPath   string
	UserChoice  bool
	MachineWide bool
}

// Reads and writes one scheme under a registry hive.
type schemeHive interface {
	read(scheme string) (Snapshot, bool, error)
	write(scheme string, snapshot Snapshot) error
	delete(scheme string) error
	exists(scheme string) (bool, error)
}

// Inspects and updates one URL scheme. Tests replace the hives so no
// real registry is touched.
type Registrar struct {
	Scheme string
	Exe    string

	user       schemeHive
	machine    schemeHive
	userChoice func(scheme string) (bool, error)
	fileExists func(path string) bool
	notify     func()
}

// Builds a registrar for scheme that points at exe. Production callers
// pass SchemeNXM and the running executable.
func New(scheme, exe string) *Registrar {
	return &Registrar{
		Scheme:     scheme,
		Exe:        exe,
		user:       userHive{},
		machine:    machineHive{},
		userChoice: readUserChoice,
		fileExists: fileExists,
		notify:     notifyAssocChanged,
	}
}

// Builds a registrar against in-memory hives so app-layer tests do not
// touch the real registry.
func NewForTest(scheme, exe string, exists func(string) bool) *Registrar {
	if exists == nil {
		exists = func(string) bool { return true }
	}
	return &Registrar{
		Scheme:     scheme,
		Exe:        exe,
		user:       newMemoryHive(),
		machine:    newMemoryHive(),
		userChoice: func(string) (bool, error) { return false, nil },
		fileExists: exists,
		notify:     func() {},
	}
}

// Reports the current owner of the scheme without writing anything.
func (r *Registrar) Status() (Status, error) {
	if err := validScheme(r.Scheme); err != nil {
		return Status{}, err
	}

	snapshot, ok, err := r.user.read(r.Scheme)
	if err != nil {
		return Status{}, err
	}
	machineWide, err := r.machine.exists(r.Scheme)
	if err != nil {
		return Status{}, err
	}
	userChoice, err := r.userChoice(r.Scheme)
	if err != nil {
		return Status{}, err
	}

	if !ok {
		return Status{
			Ownership:   OwnershipNone,
			UserChoice:  userChoice,
			MachineWide: machineWide,
		}, nil
	}

	path := commandExecutablePath(snapshot.Command)
	status := Status{
		Snapshot:    snapshot,
		OwnerPath:   path,
		UserChoice:  userChoice,
		MachineWide: machineWide,
	}
	if path == "" {
		status.Ownership = OwnershipOther
		return status, nil
	}
	if strings.EqualFold(filepath.Base(path), filepath.Base(r.Exe)) {
		if r.fileExists != nil && r.fileExists(path) {
			status.Ownership = OwnershipSelf
			return status, nil
		}
		status.Ownership = OwnershipSelfStale
		return status, nil
	}
	status.Ownership = OwnershipOther
	return status, nil
}

// Writes this executable as the per-user handler. When another app owns
// the scheme, takeOver must be true; the displaced snapshot is returned
// so the caller can persist it for restore. self and selfStale are
// rewritten without consent. Does not write HKLM or UserChoice.
func (r *Registrar) Register(takeOver bool) (Snapshot, error) {
	if err := validScheme(r.Scheme); err != nil {
		return Snapshot{}, err
	}

	status, err := r.Status()
	if err != nil {
		return Snapshot{}, err
	}

	var displaced Snapshot
	switch status.Ownership {
	case OwnershipOther:
		if !takeOver {
			return status.Snapshot, ErrTaken
		}
		displaced = status.Snapshot
	case OwnershipNone, OwnershipSelf, OwnershipSelfStale:
	default:
		return Snapshot{}, fmt.Errorf("urlscheme: unknown ownership %q", status.Ownership)
	}

	if err := r.user.delete(r.Scheme); err != nil {
		return Snapshot{}, err
	}
	if err := r.user.write(r.Scheme, r.ownSnapshot()); err != nil {
		return Snapshot{}, err
	}
	r.notifyShell()
	return displaced, nil
}

// Removes our handler when we own the scheme and writes previous back
// value-for-value. A foreign owner is left untouched.
func (r *Registrar) Unregister(previous Snapshot) error {
	if err := validScheme(r.Scheme); err != nil {
		return err
	}

	status, err := r.Status()
	if err != nil {
		return err
	}
	if status.Ownership != OwnershipSelf && status.Ownership != OwnershipSelfStale {
		return nil
	}

	if err := r.user.delete(r.Scheme); err != nil {
		return err
	}
	if !previous.Empty() {
		if err := r.user.write(r.Scheme, previous); err != nil {
			return err
		}
	}
	r.notifyShell()
	return nil
}

func (r *Registrar) ownSnapshot() Snapshot {
	command := handlerCommand(r.Exe)
	icon := handlerIcon(r.Exe)
	description := handlerDescription(r.Scheme)
	return Snapshot{
		Command:     command,
		Icon:        icon,
		Description: description,
		Values: []Value{
			{Data: description},
			{Name: urlProtocolValue},
			{SubKey: defaultIconSubKey, Data: icon},
			{SubKey: commandSubKey, Data: command},
		},
	}
}

func (r *Registrar) notifyShell() {
	if r.notify != nil {
		r.notify()
	}
}

func (s Snapshot) Empty() bool {
	return s.Command == "" && s.Icon == "" && s.Description == "" && len(s.Values) == 0
}

func (s Snapshot) values() []Value {
	if len(s.Values) > 0 {
		return s.Values
	}
	var values []Value
	if s.Description != "" {
		values = append(values, Value{Data: s.Description})
	}
	if s.Icon != "" {
		values = append(values, Value{SubKey: defaultIconSubKey, Data: s.Icon})
	}
	if s.Command != "" {
		values = append(values, Value{Name: urlProtocolValue})
		values = append(values, Value{SubKey: commandSubKey, Data: s.Command})
	}
	return values
}

func handlerCommand(exe string) string {
	return `"` + exe + `" "%1"`
}

func handlerIcon(exe string) string {
	return `"` + exe + `"`
}

func handlerDescription(scheme string) string {
	return "URL:" + scheme + " Protocol"
}

// Extracts the executable path from a shell\open\command default value.
func commandExecutablePath(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if command[0] == '"' {
		end := strings.IndexByte(command[1:], '"')
		if end < 0 {
			return ""
		}
		return command[1 : 1+end]
	}
	if i := strings.IndexAny(command, " \t"); i >= 0 {
		return command[:i]
	}
	return command
}

func validScheme(scheme string) error {
	if scheme == "" {
		return errEmptyScheme
	}
	if strings.ContainsAny(scheme, `/\`) {
		return errInvalidScheme
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
