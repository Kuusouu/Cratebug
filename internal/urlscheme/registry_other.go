//go:build !windows && !linux

package urlscheme

import "errors"

type userHive struct{}

func (userHive) read(string) (Snapshot, bool, error) { return Snapshot{}, false, nil }
func (userHive) write(string, Snapshot) error {
	return errors.New("urlscheme: protocol registration is not supported on this platform")
}
func (userHive) delete(string) error { return nil }
func (userHive) exists(string) (bool, error) {
	return false, nil
}

type machineHive struct{}

func (machineHive) read(string) (Snapshot, bool, error) { return Snapshot{}, false, nil }
func (machineHive) write(string, Snapshot) error {
	return errors.New("urlscheme: refusing to write machine-wide")
}
func (machineHive) delete(string) error {
	return errors.New("urlscheme: refusing to delete machine-wide")
}
func (machineHive) exists(string) (bool, error) { return false, nil }

func readUserChoice(string) (bool, error) { return false, nil }

func notifyAssocChanged() {}
