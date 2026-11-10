// Package patchupdate carries cooked mod assets across a game patch that moved
// their unversioned property layout. Unversioned properties are numbered by
// their position in the engine's schema, so a patch that inserts a property
// shifts every later number and makes old assets misread. The values
// themselves do not change, so renumbering each serialized header from the
// mod's original layout to the current one is enough to make the asset read
// correctly again, without touching any property value.
package patchupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Kuusouu/Cratebug/internal/usmap"
)

const (
	headerTypeName   = "UAssetAPI.Unversioned.FUnversionedHeader, UAssetAPI"
	fragmentTypeName = "UAssetAPI.Unversioned.FFragment, UAssetAPI"

	exportHeaderKey = "OriginalUnversionedHeader"
	structHeaderKey = "OriginalStructHeader"

	// Fragment fields are seven bits wide in the serialized header.
	fragmentSkipMax  = 127
	fragmentValueMax = 127
)

// ErrLayoutNotPortable means a property the asset uses has no safe place in
// the target layout: it was removed, retyped, or reordered.
var ErrLayoutNotPortable = errors.New("property layout cannot be carried to the current game")

// RemapStats counts what a remap changed.
type RemapStats struct {
	Headers   int
	Changed   int
	Unchanged int
}

type serializedProperty struct {
	index int
	zero  bool
}

type remapper struct {
	from, to *usmap.Mappings
	imports  []any
	stats    RemapStats
	cache    map[string][]int
}

// RemapAssetJSON rewrites every unversioned header in a UAssetAPI JSON
// document from the from layout to the to layout. The document must have been
// read with from, so its headers number properties in that layout.
func RemapAssetJSON(document []byte, from, to *usmap.Mappings) ([]byte, RemapStats, error) {
	root, err := decodeJSON(document)
	if err != nil {
		return nil, RemapStats{}, err
	}
	asset, ok := root.(map[string]any)
	if !ok {
		return nil, RemapStats{}, fmt.Errorf("patchupdate: asset JSON is not an object")
	}

	r := &remapper{from: from, to: to, cache: map[string][]int{}}
	r.imports, _ = asset["Imports"].([]any)
	exports, _ := asset["Exports"].([]any)
	for index, raw := range exports {
		export, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if err := r.export(export); err != nil {
			return nil, r.stats, fmt.Errorf("export %d: %w", index+1, err)
		}
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(asset); err != nil {
		return nil, r.stats, fmt.Errorf("patchupdate: encode asset JSON: %w", err)
	}
	return out.Bytes(), r.stats, nil
}

func decodeJSON(document []byte) (any, error) {
	// The worker writes its JSON files with a UTF-8 byte order mark.
	document = bytes.TrimPrefix(document, []byte("\xef\xbb\xbf"))
	decoder := json.NewDecoder(bytes.NewReader(document))
	// Numbers stay textual so 64-bit hashes and float spellings survive.
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("patchupdate: decode asset JSON: %w", err)
	}
	return root, nil
}

func (r *remapper) export(export map[string]any) error {
	if header, ok := export[exportHeaderKey].(map[string]any); ok {
		className, err := r.exportClass(export)
		if err != nil {
			return err
		}
		if err := r.header(header, className); err != nil {
			return fmt.Errorf("%s: %w", className, err)
		}
	}
	for key, value := range export {
		if key == exportHeaderKey {
			continue
		}
		if err := r.walk(value); err != nil {
			return err
		}
	}
	return nil
}

// Export class names come from the import the export's ClassIndex points to.
func (r *remapper) exportClass(export map[string]any) (string, error) {
	number, ok := export["ClassIndex"].(json.Number)
	if !ok {
		return "", fmt.Errorf("%w: export has no class index", ErrLayoutNotPortable)
	}
	index, err := number.Int64()
	if err != nil || index >= 0 || int(-index) > len(r.imports) {
		return "", fmt.Errorf("%w: export class is not an import", ErrLayoutNotPortable)
	}
	entry, _ := r.imports[-index-1].(map[string]any)
	name, _ := entry["ObjectName"].(string)
	if name == "" {
		return "", fmt.Errorf("%w: export class import has no name", ErrLayoutNotPortable)
	}
	return name, nil
}

