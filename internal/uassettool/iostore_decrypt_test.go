package uassettool

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestDecryptIoStorePreservesCompressedBytesAndMetadata(t *testing.T) {
	for _, encryptedDirectory := range []bool{false, true} {
		t.Run(map[bool]string{false: "clear directory", true: "encrypted directory"}[encryptedDirectory], func(t *testing.T) {
			// Arrange
			path, plainTOC, plainUCAS, compressedData := encryptedBlockFixture(t, encryptedDirectory)

			// Act
			err := DecryptIoStoreDirect(path)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			for file, want := range map[string][]byte{path: plainTOC, fixtureUCASPath(path): plainUCAS} {
				got, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s changed beyond decryption", filepath.Base(file))
				}
			}
			got, _ := os.ReadFile(fixtureUCASPath(path))
			if !bytes.Equal(got[64:64+len(compressedData)], compressedData) {
				t.Fatal("the compressed payload changed")
			}
		})
	}
}

func TestDecryptIoStoreRejectsUnsafeLayoutsBeforeWrites(t *testing.T) {
	cases := []struct {
		name   string
		change func([]byte)
	}{
		{"bad magic", func(toc []byte) { toc[0] = 0 }},
		{"unknown version", func(toc []byte) { toc[16] = 6 }},
		{"signed", func(toc []byte) { toc[80] |= 4 }},
		{"multiple partitions", func(toc []byte) { binary.LittleEndian.PutUint32(toc[52:56], 2) }},
		{"invalid block table", func(toc []byte) { binary.LittleEndian.PutUint32(toc[28:32], ^uint32(0)) }},
		{"truncated directory", func(toc []byte) { binary.LittleEndian.PutUint32(toc[48:52], ^uint32(0)) }},
		{"overlapping blocks", func(toc []byte) { toc[blockFixtureTableOffset+12] = 0 }},
		{"invalid method", func(toc []byte) { toc[blockFixtureTableOffset+11] = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			path, _, _, _ := encryptedBlockFixture(t, false)
			toc, _ := os.ReadFile(path)
			tc.change(toc)
			if err := os.WriteFile(path, toc, 0o600); err != nil {
				t.Fatal(err)
			}
			ucas, _ := os.ReadFile(fixtureUCASPath(path))

			// Act
			err := DecryptIoStoreDirect(path)

			// Assert
			if err == nil {
				t.Fatal("unsafe layout was accepted")
			}
			for file, want := range map[string][]byte{path: toc, fixtureUCASPath(path): ucas} {
				got, _ := os.ReadFile(file)
				if !bytes.Equal(got, want) {
					t.Fatal("invalid input changed before rejection")
				}
			}
		})
	}
}

func TestDecryptIoStoreRejectsTruncatedUCAS(t *testing.T) {
	// Arrange
	path, _, _, _ := encryptedBlockFixture(t, false)
	toc, _ := os.ReadFile(path)
	if err := os.Truncate(fixtureUCASPath(path), 1); err != nil {
		t.Fatal(err)
	}

	// Act
	err := DecryptIoStoreDirect(path)

	// Assert
	if err == nil {
		t.Fatal("truncated UCAS was accepted")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, toc) {
		t.Fatal("failed decrypt cleared the encrypted flag")
	}
}

