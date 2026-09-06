package modtype

import "testing"

func TestRequiresIoStoreEncryption(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  bool
	}{
		{"characters only", []string{"Characters/1011/Meshes/SK_Hulk.uasset"}, false},
		{"marvel content prefix", []string{"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset"}, false},
		{"game marvel prefix", []string{"/Game/Marvel/Characters/1011/Meshes/SK_Hulk.uasset"}, false},
		{"marvel content without extra marvel", []string{"Marvel/Content/Characters/1044/SK_Blade.uasset"}, false},
		{"extensionless characters", []string{"/Game/Marvel/Characters/1011/Meshes/SK_Hulk"}, false},
		{"ui", []string{"UI/Icons/Icon.uasset"}, true},
		{"game marvel ui", []string{"/Game/Marvel/UI/Icons/Icon.uasset"}, true},
		{"audio", []string{"WwiseAudio/Media/Sound.bnk"}, true},
		{"game audio", []string{"/Game/WwiseAudio/Media/Sound.bnk"}, true},
		{"mixed characters and ui", []string{
			"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset",
			"/Game/Marvel/UI/Icons/Icon.uasset",
		}, true},
		{"empty listing", nil, false},
		{"only companion metadata", []string{"../../../chunknames", "patched_files"}, false},
		{"metadata plus characters", []string{
			"Characters/1011/Meshes/SK_Hulk.uasset",
			"../../../chunknames",
		}, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Act
			got := RequiresIoStoreEncryption(testCase.paths)

			// Assert
			if got != testCase.want {
				t.Errorf("RequiresIoStoreEncryption(%v) = %v, want %v", testCase.paths, got, testCase.want)
			}
		})
	}
}
