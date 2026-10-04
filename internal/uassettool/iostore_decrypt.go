package uassettool

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	ioStoreHeaderSize           = 144
	ioStoreFlagsOffset          = 80
	ioStoreEncrypted            = 2
	ioStoreSigned               = 4
	ioStoreCompressionEntrySize = 12
	ioStorePerfectHashVersion   = 4
	ioStoreOverflowVersion      = 5
)

// Decrypts a private staged copy without changing chunk IDs, compression, or cooked asset bytes.
// Signed containers and multiple UCAS partitions fail before any block changes.
func DecryptIoStoreDirect(utocPath string) error {
	toc, err := os.ReadFile(utocPath)
	if err != nil {
		return fmt.Errorf("read encrypted TOC: %w", err)
	}
	ucasPath := strings.TrimSuffix(utocPath, filepath.Ext(utocPath)) + ".ucas"
	ucas, err := os.OpenFile(ucasPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open encrypted UCAS: %w", err)
	}
	defer ucas.Close()
	info, err := ucas.Stat()
	if err != nil {
		return err
	}
	blocks, directory, err := encryptedIoStoreLayout(toc, info.Size())
	if err != nil {
		return err
	}
	key, err := hex.DecodeString(MarvelRivalsAESKey)
	if err != nil {
		return err
	}
	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// The pinned writer leaves its directory index clear even when it encrypts the chunk blocks.
	if len(directory) != 0 && !validIoStoreDirectory(directory) {
		if len(directory)%aes.BlockSize != 0 {
			return fmt.Errorf("encrypted directory index is not AES-aligned")
		}
		decryptIoStoreBytes(blockCipher, directory)
		if !validIoStoreDirectory(directory) {
			return fmt.Errorf("directory index cannot be decrypted with the game key")
		}
	}

	for offset := 0; offset < len(blocks); offset += ioStoreCompressionEntrySize {
		entry := blocks[offset : offset+ioStoreCompressionEntrySize]
		position := int64(littleEndianBytes(entry[:5]))
		size := alignedIoStoreBlockSize(entry)
		data := make([]byte, size)
		if _, err := ucas.ReadAt(data, position); err != nil {
			return fmt.Errorf("read encrypted block: %w", err)
		}
		decryptIoStoreBytes(blockCipher, data)
		if _, err := ucas.WriteAt(data, position); err != nil {
			return fmt.Errorf("write decrypted block: %w", err)
		}
	}
	if err := ucas.Close(); err != nil {
		return fmt.Errorf("close decrypted UCAS: %w", err)
	}
	toc[ioStoreFlagsOffset] &^= ioStoreEncrypted
	return os.WriteFile(utocPath, toc, 0o600)
}

