//go:build windows

package urlscheme

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	shcneAssocChanged = 0x08000000
	shcnfIDList       = 0x0000
)

var (
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	procSHChangeNotify = shell32.NewProc("SHChangeNotify")
)

type userHive struct{}

func (userHive) read(scheme string) (Snapshot, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, classesPath(scheme), registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		if err == registry.ErrNotExist {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("open protocol key: %w", err)
	}
	defer key.Close()

	var values []Value
	if err := collectRegistryValues(key, "", &values); err != nil {
		return Snapshot{}, false, err
	}
	return snapshotFromValues(values), true, nil
}

func (userHive) write(scheme string, snapshot Snapshot) error {
	for _, value := range snapshot.values() {
		path := classesPath(scheme)
		if value.SubKey != "" {
			path = path + `\` + value.SubKey
		}
		key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("create protocol key: %w", err)
		}
		if err := key.SetStringValue(value.Name, value.Data); err != nil {
			key.Close()
			return fmt.Errorf("write protocol value: %w", err)
		}
		key.Close()
	}
	return nil
}

func (userHive) delete(scheme string) error {
	return deleteRegistryTree(registry.CURRENT_USER, classesPath(scheme))
}

func (userHive) exists(scheme string) (bool, error) {
	return keyExists(registry.CURRENT_USER, classesPath(scheme))
}

type machineHive struct{}

func (machineHive) read(string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}

func (machineHive) write(string, Snapshot) error {
	return fmt.Errorf("urlscheme: refusing to write HKLM")
}

func (machineHive) delete(string) error {
	return fmt.Errorf("urlscheme: refusing to delete HKLM")
}

func (machineHive) exists(scheme string) (bool, error) {
	// HKLM is read-only here. Access-denied or a missing Classes hive is
	// "not visible", not a failure: Status must still report HKCU ownership.
	return keyExistsOptional(registry.LOCAL_MACHINE, classesPath(scheme)), nil
}

func readUserChoice(scheme string) (bool, error) {
	path := `Software\Microsoft\Windows\Shell\Associations\UrlAssociations\` + scheme + `\UserChoice`
	return keyExistsOptional(registry.CURRENT_USER, path), nil
}

func notifyAssocChanged() {
	_, _, _ = procSHChangeNotify.Call(shcneAssocChanged, shcnfIDList, 0, 0)
}

func classesPath(scheme string) string {
	return filepath.Join(classesPrefix, scheme)
}

func keyExists(root registry.Key, path string) (bool, error) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	key.Close()
	return true, nil
}

func keyExistsOptional(root registry.Key, path string) bool {
	exists, err := keyExists(root, path)
	return err == nil && exists
}

func collectRegistryValues(key registry.Key, subKey string, into *[]Value) error {
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return fmt.Errorf("list protocol values: %w", err)
	}
	for _, name := range names {
		data, _, err := key.GetStringValue(name)
		if err != nil {
			continue
		}
		*into = append(*into, Value{SubKey: subKey, Name: name, Data: data})
	}

	subkeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return fmt.Errorf("list protocol subkeys: %w", err)
	}
	for _, name := range subkeys {
		child, err := registry.OpenKey(key, name, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			return fmt.Errorf("open protocol subkey: %w", err)
		}
		next := name
		if subKey != "" {
			next = subKey + `\` + name
		}
		err = collectRegistryValues(child, next, into)
		child.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteRegistryTree(root registry.Key, path string) error {
	return deleteKeyTree(
		func(keyPath string) ([]string, error) {
			key, err := registry.OpenKey(root, keyPath, registry.ENUMERATE_SUB_KEYS)
			if err != nil {
				if err == registry.ErrNotExist {
					return nil, nil
				}
				return nil, err
			}
			names, err := key.ReadSubKeyNames(-1)
			key.Close()
			return names, err
		},
		func(keyPath string) error {
			parent, name := splitLast(keyPath)
			parentKey := root
			if parent != "" {
				opened, err := registry.OpenKey(root, parent, registry.WRITE)
				if err != nil {
					if err == registry.ErrNotExist {
						return nil
					}
					return err
				}
				defer opened.Close()
				parentKey = opened
			}
			err := registry.DeleteKey(parentKey, name)
			if err == registry.ErrNotExist {
				return nil
			}
			return err
		},
		path,
	)
}
