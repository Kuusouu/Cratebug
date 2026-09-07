package urlscheme

import (
	"strings"
	"sync"
)

const (
	urlProtocolValue  = "URL Protocol"
	defaultIconSubKey = "DefaultIcon"
	commandSubKey     = `shell\open\command`
	classesPrefix     = `Software\Classes`
)

// Deletes path after every descendant. Children are visited first so a
// registry key is empty before DeleteKey runs.
func deleteKeyTree(list func(path string) ([]string, error), remove func(path string) error, path string) error {
	names, err := list(path)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := deleteKeyTree(list, remove, path+`\`+name); err != nil {
			return err
		}
	}
	return remove(path)
}

type memoryNode struct {
	values   map[string]string
	children map[string]*memoryNode
}

type memoryHive struct {
	mu      sync.Mutex
	root    *memoryNode
	deleted []string
}

func newMemoryHive() *memoryHive {
	return &memoryHive{root: newMemoryNode()}
}

func newMemoryNode() *memoryNode {
	return &memoryNode{
		values:   make(map[string]string),
		children: make(map[string]*memoryNode),
	}
}

func (h *memoryHive) read(scheme string) (Snapshot, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	node := h.lookup(scheme)
	if node == nil {
		return Snapshot{}, false, nil
	}
	var values []Value
	collectMemoryValues(node, "", &values)
	return snapshotFromValues(values), true, nil
}

func (h *memoryHive) write(scheme string, snapshot Snapshot) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeNodeLocked(scheme)
	for _, value := range snapshot.values() {
		path := scheme
		if value.SubKey != "" {
			path = scheme + `\` + value.SubKey
		}
		node := h.ensure(path)
		node.values[value.Name] = value.Data
	}
	return nil
}

func (h *memoryHive) delete(scheme string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeNodeLocked(scheme)
	return nil
}

func (h *memoryHive) exists(scheme string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lookup(scheme) != nil, nil
}

func (h *memoryHive) lookup(path string) *memoryNode {
	node := h.root
	for _, part := range splitKey(path) {
		next, ok := node.children[part]
		if !ok {
			return nil
		}
		node = next
	}
	return node
}

func (h *memoryHive) ensure(path string) *memoryNode {
	node := h.root
	for _, part := range splitKey(path) {
		next, ok := node.children[part]
		if !ok {
			next = newMemoryNode()
			node.children[part] = next
		}
		node = next
	}
	return node
}

func (h *memoryHive) removeNodeLocked(path string) {
	_ = deleteKeyTree(
		func(keyPath string) ([]string, error) {
			node := h.lookup(keyPath)
			if node == nil {
				return nil, nil
			}
			names := make([]string, 0, len(node.children))
			for name := range node.children {
				names = append(names, name)
			}
			return names, nil
		},
		func(keyPath string) error {
			h.deleted = append(h.deleted, keyPath)
			parentPath, name := splitLast(keyPath)
			parent := h.root
			if parentPath != "" {
				parent = h.lookup(parentPath)
				if parent == nil {
					return nil
				}
			}
			delete(parent.children, name)
			return nil
		},
		path,
	)
}

func collectMemoryValues(node *memoryNode, subKey string, into *[]Value) {
	for name, data := range node.values {
		*into = append(*into, Value{SubKey: subKey, Name: name, Data: data})
	}
	for name, child := range node.children {
		next := name
		if subKey != "" {
			next = subKey + `\` + name
		}
		collectMemoryValues(child, next, into)
	}
}

func snapshotFromValues(values []Value) Snapshot {
	snapshot := Snapshot{Values: values}
	for _, value := range values {
		if value.Name != "" {
			continue
		}
		switch value.SubKey {
		case "":
			snapshot.Description = value.Data
		case defaultIconSubKey:
			snapshot.Icon = value.Data
		case commandSubKey:
			snapshot.Command = value.Data
		}
	}
	return snapshot
}

func splitKey(path string) []string {
	path = strings.Trim(path, `\`)
	if path == "" {
		return nil
	}
	return strings.Split(path, `\`)
}

func splitLast(path string) (parent, name string) {
	path = strings.Trim(path, `\`)
	i := strings.LastIndex(path, `\`)
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}
