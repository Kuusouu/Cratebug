//go:build !windows

package urlscheme

import "errors"

type userHive struct{}

func (userHive) read(string) (Snapshot, bool, error) { return Snapshot{}, false, nil }
func (userHive) write(string, Snapshot) error {
	return errors.New("urlscheme: protocol registration is Windows-only")
}
func (userHive) delete(string) error { return nil }
func (userHive) exists(string) (bool, error) {
	return false, nil
}

type machineHive struct{}

func (machineHive) read(string) (Snapshot, bool, error) { return Snapshot{}, false, nil }
func (machineHive) write(string, Snapshot) error {
	return errors.New("urlscheme: refusing to write HKLM")
}
func (machineHive) delete(string) error {
	return errors.New("urlscheme: refusing to delete HKLM")
}
func (machineHive) exists(string) (bool, error) { return false, nil }

func readUserChoice(string) (bool, error) { return false, nil }

func notifyAssocChanged() {}
