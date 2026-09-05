package uassettool

import (
	"fmt"
	"strings"
)

// Marvel Rivals ships IoStore containers under this AES-256 key. BentoMod
// hardcodes the same value so obfuscated mods stay playable. The key is a
// well-known public constant, not a secret, and must stay in this package.
const MarvelRivalsAESKey = "0C263D8C22DCB085894899C3A3796383E9BF9DE0CBFB08C9BF2DEF2E84F29D74"

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
	return ListPakWithKey(c, pakPath, "")
}

// Lists a PAK and, when aesKey is set, decrypts an encrypted index with it.
func ListPakWithKey(c caller, pakPath, aesKey string) ([]PakEntry, error) {
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
	params := map[string]any{"file_path": pakPath}
	if aesKey != "" {
		params["aes_key"] = aesKey
	}
	if err := c.Call("list_pak", params, &raw); err != nil {
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

// Lists the resolved internal asset paths inside the IoStore container at
// utocPath. aesKey may be empty for an unencrypted container. Encrypted
// containers need MarvelRivalsAESKey.
func ListIoStoreFiles(c caller, utocPath, aesKey string) ([]string, error) {
	if utocPath == "" {
		return nil, fmt.Errorf("uassettool: list_iostore_files: utoc path is required")
	}

	params := map[string]any{"file_path": utocPath}
	if aesKey != "" {
		params["aes_key"] = aesKey
	}

	var raw struct {
		Files []string `json:"files"`
	}
	if err := c.Call("list_iostore_files", params, &raw); err != nil {
		return nil, err
	}

	for _, path := range raw.Files {
		if path == "" {
			return nil, fmt.Errorf("%w: list_iostore_files: entry with an empty path", ErrMalformedResponse)
		}
	}
	return raw.Files, nil
}

// Fields a Cratebug caller may set on create_mod_iostore. Obfuscate AES-wraps
// the output with the game key. Hybrid keeps non-Unreal files in the companion PAK.
// InputPak, when set, is the worker's extract-then-create path. It trims
// leading slashes that extract_pak_all drops on Windows.
type IoStoreCreateOptions struct {
	MountPoint string
	Compress   bool
	AESKey     string
	Obfuscate  bool
	Hybrid     bool
	InputPak   string
}

// Absolute paths and counts returned after a successful create_mod_iostore.
type IoStoreCreateResult struct {
	UTOCPath       string
	UCASPath       string
	PAKPath        string
	ConvertedCount int
	FileCount      int
}

// Extracts an IoStore container to loose legacy files at outputPath.
func ExtractIoStore(c caller, utocPath, outputPath, aesKey string) (int, error) {
	if utocPath == "" {
		return 0, fmt.Errorf("uassettool: extract_iostore: utoc path is required")
	}
	if outputPath == "" {
		return 0, fmt.Errorf("uassettool: extract_iostore: output path is required")
	}

	params := map[string]any{
		"file_path":   utocPath,
		"output_path": outputPath,
	}
	if aesKey != "" {
		params["aes_key"] = aesKey
	}

	var raw struct {
		Count int `json:"count"`
	}
	if err := c.Call("extract_iostore", params, &raw); err != nil {
		return 0, err
	}
	return raw.Count, nil
}

// Extracts every file from a PAK into outputPath.
func ExtractPakAll(c caller, pakPath, outputPath, aesKey string) (int, error) {
	if pakPath == "" {
		return 0, fmt.Errorf("uassettool: extract_pak_all: pak path is required")
	}
	if outputPath == "" {
		return 0, fmt.Errorf("uassettool: extract_pak_all: output path is required")
	}

	params := map[string]any{
		"file_path":   pakPath,
		"output_path": outputPath,
	}
	if aesKey != "" {
		params["aes_key"] = aesKey
	}

	var raw struct {
		ExtractedCount int `json:"extracted_count"`
	}
	if err := c.Call("extract_pak_all", params, &raw); err != nil {
		return 0, err
	}
	return raw.ExtractedCount, nil
}

// Builds an IoStore bundle from a directory of loose files, or from InputPak.
func CreateModIoStore(c caller, outputPath, inputDir string, options IoStoreCreateOptions) (IoStoreCreateResult, error) {
	if outputPath == "" {
		return IoStoreCreateResult{}, fmt.Errorf("uassettool: create_mod_iostore: output path is required")
	}
	if inputDir == "" && options.InputPak == "" {
		return IoStoreCreateResult{}, fmt.Errorf("uassettool: create_mod_iostore: input directory or input PAK is required")
	}

	params := map[string]any{
		"output_path": outputPath,
		"obfuscate":   options.Obfuscate,
		"hybrid":      options.Hybrid,
	}
	if inputDir != "" {
		params["input_dir"] = inputDir
	}
	if options.InputPak != "" {
		params["input_pak"] = options.InputPak
	}
	if options.MountPoint != "" {
		params["mount_point"] = options.MountPoint
	}
	if options.AESKey != "" {
		params["aes_key"] = options.AESKey
	}
	if options.Compress {
		params["compress"] = true
	}

	var raw struct {
		UTOCPath       string `json:"utoc_path"`
		UCASPath       string `json:"ucas_path"`
		PAKPath        string `json:"pak_path"`
		ConvertedCount int    `json:"converted_count"`
		FileCount      int    `json:"file_count"`
	}
	if err := c.Call("create_mod_iostore", params, &raw); err != nil {
		return IoStoreCreateResult{}, err
	}
	return IoStoreCreateResult{
		UTOCPath:       raw.UTOCPath,
		UCASPath:       raw.UCASPath,
		PAKPath:        raw.PAKPath,
		ConvertedCount: raw.ConvertedCount,
		FileCount:      raw.FileCount,
	}, nil
}

// Reports whether a companion PAK listing contains raw files that must be
// kept with hybrid:true on recreate. Chunknames and patched_files metadata
// do not count.
func CompanionPakHasRawFiles(entries []PakEntry) bool {
	for _, entry := range entries {
		lower := strings.ToLower(entry.Path)
		if strings.Contains(lower, "chunknames") || strings.Contains(lower, "patched_files") {
			continue
		}
		return true
	}
	return false
}
