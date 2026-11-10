package patchupdate

import (
	"fmt"
	"reflect"
	"strings"
)

// Everything about an export that describes its content rather than where
// its bytes sit. Offsets and sizes move whenever a header changes length.
var exportContentKeys = []string{"$type", "ObjectName", "ClassIndex", "Data", "Extras"}

// One worker read of an asset, decoded from its UAssetAPI JSON.
type assetRead struct {
	document    []byte
	unversioned bool
	clean       bool
	classes     []string
	content     []any
}

func parseAssetRead(document []byte) (assetRead, error) {
	root, err := decodeJSON(document)
	if err != nil {
		return assetRead{}, err
	}
	asset, ok := root.(map[string]any)
	if !ok {
		return assetRead{}, fmt.Errorf("patchupdate: asset JSON is not an object")
	}

	read := assetRead{document: document, clean: true}
	read.unversioned, _ = asset["HasUnversionedProperties"].(bool)
	imports, _ := asset["Imports"].([]any)
	r := &remapper{imports: imports}
	exports, _ := asset["Exports"].([]any)
	for _, raw := range exports {
		export, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		// UAssetAPI falls back to a raw byte export when a schema misreads.
		if typeName, _ := export["$type"].(string); strings.Contains(typeName, "RawExport") {
			read.clean = false
		}
		if class, err := r.exportClass(export); err == nil {
			read.classes = append(read.classes, class)
		}
		content := make(map[string]any, len(exportContentKeys))
		for _, key := range exportContentKeys {
			content[key] = stripHeaders(export[key])
		}
		read.content = append(read.content, content)
	}
	return read, nil
}

// sameValues reports whether two reads hold the same exports and values,
// whatever layout their headers numbered them in.
func (a assetRead) sameValues(b assetRead) bool {
	return reflect.DeepEqual(a.content, b.content)
}

func stripHeaders(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if key == exportHeaderKey || key == structHeaderKey {
				continue
			}
			out[key] = stripHeaders(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = stripHeaders(child)
		}
		return out
	default:
		return value
	}
}
