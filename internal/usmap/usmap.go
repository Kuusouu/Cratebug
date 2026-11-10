// Package usmap reads Unreal Engine mapping files (.usmap), which describe the
// property layout that unversioned cooked assets are serialized against.
package usmap

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	fileMagic = 0x30C4

	versionPackageVersioning = 1
	versionLongFName         = 2
	versionLargeEnums        = 3
	versionExplicitEnumValue = 4
	latestVersion            = versionExplicitEnumValue

	compressionNone = 0
	compressionZstd = 3

	noSuper = 0xFFFFFFFF

	// Bounds the decompressed payload so a corrupt size field cannot request
	// an unbounded allocation. Current Marvel Rivals mappings are ~4.4 MB.
	maxPayloadBytes = 64 << 20
)

// Property kinds as numbered in the mapping format. Only kinds that carry
// extra type data need names here.
const (
	kindArray     = 8
	kindStruct    = 9
	kindMap       = 24
	kindSet       = 25
	kindEnum      = 26
	kindOptional  = 28
	maxKnownKinds = 31
)

// ErrNotMappings is reported for a file that is not a supported .usmap.
var ErrNotMappings = errors.New("usmap: not a supported mappings file")

// Type describes a property's type, including container element types.
type Type struct {
	Kind       uint8
	StructName string
	EnumName   string
	Inner      *Type
	Value      *Type
}

// Property is one serializable property of a struct or class.
type Property struct {
	Name      string
	SchemaIdx uint16
	ArraySize uint8
	Type      Type
}

// Struct is the schema of one struct or class. PropCount counts static array
// elements individually, which is how unversioned headers number properties.
type Struct struct {
	Name       string
	Super      string
	PropCount  uint16
	Properties []Property
}

// Mappings holds every struct schema in one mappings file, keyed by name.
type Mappings struct {
	Structs map[string]*Struct
	// FileVersionUE4 and FileVersionUE5 are the package versions the mappings
	// file records. They are zero when the file has no package versioning, which
	// is the case for the community Marvel Rivals files.
	FileVersionUE4 int32
	FileVersionUE5 int32
}

// Slot is one unversioned property index of a flattened struct schema. A
// static array property occupies one slot per element.
type Slot struct {
	Name       string
	ArrayIndex int
	Type       Type
}

// ErrUnknownStruct is reported when a schema names a struct the file lacks.
var ErrUnknownStruct = errors.New("usmap: unknown struct")

// Flatten lists name's unversioned property slots in serialization order:
// the root super's properties first, as unversioned headers number them.
func (m *Mappings) Flatten(name string) ([]Slot, error) {
	var chain []*Struct
	for current, depth := name, 0; current != ""; depth++ {
		s, ok := m.Structs[current]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownStruct, current)
		}
		const maxSuperDepth = 64
		if depth > maxSuperDepth {
			return nil, fmt.Errorf("usmap: super chain of %s does not end", name)
		}
		chain = append(chain, s)
		current = s.Super
	}

	var slots []Slot
	for i := len(chain) - 1; i >= 0; i-- {
		s := chain[i]
		own := make([]Slot, s.PropCount)
		for _, prop := range s.Properties {
			for element := 0; element < int(prop.ArraySize); element++ {
				index := int(prop.SchemaIdx) + element
				if index >= len(own) {
					return nil, fmt.Errorf("usmap: %s.%s index %d exceeds count %d", s.Name, prop.Name, index, s.PropCount)
				}
				own[index] = Slot{Name: prop.Name, ArrayIndex: element, Type: prop.Type}
			}
		}
		slots = append(slots, own...)
	}
	return slots, nil
}

// Closure names every struct name's layout depends on: name itself, its
// supers, and every struct its properties hold, directly or in containers.
func (m *Mappings) Closure(name string) map[string]bool {
	seen := map[string]bool{}
	pending := []string{name}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == "" || seen[current] {
			continue
		}
		seen[current] = true
		s, ok := m.Structs[current]
		if !ok {
			continue
		}
		pending = append(pending, s.Super)
		for _, prop := range s.Properties {
			pending = prop.Type.structNames(pending)
		}
	}
	return seen
}

func (t Type) structNames(into []string) []string {
	if t.StructName != "" {
		into = append(into, t.StructName)
	}
	if t.Inner != nil {
		into = t.Inner.structNames(into)
	}
	if t.Value != nil {
		into = t.Value.structNames(into)
	}
	return into
}

