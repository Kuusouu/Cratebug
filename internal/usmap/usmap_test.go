package usmap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/klauspost/compress/zstd"
)

const (
	testKindBool  = 1
	testKindInt   = 2
	testKindFloat = 3
)

type testProperty struct {
	name      string
	schemaIdx uint16
	arraySize uint8
	// Raw type bytes after the kind, with names given as indexes.
	kind  uint8
	extra []uint32
}

type testStruct struct {
	name, super string
	count       uint16
	properties  []testProperty
}

// Builds a version 4 mappings file in the format Parse reads.
func buildMappings(t *testing.T, structs []testStruct, compression uint8) []byte {
	t.Helper()
	var names []string
	index := map[string]uint32{}
	name := func(value string) uint32 {
		if i, ok := index[value]; ok {
			return i
		}
		index[value] = uint32(len(names))
		names = append(names, value)
		return index[value]
	}
	for _, s := range structs {
		name(s.name)
		if s.super != "" {
			name(s.super)
		}
		for _, p := range s.properties {
			name(p.name)
		}
	}

	var payload bytes.Buffer
	write := func(value any) {
		if err := binary.Write(&payload, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	write(uint32(len(names)))
	for _, n := range names {
		write(uint16(len(n)))
		payload.WriteString(n)
	}
	write(uint32(0))
	write(uint32(len(structs)))
	for _, s := range structs {
		write(name(s.name))
		if s.super == "" {
			write(uint32(noSuper))
		} else {
			write(name(s.super))
		}
		write(s.count)
		write(uint16(len(s.properties)))
		for _, p := range s.properties {
			write(p.schemaIdx)
			write(p.arraySize)
			write(name(p.name))
			write(p.kind)
			for _, value := range p.extra {
				write(value)
			}
		}
	}

	body := payload.Bytes()
	if compression == compressionZstd {
		encoder, err := zstd.NewWriter(nil)
		if err != nil {
			t.Fatal(err)
		}
		body = encoder.EncodeAll(payload.Bytes(), nil)
		if err := encoder.Close(); err != nil {
			t.Fatal(err)
		}
	}

	var file bytes.Buffer
	for _, value := range []any{uint16(fileMagic), uint8(latestVersion), int32(0), compression, uint32(len(body)), uint32(payload.Len())} {
		if err := binary.Write(&file, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	file.Write(body)
	return file.Bytes()
}

func baseAndChild() []testStruct {
	return []testStruct{
		{name: "Base", count: 2, properties: []testProperty{
			{name: "Flag", schemaIdx: 0, arraySize: 1, kind: testKindBool},
			{name: "Count", schemaIdx: 1, arraySize: 1, kind: testKindInt},
		}},
		{name: "Child", super: "Base", count: 4, properties: []testProperty{
			{name: "Pair", schemaIdx: 0, arraySize: 2, kind: testKindFloat},
			{name: "Nested", schemaIdx: 3, arraySize: 1, kind: kindStruct, extra: []uint32{0}},
		}},
	}
}

func TestParseReadsStructsFromCompressedMappings(t *testing.T) {
	// Arrange
	data := buildMappings(t, baseAndChild(), compressionZstd)

	// Act
	mappings, err := Parse(data)

	// Assert
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	child := mappings.Structs["Child"]
	if child == nil || child.Super != "Base" || child.PropCount != 4 || len(child.Properties) != 2 {
		t.Fatalf("Child = %+v", child)
	}
	if nested := child.Properties[1]; nested.Type.Kind != kindStruct || nested.Type.StructName != "Base" {
		t.Errorf("Nested type = %+v, want a Base struct", nested.Type)
	}
}

func TestFlattenNumbersSuperPropertiesFirstAndExpandsStaticArrays(t *testing.T) {
	// Arrange
	mappings, err := Parse(buildMappings(t, baseAndChild(), compressionNone))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	slots, err := mappings.Flatten("Child")

	// Assert
	if err != nil {
		t.Fatalf("Flatten() error = %v", err)
	}
	want := []struct {
		name  string
		index int
	}{{"Flag", 0}, {"Count", 0}, {"Pair", 0}, {"Pair", 1}, {"", 0}, {"Nested", 0}}
	if len(slots) != len(want) {
		t.Fatalf("Flatten() = %d slots, want %d", len(slots), len(want))
	}
	for i, slot := range slots {
		if slot.Name != want[i].name || slot.ArrayIndex != want[i].index {
			t.Errorf("slot %d = %s[%d], want %s[%d]", i, slot.Name, slot.ArrayIndex, want[i].name, want[i].index)
		}
	}
}

func TestFingerprintChangesOnlyWhenADependedOnStructChanges(t *testing.T) {
	// Arrange
	before, err := Parse(buildMappings(t, baseAndChild(), compressionNone))
	if err != nil {
		t.Fatal(err)
	}
	changed := baseAndChild()
	changed[0].count = 3
	changed[0].properties = append(changed[0].properties, testProperty{name: "Added", schemaIdx: 2, arraySize: 1, kind: testKindBool})
	changed = append(changed, testStruct{name: "Unrelated", count: 0})
	after, err := Parse(buildMappings(t, changed, compressionNone))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	closure := before.Closure("Child")
	unrelated := map[string]bool{"Unrelated": true}

	// Assert
	if !closure["Base"] || !closure["Child"] {
		t.Fatalf("Closure(Child) = %v, want Base and Child", closure)
	}
	if before.Fingerprint(closure) == after.Fingerprint(closure) {
		t.Error("fingerprint did not change when Base gained a property")
	}
	if before.Fingerprint(map[string]bool{"Child": true}) != after.Fingerprint(map[string]bool{"Child": true}) {
		t.Error("fingerprint of Child alone changed although Child did not")
	}
	if before.Fingerprint(unrelated) == after.Fingerprint(unrelated) {
		t.Error("a struct missing from one file fingerprinted the same as one present")
	}
}

func TestParseRejectsFilesThatAreNotMappings(t *testing.T) {
	// Arrange
	data := buildMappings(t, baseAndChild(), compressionNone)
	truncated := data[:len(data)-3]

	// Act
	_, badMagic := Parse([]byte("not a mappings file"))
	_, short := Parse(truncated)

	// Assert
	if !errors.Is(badMagic, ErrNotMappings) {
		t.Errorf("Parse(garbage) error = %v, want ErrNotMappings", badMagic)
	}
	if !errors.Is(short, ErrNotMappings) {
		t.Errorf("Parse(truncated) error = %v, want ErrNotMappings", short)
	}
}