func (r *remapper) walk(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		if header, ok := typed[structHeaderKey].(map[string]any); ok {
			structType, _ := typed["StructType"].(string)
			if structType == "" {
				return fmt.Errorf("%w: struct header without a struct type", ErrLayoutNotPortable)
			}
			if err := r.header(header, structType); err != nil {
				return fmt.Errorf("%s: %w", structType, err)
			}
		}
		for key, child := range typed {
			if key == structHeaderKey {
				continue
			}
			if err := r.walk(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := r.walk(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *remapper) header(header map[string]any, schema string) error {
	r.stats.Headers++
	properties, err := decodeHeader(header)
	if err != nil {
		return err
	}
	if len(properties) == 0 {
		r.stats.Unchanged++
		return nil
	}

	mapping, err := r.indexMapping(schema)
	if err != nil {
		return err
	}
	changed := false
	previous := -1
	for i := range properties {
		old := properties[i].index
		if old >= len(mapping) || mapping[old] < 0 {
			return fmt.Errorf("%w: property %d of %s has no counterpart", ErrLayoutNotPortable, old, schema)
		}
		next := mapping[old]
		if next <= previous {
			return fmt.Errorf("%w: properties of %s changed order", ErrLayoutNotPortable, schema)
		}
		if next != old {
			changed = true
		}
		properties[i].index = next
		previous = next
	}
	if !changed {
		r.stats.Unchanged++
		return nil
	}

	encodeHeader(header, properties)
	r.stats.Changed++
	return nil
}

// Maps each slot of schema in the from layout to its slot in the to layout,
// or -1 when the property is gone or serializes differently.
func (r *remapper) indexMapping(schema string) ([]int, error) {
	if cached, ok := r.cache[schema]; ok {
		return cached, nil
	}
	fromSlots, err := r.from.Flatten(schema)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLayoutNotPortable, err)
	}
	toSlots, err := r.to.Flatten(schema)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLayoutNotPortable, err)
	}

	type slotKey struct {
		name  string
		index int
	}
	positions := make(map[slotKey]int, len(toSlots))
	for i, slot := range toSlots {
		if slot.Name != "" {
			positions[slotKey{slot.Name, slot.ArrayIndex}] = i
		}
	}
	mapping := make([]int, len(fromSlots))
	for i, slot := range fromSlots {
		mapping[i] = -1
		if slot.Name == "" {
			continue
		}
		if next, ok := positions[slotKey{slot.Name, slot.ArrayIndex}]; ok && toSlots[next].Type.Equal(slot.Type) {
			mapping[i] = next
		}
	}
	r.cache[schema] = mapping
	return mapping, nil
}

// Reads the property indices and zero flags a JSON header describes, the way
// the engine walks fragments and the zero mask.
func decodeHeader(header map[string]any) ([]serializedProperty, error) {
	fragments, ok := header["Fragments"].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: header has no fragments", ErrLayoutNotPortable)
	}
	mask, _ := header["ZeroMask"].([]any)

	var properties []serializedProperty
	next, maskIndex := 0, 0
	for _, raw := range fragments {
		fragment, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: malformed fragment", ErrLayoutNotPortable)
		}
		skip, err := jsonInt(fragment["SkipNum"])
		if err != nil {
			return nil, err
		}
		values, err := jsonInt(fragment["ValueNum"])
		if err != nil {
			return nil, err
		}
		hasZeroes, _ := fragment["bHasAnyZeroes"].(bool)
		next += skip
		for i := 0; i < values; i++ {
			zero := false
			if hasZeroes {
				if maskIndex >= len(mask) {
					return nil, fmt.Errorf("%w: zero mask is shorter than its fragments", ErrLayoutNotPortable)
				}
				zero, _ = mask[maskIndex].(bool)
				maskIndex++
			}
			properties = append(properties, serializedProperty{index: next, zero: zero})
			next++
		}
	}
	return properties, nil
}

// Writes properties back as fragments and a zero mask, following the
// engine's header builder: a fragment breaks at every gap or after 127
// values, and the mask holds only the values of fragments that have zeroes.
func encodeHeader(header map[string]any, properties []serializedProperty) {
	type fragment struct {
		skip, first, values int
		hasZeroes           bool
	}
	var fragments []fragment
	last := -1
	for _, property := range properties {
		gap := property.index - last - 1
		if len(fragments) == 0 || gap > 0 || fragments[len(fragments)-1].values == fragmentValueMax {
			for gap > fragmentSkipMax {
				fragments = append(fragments, fragment{skip: fragmentSkipMax, first: last + 1})
				gap -= fragmentSkipMax
				last += fragmentSkipMax
			}
			fragments = append(fragments, fragment{skip: gap, first: property.index})
		}
		current := &fragments[len(fragments)-1]
		current.values++
		current.hasZeroes = current.hasZeroes || property.zero
		last = property.index
	}

	var mask []bool
	nonZero := false
	encoded := make([]any, 0, len(fragments))
	position := 0
	for i, f := range fragments {
		for j := 0; j < f.values; j++ {
			zero := properties[position+j].zero
			if !zero {
				nonZero = true
			}
			if f.hasZeroes {
				mask = append(mask, zero)
			}
		}
		position += f.values
		encoded = append(encoded, map[string]any{
			"$type":         fragmentTypeName,
			"SkipNum":       f.skip,
			"ValueNum":      f.values,
			"bIsLast":       i == len(fragments)-1,
			"FirstNum":      f.first,
			"bHasAnyZeroes": f.hasZeroes,
			"LastNum":       f.first + f.values - 1,
		})
	}

	header["$type"] = headerTypeName
	header["Fragments"] = encoded
	header["ZeroMask"] = padZeroMask(mask)
	header["bHasNonZeroValues"] = nonZero
}

// The serialized mask is one byte up to 8 bits, two up to 16, then whole
// 32-bit words. The reader exposes that full width, so the JSON form does too.
func padZeroMask(mask []bool) []any {
	if len(mask) == 0 {
		return []any{}
	}
	width := 8
	switch {
	case len(mask) <= 8:
	case len(mask) <= 16:
		width = 16
	default:
		width = (len(mask) + 31) / 32 * 32
	}
	padded := make([]any, width)
	for i := range padded {
		padded[i] = i < len(mask) && mask[i]
	}
	return padded
}

func jsonInt(value any) (int, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%w: expected a number, got %T", ErrLayoutNotPortable, value)
	}
	parsed, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrLayoutNotPortable, err)
	}
	return int(parsed), nil
}
