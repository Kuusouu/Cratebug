package patchupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/usmap"
)

func boolProperty(name string, index uint16) usmap.Property {
	return usmap.Property{Name: name, SchemaIdx: index, ArraySize: 1, Type: usmap.Type{Kind: 1}}
}

func mappingsOf(structs ...*usmap.Struct) *usmap.Mappings {
	m := &usmap.Mappings{Structs: map[string]*usmap.Struct{}}
	for _, s := range structs {
		m.Structs[s.Name] = s
	}
	return m
}

// Overrides gains two properties in the middle, as the 2026-10-29 patch did to
// MaterialInstanceBasePropertyOverrides. Material holds one Overrides struct.
func shiftedLayouts() (*usmap.Mappings, *usmap.Mappings) {
	overridesProperty := usmap.Property{Name: "Overrides", SchemaIdx: 1, ArraySize: 1, Type: usmap.Type{Kind: 9, StructName: "Overrides"}}
	before := mappingsOf(
		&usmap.Struct{Name: "Overrides", PropCount: 3, Properties: []usmap.Property{
			boolProperty("First", 0), boolProperty("Moved", 1), boolProperty("Last", 2),
		}},
		&usmap.Struct{Name: "Material", PropCount: 2, Properties: []usmap.Property{boolProperty("Flag", 0), overridesProperty}},
	)
	after := mappingsOf(
		&usmap.Struct{Name: "Overrides", PropCount: 5, Properties: []usmap.Property{
			boolProperty("First", 0), boolProperty("NewA", 1), boolProperty("NewB", 2), boolProperty("Moved", 3), boolProperty("Last", 4),
		}},
		&usmap.Struct{Name: "Material", PropCount: 2, Properties: []usmap.Property{boolProperty("Flag", 0), overridesProperty}},
	)
	return before, after
}

// A header in UAssetAPI's JSON form for properties at indices, zero where flagged.
func headerJSON(properties ...serializedProperty) map[string]any {
	header := map[string]any{}
	encodeHeader(header, properties)
	return header
}