// Fingerprint identifies the serialized layout of every struct in names.
// Two mappings with equal fingerprints read those structs identically.
func (m *Mappings) Fingerprint(names map[string]bool) string {
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	var b strings.Builder
	for _, name := range sorted {
		s, ok := m.Structs[name]
		if !ok {
			fmt.Fprintf(&b, "%s:missing;", name)
			continue
		}
		fmt.Fprintf(&b, "%s:%s:%d", s.Name, s.Super, s.PropCount)
		for _, prop := range s.Properties {
			fmt.Fprintf(&b, ",%d/%d/%s/", prop.SchemaIdx, prop.ArraySize, prop.Name)
			prop.Type.describe(&b)
		}
		b.WriteByte(';')
	}
	return b.String()
}

func (t Type) describe(b *strings.Builder) {
	fmt.Fprintf(b, "%d", t.Kind)
	if t.StructName != "" {
		b.WriteString("<" + t.StructName + ">")
	}
	if t.EnumName != "" {
		b.WriteString("{" + t.EnumName + "}")
	}
	if t.Inner != nil {
		b.WriteByte('(')
		t.Inner.describe(b)
		if t.Value != nil {
			b.WriteByte(',')
			t.Value.describe(b)
		}
		b.WriteByte(')')
	}
}

// Equal reports whether two types serialize identically.
func (t Type) Equal(other Type) bool {
	if t.Kind != other.Kind || t.StructName != other.StructName || t.EnumName != other.EnumName {
		return false
	}
	if (t.Inner == nil) != (other.Inner == nil) || (t.Value == nil) != (other.Value == nil) {
		return false
	}
	if t.Inner != nil && !t.Inner.Equal(*other.Inner) {
		return false
	}
	return t.Value == nil || t.Value.Equal(*other.Value)
}

// Load reads and parses the mappings file at path.
func Load(path string) (*Mappings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("usmap: read %s: %w", path, err)
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("usmap: %s: %w", path, err)
	}
	return m, nil
}

// Parse decodes a mappings file held in memory.
func Parse(data []byte) (*Mappings, error) {
	r := bytes.NewReader(data)
	var magic uint16
	var version uint8
	if err := binary.Read(r, binary.LittleEndian, &magic); err != nil || magic != fileMagic {
		return nil, ErrNotMappings
	}
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil || version > latestVersion {
		return nil, fmt.Errorf("%w: version %d", ErrNotMappings, version)
	}
	var fileUE4, fileUE5 int32
	if version >= versionPackageVersioning {
		var hasVersioning int32
		if err := binary.Read(r, binary.LittleEndian, &hasVersioning); err != nil {
			return nil, err
		}
		if hasVersioning != 0 {
			var err error
			if fileUE4, fileUE5, err = packageVersioning(r); err != nil {
				return nil, err
			}
		}
	}

	var header struct {
		Compression      uint8
		CompressedSize   uint32
		DecompressedSize uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	if header.DecompressedSize > maxPayloadBytes || int64(header.CompressedSize) > int64(r.Len()) {
		return nil, fmt.Errorf("%w: payload size out of range", ErrNotMappings)
	}
	compressed := make([]byte, header.CompressedSize)
	if _, err := io.ReadFull(r, compressed); err != nil {
		return nil, err
	}

	payload, err := decompress(header.Compression, compressed, header.DecompressedSize)
	if err != nil {
		return nil, err
	}
	p := &parser{r: bufio.NewReader(bytes.NewReader(payload)), version: version}
	mappings, err := p.mappings()
	if err != nil {
		return nil, err
	}
	mappings.FileVersionUE4 = fileUE4
	mappings.FileVersionUE5 = fileUE5
	return mappings, nil
}

func packageVersioning(r *bytes.Reader) (int32, int32, error) {
	// FPackageFileVersion (UE4, UE5), then custom versions, then the net CL.
	var fileVersion [2]int32
	if err := binary.Read(r, binary.LittleEndian, &fileVersion); err != nil {
		return 0, 0, err
	}
	var customCount int32
	if err := binary.Read(r, binary.LittleEndian, &customCount); err != nil {
		return 0, 0, err
	}
	const customVersionBytes = 16 + 4
	if customCount < 0 || int64(customCount)*customVersionBytes > int64(r.Len()) {
		return 0, 0, fmt.Errorf("%w: custom version count out of range", ErrNotMappings)
	}
	if _, err := r.Seek(int64(customCount)*customVersionBytes+4, io.SeekCurrent); err != nil {
		return 0, 0, err
	}
	return fileVersion[0], fileVersion[1], nil
}

func decompress(method uint8, data []byte, size uint32) ([]byte, error) {
	switch method {
	case compressionNone:
		return data, nil
	case compressionZstd:
		decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxPayloadBytes))
		if err != nil {
			return nil, err
		}
		defer decoder.Close()
		out, err := decoder.DecodeAll(data, make([]byte, 0, size))
		if err != nil {
			return nil, fmt.Errorf("usmap: zstd: %w", err)
		}
		if len(out) != int(size) {
			return nil, fmt.Errorf("%w: decompressed %d bytes, header says %d", ErrNotMappings, len(out), size)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: unsupported compression %d", ErrNotMappings, method)
	}
}