func encryptedIoStoreLayout(toc []byte, ucasSize int64) ([]byte, []byte, error) {
	if len(toc) < ioStoreHeaderSize || !bytes.Equal(toc[:16], []byte("-==--==--==--==-")) {
		return nil, nil, fmt.Errorf("invalid IoStore TOC header")
	}
	version := toc[16]
	if version < 1 || version > ioStoreOverflowVersion || binary.LittleEndian.Uint32(toc[20:24]) != ioStoreHeaderSize {
		return nil, nil, fmt.Errorf("unsupported IoStore TOC version or header size")
	}
	if toc[ioStoreFlagsOffset]&ioStoreEncrypted == 0 {
		return nil, nil, fmt.Errorf("IoStore container is not encrypted")
	}
	if toc[ioStoreFlagsOffset]&ioStoreSigned != 0 || binary.LittleEndian.Uint32(toc[52:56]) != 1 {
		return nil, nil, fmt.Errorf("direct decrypt does not support signed or multi-partition containers")
	}
	if binary.LittleEndian.Uint32(toc[32:36]) != ioStoreCompressionEntrySize {
		return nil, nil, fmt.Errorf("unsupported compression block entry size")
	}
	chunkCount := uint64(binary.LittleEndian.Uint32(toc[24:28]))
	blockCount := uint64(binary.LittleEndian.Uint32(toc[28:32]))
	methodCount := uint64(binary.LittleEndian.Uint32(toc[36:40]))
	methodLength := uint64(binary.LittleEndian.Uint32(toc[40:44]))
	directorySize := uint64(binary.LittleEndian.Uint32(toc[48:52]))
	blockOffset := uint64(ioStoreHeaderSize) + chunkCount*(12+10)
	if version >= ioStorePerfectHashVersion {
		blockOffset += uint64(binary.LittleEndian.Uint32(toc[84:88])) * 4
	}
	if version >= ioStoreOverflowVersion {
		blockOffset += uint64(binary.LittleEndian.Uint32(toc[96:100])) * 4
	}
	blockEnd := blockOffset + blockCount*ioStoreCompressionEntrySize
	if (blockCount == 0 && chunkCount != 0) || blockEnd > uint64(len(toc)) || methodCount*methodLength > uint64(len(toc))-blockEnd {
		return nil, nil, fmt.Errorf("invalid IoStore compression table")
	}
	directoryOffset := blockEnd + methodCount*methodLength
	if directorySize > uint64(len(toc))-directoryOffset {
		return nil, nil, fmt.Errorf("truncated IoStore directory index")
	}
	blocks := toc[blockOffset:blockEnd]
	previousEnd := int64(0)
	for offset := 0; offset < len(blocks); offset += ioStoreCompressionEntrySize {
		entry := blocks[offset : offset+ioStoreCompressionEntrySize]
		position := int64(littleEndianBytes(entry[:5]))
		size := int64(alignedIoStoreBlockSize(entry))
		if size == 0 || position < previousEnd || position+size > ucasSize || uint64(entry[11]) > methodCount {
			return nil, nil, fmt.Errorf("invalid or truncated IoStore compression block")
		}
		previousEnd = position + size
	}
	return blocks, toc[directoryOffset : directoryOffset+directorySize], nil
}

func littleEndianBytes(data []byte) uint64 {
	var value uint64
	for i, part := range data {
		value |= uint64(part) << (8 * i)
	}
	return value
}

func alignedIoStoreBlockSize(entry []byte) int {
	size := littleEndianBytes(entry[5:8])
	if entry[11] == 0 {
		size = littleEndianBytes(entry[8:11])
	}
	return int((size + aes.BlockSize - 1) &^ uint64(aes.BlockSize-1))
}

func decryptIoStoreBytes(blockCipher cipher.Block, data []byte) {
	for offset := 0; offset < len(data); offset += aes.BlockSize {
		blockCipher.Decrypt(data[offset:offset+aes.BlockSize], data[offset:offset+aes.BlockSize])
	}
}

func validIoStoreDirectory(data []byte) bool {
	mount, rest, ok := readIoStoreString(data)
	if !ok || (!strings.HasPrefix(mount, "../") && !strings.HasPrefix(mount, "/")) {
		return false
	}
	// Check all array boundaries before treating an obfuscated index as clear text.
	for _, entrySize := range []uint64{16, 12} {
		if len(rest) < 4 {
			return false
		}
		size := uint64(binary.LittleEndian.Uint32(rest[:4])) * entrySize
		if size > uint64(len(rest)-4) {
			return false
		}
		rest = rest[4+size:]
	}
	if len(rest) < 4 {
		return false
	}
	count := binary.LittleEndian.Uint32(rest[:4])
	rest = rest[4:]
	if uint64(count)*4 > uint64(len(rest)) {
		return false
	}
	for range count {
		_, rest, ok = readIoStoreString(rest)
		if !ok {
			return false
		}
	}
	return len(bytes.Trim(rest, "\x00")) == 0
}

func readIoStoreString(data []byte) (string, []byte, bool) {
	if len(data) < 4 {
		return "", nil, false
	}
	length := int64(int32(binary.LittleEndian.Uint32(data[:4])))
	data = data[4:]
	if length == 0 {
		return "", data, true
	}
	if length > 0 {
		if length > int64(len(data)) || data[length-1] != 0 || !utf8.Valid(data[:length-1]) {
			return "", nil, false
		}
		return string(data[:length-1]), data[length:], true
	}
	byteCount := -length * 2
	if byteCount > int64(len(data)) || binary.LittleEndian.Uint16(data[byteCount-2:byteCount]) != 0 {
		return "", nil, false
	}
	characters := make([]uint16, -length-1)
	for i := range characters {
		characters[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return string(utf16.Decode(characters)), data[byteCount:], true
}
