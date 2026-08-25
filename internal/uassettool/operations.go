package uassettool

import "fmt"

// Sends one worker request and decodes its data payload into result. Adapter
// and Worker both implement this, so the operations below run against a fake
// transport in unit tests and a supervised process in integration tests with
// no special-casing.
type caller interface {
	Call(action string, params map[string]any, result any) error
}

// Describes one internal path inside a classic PAK archive.
type PakEntry struct {
	Path           string
	Size           uint64
	CompressedSize uint64
	Encrypted      bool
	Compressed     bool
}

// Lists the internal asset paths inside a classic PAK archive at pakPath.
func ListPak(c caller, pakPath string) ([]PakEntry, error) {
	if pakPath == "" {
		return nil, fmt.Errorf("uassettool: list_pak: pak path is required")
	}

	var raw struct {
		Files []struct {
			Path           string `json:"path"`
			Size           uint64 `json:"size"`
			CompressedSize uint64 `json:"compressed_size"`
			Encrypted      bool   `json:"encrypted"`
			Compressed     bool   `json:"compressed"`
		} `json:"files"`
	}
	if err := c.Call("list_pak", map[string]any{"file_path": pakPath}, &raw); err != nil {
		return nil, err
	}

	entries := make([]PakEntry, 0, len(raw.Files))
	for _, file := range raw.Files {
		if file.Path == "" {
			return nil, fmt.Errorf("%w: list_pak: entry with an empty path", ErrMalformedResponse)
		}
		entries = append(entries, PakEntry{
			Path:           file.Path,
			Size:           file.Size,
			CompressedSize: file.CompressedSize,
			Encrypted:      file.Encrypted,
			Compressed:     file.Compressed,
		})
	}
	return entries, nil
}

// Reports whether the IoStore container at utocPath is encrypted.
func IsIoStoreEncrypted(c caller, utocPath string) (bool, error) {
	if utocPath == "" {
		return false, fmt.Errorf("uassettool: is_iostore_encrypted: utoc path is required")
	}

	var raw struct {
		Encrypted bool `json:"encrypted"`
	}
	if err := c.Call("is_iostore_encrypted", map[string]any{"file_path": utocPath}, &raw); err != nil {
		return false, err
	}
	return raw.Encrypted, nil
}