func assetJSON(t *testing.T, exportHeader, structHeader map[string]any) []byte {
	t.Helper()
	asset := map[string]any{
		"HasUnversionedProperties": true,
		"Imports":                  []any{map[string]any{"ObjectName": "Material"}},
		"Exports": []any{map[string]any{
			"$type":         "UAssetAPI.ExportTypes.NormalExport, UAssetAPI",
			"ClassIndex":    -1,
			exportHeaderKey: exportHeader,
			"Data": []any{map[string]any{
				"Name":          "Overrides",
				"StructType":    "Overrides",
				structHeaderKey: structHeader,
				"Value":         []any{},
			}},
		}},
	}
	data, err := json.Marshal(asset)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func structHeaderOf(t *testing.T, document []byte) []serializedProperty {
	t.Helper()
	root, err := decodeJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	export := root.(map[string]any)["Exports"].([]any)[0].(map[string]any)
	data := export["Data"].([]any)[0].(map[string]any)
	properties, err := decodeHeader(data[structHeaderKey].(map[string]any))
	if err != nil {
		t.Fatal(err)
	}
	return properties
}

func TestRemapAssetJSONRenumbersPropertiesAfterAnInsertion(t *testing.T) {
	// Arrange
	before, after := shiftedLayouts()
	document := assetJSON(t,
		headerJSON(serializedProperty{index: 1}),
		headerJSON(serializedProperty{index: 0}, serializedProperty{index: 2, zero: true}),
	)

	// Act
	remapped, stats, err := RemapAssetJSON(document, before, after)

	// Assert
	if err != nil {
		t.Fatalf("RemapAssetJSON() error = %v", err)
	}
	if stats.Headers != 2 || stats.Changed != 1 {
		t.Errorf("stats = %+v, want 2 headers with 1 changed", stats)
	}
	want := []serializedProperty{{index: 0}, {index: 4, zero: true}}
	if got := structHeaderOf(t, remapped); !reflect.DeepEqual(got, want) {
		t.Errorf("struct header = %+v, want %+v", got, want)
	}
}

func TestRemapAssetJSONLeavesUnmovedHeadersByteForByte(t *testing.T) {
	// Arrange
	before, after := shiftedLayouts()
	original := headerJSON(serializedProperty{index: 0})
	original["Fragments"].([]any)[0].(map[string]any)["Marker"] = "kept"
	document := assetJSON(t, headerJSON(serializedProperty{index: 1}), original)

	// Act
	remapped, stats, err := RemapAssetJSON(document, before, after)

	// Assert
	if err != nil {
		t.Fatalf("RemapAssetJSON() error = %v", err)
	}
	if stats.Changed != 0 {
		t.Errorf("stats = %+v, want nothing changed", stats)
	}
	if !strings.Contains(string(remapped), `"Marker":"kept"`) {
		t.Error("an unmoved header was rewritten")
	}
}

func TestRemapAssetJSONRefusesRemovedAndReorderedProperties(t *testing.T) {
	// Arrange
	before, after := shiftedLayouts()
	removed := mappingsOf(after.Structs["Material"], &usmap.Struct{Name: "Overrides", PropCount: 2, Properties: []usmap.Property{
		boolProperty("First", 0), boolProperty("Last", 1),
	}})
	reordered := mappingsOf(after.Structs["Material"], &usmap.Struct{Name: "Overrides", PropCount: 3, Properties: []usmap.Property{
		boolProperty("Last", 0), boolProperty("First", 1), boolProperty("Moved", 2),
	}})
	document := assetJSON(t,
		headerJSON(serializedProperty{index: 1}),
		headerJSON(serializedProperty{index: 0}, serializedProperty{index: 1}, serializedProperty{index: 2}),
	)

	for name, target := range map[string]*usmap.Mappings{"removed": removed, "reordered": reordered} {
		t.Run(name, func(t *testing.T) {
			// Act
			_, _, err := RemapAssetJSON(document, before, target)

			// Assert
			if !errors.Is(err, ErrLayoutNotPortable) {
				t.Errorf("RemapAssetJSON() error = %v, want ErrLayoutNotPortable", err)
			}
		})
	}
}

func TestEncodeHeaderFollowsTheEngineFragmentRules(t *testing.T) {
	// Arrange
	var properties []serializedProperty
	for i := range 130 {
		properties = append(properties, serializedProperty{index: i, zero: i == 128})
	}
	properties = append(properties, serializedProperty{index: 400})

	// Act
	header := roundTrip(t, headerJSON(properties...))
	decoded, err := decodeHeader(header)

	// Assert
	if err != nil {
		t.Fatalf("decodeHeader() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, properties) {
		t.Fatal("decodeHeader(encodeHeader(p)) did not return p")
	}
	var shape []string
	for _, raw := range header["Fragments"].([]any) {
		f := raw.(map[string]any)
		shape = append(shape, fmt.Sprintf("%v+%v/%v", f["SkipNum"], f["ValueNum"], f["bHasAnyZeroes"]))
	}
	// 127 values fill one fragment. The 270 skipped indices between 129 and
	// 400 exceed the 127 one fragment holds, so two skip-only fragments come first.
	want := []string{"0+127/false", "0+3/true", "127+0/false", "127+0/false", "16+1/false"}
	if strings.Join(shape, " ") != strings.Join(want, " ") {
		t.Errorf("fragments = %v, want %v", shape, want)
	}
	if mask := header["ZeroMask"].([]any); len(mask) != 8 {
		t.Errorf("zero mask width = %d, want 8 (one byte for three masked values)", len(mask))
	}
}

func TestParseAssetReadSpotsRawExportsAndComparesValuesWithoutHeaders(t *testing.T) {
	// Arrange
	before, after := shiftedLayouts()
	document := assetJSON(t, headerJSON(serializedProperty{index: 1}), headerJSON(serializedProperty{index: 2}))
	remapped, _, err := RemapAssetJSON(document, before, after)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Replace(string(document), "NormalExport", "RawExport", 1)

	// Act
	original, err := parseAssetRead(document)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := parseAssetRead(remapped)
	if err != nil {
		t.Fatal(err)
	}
	misread, err := parseAssetRead([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if !original.clean || misread.clean {
		t.Errorf("clean = %v and %v, want true and false", original.clean, misread.clean)
	}
	if !original.unversioned || !reflect.DeepEqual(original.classes, []string{"Material"}) {
		t.Errorf("read = unversioned %v classes %v", original.unversioned, original.classes)
	}
	if !original.sameValues(moved) {
		t.Error("renumbered headers changed the compared values")
	}
	if original.sameValues(misread) {
		t.Error("a raw export compared equal to a parsed one")
	}
}

// Passes a value through JSON so numbers arrive as they do from the worker.
func roundTrip(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return decoded.(map[string]any)
}
