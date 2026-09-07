package install

import "testing"

func TestIsSupportedInstallFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "Mod.zip", want: true},
		{name: "Mod.7z", want: true},
		{name: "Example_9999999_P.pak", want: true},
		{name: "Example_9999999_P.pak_crateoff", want: true},
		{name: "Example_9999999_P.utoc", want: true},
		{name: "readme.txt", want: false},
		{name: "setup.exe", want: false},
		{name: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := IsSupportedInstallFile(tt.name)

			// Assert
			if got != tt.want {
				t.Fatalf("IsSupportedInstallFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