type parser struct {
	r       *bufio.Reader
	version uint8
	names   []string
	err     error
}

func (p *parser) mappings() (*Mappings, error) {
	nameCount := p.u32()
	for i := uint32(0); i < nameCount && p.err == nil; i++ {
		var length int
		if p.version >= versionLongFName {
			length = int(p.u16())
		} else {
			length = int(p.u8())
		}
		buf := make([]byte, length)
		p.read(buf)
		p.names = append(p.names, string(buf))
	}

	// Enum values are not needed to remap property indices; skip them.
	enumCount := p.u32()
	for i := uint32(0); i < enumCount && p.err == nil; i++ {
		p.u32()
		var count int
		if p.version >= versionLargeEnums {
			count = int(p.u16())
		} else {
			count = int(p.u8())
		}
		for j := 0; j < count && p.err == nil; j++ {
			if p.version >= versionExplicitEnumValue {
				p.u64()
			}
			p.u32()
		}
	}

	structCount := p.u32()
	m := &Mappings{Structs: make(map[string]*Struct, structCount)}
	for i := uint32(0); i < structCount && p.err == nil; i++ {
		s := &Struct{Name: p.name()}
		if super := p.u32(); super != noSuper {
			s.Super = p.nameAt(super)
		}
		s.PropCount = p.u16()
		serializable := p.u16()
		for j := uint16(0); j < serializable && p.err == nil; j++ {
			prop := Property{SchemaIdx: p.u16(), ArraySize: p.u8(), Name: p.name()}
			prop.Type = p.propertyType()
			s.Properties = append(s.Properties, prop)
		}
		m.Structs[s.Name] = s
	}
	if p.err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotMappings, p.err)
	}
	return m, nil
}

func (p *parser) propertyType() Type {
	t := Type{Kind: p.u8()}
	if t.Kind >= maxKnownKinds && p.err == nil {
		p.err = fmt.Errorf("unknown property kind %d", t.Kind)
		return t
	}
	switch t.Kind {
	case kindEnum:
		inner := p.propertyType()
		t.Inner = &inner
		t.EnumName = p.name()
	case kindStruct:
		t.StructName = p.name()
	case kindArray, kindSet, kindOptional:
		inner := p.propertyType()
		t.Inner = &inner
	case kindMap:
		key := p.propertyType()
		value := p.propertyType()
		t.Inner, t.Value = &key, &value
	}
	return t
}

func (p *parser) name() string { return p.nameAt(p.u32()) }

func (p *parser) nameAt(index uint32) string {
	if p.err != nil {
		return ""
	}
	if int(index) >= len(p.names) {
		p.err = fmt.Errorf("name index %d out of range", index)
		return ""
	}
	return p.names[index]
}

func (p *parser) read(buf []byte) {
	if p.err == nil {
		_, p.err = io.ReadFull(p.r, buf)
	}
}

func (p *parser) u8() uint8 {
	var b [1]byte
	p.read(b[:])
	return b[0]
}

func (p *parser) u16() uint16 {
	var b [2]byte
	p.read(b[:])
	return binary.LittleEndian.Uint16(b[:])
}

func (p *parser) u32() uint32 {
	var b [4]byte
	p.read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}

func (p *parser) u64() uint64 {
	var b [8]byte
	p.read(b[:])
	return binary.LittleEndian.Uint64(b[:])
}