func TestDecryptIoStoreWithPinnedWorker(t *testing.T) {
	// Arrange
	executable := pinnedWorkerExecutablePath(t)
	if _, err := os.Stat(executable); err != nil {
		t.Skip("fetch the pinned worker to run this archive test")
	}
	worker, err := NewWorker(WorkerConfig{ExecutablePath: executable, ExpectedSourceRevision: PinnedSourceRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	path := buildIoStoreFixture(t, worker, t.TempDir())
	plainUCAS, err := os.ReadFile(fixtureUCASPath(path))
	if err != nil {
		t.Fatal(err)
	}
	plainTOC, _ := os.ReadFile(path)
	pak := path[:len(path)-len(".utoc")] + ".pak"
	if _, err := EncryptIoStoreDirect(worker, pak, MarvelRivalsAESKey); err != nil {
		t.Fatal(err)
	}

	// Act
	err = DecryptIoStoreDirect(path)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := IsIoStoreEncrypted(worker, path)
	if err != nil || encrypted {
		t.Fatalf("decrypted state = %v, %v", encrypted, err)
	}
	gotTOC, _ := os.ReadFile(path)
	if !bytes.Equal(gotTOC[56:64], plainTOC[56:64]) {
		t.Fatal("container identity changed")
	}
	gotUCAS, _ := os.ReadFile(fixtureUCASPath(path))
	if len(gotUCAS) < len(plainUCAS) || !bytes.Equal(gotUCAS[:len(plainUCAS)], plainUCAS) || len(bytes.Trim(gotUCAS[len(plainUCAS):], "\x00")) != 0 {
		t.Fatal("the worker's original container header payload changed")
	}
}

// Two chunk entries, one perfect-hash seed, and one overflow index precede the blocks.
const blockFixtureTableOffset = 144 + 2*(12+10) + 4 + 4

func encryptedBlockFixture(t *testing.T, encryptedDirectory bool) (string, []byte, []byte, []byte) {
	t.Helper()
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("cooked texture bytes"), 10)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	directory := blockFixtureDirectory(t)
	if encryptedDirectory {
		directory = append(directory, make([]byte, (-len(directory))&15)...)
	}
	const blockTableSize = 2 * 12
	const methodNameSize = 32
	directoryOffset := blockFixtureTableOffset + blockTableSize + methodNameSize
	toc := make([]byte, directoryOffset+len(directory)+64)
	copy(toc, "-==--==--==--==-")
	toc[16] = 5
	for offset, value := range map[int]uint32{20: 144, 24: 2, 28: 2, 32: 12, 36: 1, 40: 32, 44: 256, 48: uint32(len(directory)), 52: 1, 84: 1, 96: 1} {
		binary.LittleEndian.PutUint32(toc[offset:offset+4], value)
	}
	binary.LittleEndian.PutUint64(toc[56:64], 987654321)
	copy(toc[144:168], bytes.Repeat([]byte{0xAB}, 24))
	toc[80] = 9
	entry := toc[blockFixtureTableOffset:]
	entry[5], entry[8] = 19, 19
	entry[12], entry[17], entry[20], entry[23] = 64, byte(compressed.Len()), 200, 1
	copy(toc[blockFixtureTableOffset+blockTableSize:], "Zlib")
	copy(toc[directoryOffset:], directory)
	plainTOC := bytes.Clone(toc)
	plainUCAS := make([]byte, 64+((compressed.Len()+15)&^15))
	copy(plainUCAS, "uncompressed cooked")
	copy(plainUCAS[64:], compressed.Bytes())
	encryptedUCAS := bytes.Clone(plainUCAS)
	key, err := hex.DecodeString(MarvelRivalsAESKey)
	if err != nil {
		t.Fatal(err)
	}
	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{encryptedUCAS[:32], encryptedUCAS[64:]} {
		for i := 0; i < len(data); i += aes.BlockSize {
			blockCipher.Encrypt(data[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
		}
	}
	if encryptedDirectory {
		data := toc[directoryOffset : directoryOffset+len(directory)]
		for i := 0; i < len(data); i += aes.BlockSize {
			blockCipher.Encrypt(data[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
		}
	}
	toc[80] |= 2
	path := filepath.Join(t.TempDir(), "test.utoc")
	for file, data := range map[string][]byte{path: toc, fixtureUCASPath(path): encryptedUCAS} {
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path, plainTOC, plainUCAS, compressed.Bytes()
}

func blockFixtureDirectory(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	values := []any{
		uint32(10), []byte("../../../\x00"),
		uint32(1), []uint32{^uint32(0), ^uint32(0), ^uint32(0), 0},
		uint32(1), []uint32{0, ^uint32(0), 0},
		uint32(1), uint32(13), []byte("asset.uasset\x00"),
	}
	for _, value := range values {
		if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	return data.Bytes()
}

func fixtureUCASPath(path string) string {
	return path[:len(path)-len(filepath.Ext(path))] + ".ucas"
}
