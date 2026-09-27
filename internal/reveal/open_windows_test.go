//go:build windows

package reveal

import "testing"

func TestExplorerCommandQuotesPathsWithoutQuotingSelect(t *testing.T) {
	for _, test := range []struct {
		name   string
		target Target
		want   string
	}{
		{
			name:   "mod with spaces",
			target: Target{Path: `C:\Mod library\Hero skin.pak`, SelectItem: true},
			want:   `explorer.exe /select,"C:\Mod library\Hero skin.pak"`,
		},
		{
			name:   "mod without spaces",
			target: Target{Path: `C:\Mods\Hero.pak`, SelectItem: true},
			want:   `explorer.exe /select,"C:\Mods\Hero.pak"`,
		},
		{
			name:   "disabled mod with comma and Unicode",
			target: Target{Path: `C:\Mods\Héro,青.pak.disabled`, SelectItem: true},
			want:   `explorer.exe /select,"C:\Mods\Héro,青.pak.disabled"`,
		},
		{
			name:   "folder with spaces",
			target: Target{Path: `C:\Mod library\Hero skins`},
			want:   `explorer.exe "C:\Mod library\Hero skins"`,
		},
		{
			name:   "folder with comma",
			target: Target{Path: `C:\Mods\Hero,skins`},
			want:   `explorer.exe "C:\Mods\Hero,skins"`,
		},
		{
			name:   "drive root",
			target: Target{Path: `C:\`},
			want:   `explorer.exe "C:\"`,
		},
		{
			name:   "UNC path",
			target: Target{Path: `\\server\Mod library\Hero.pak`, SelectItem: true},
			want:   `explorer.exe /select,"\\server\Mod library\Hero.pak"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange / Act
			cmd := explorerCommand(test.target)

			// Assert
			if cmd.SysProcAttr == nil {
				t.Fatal("Explorer command uses Go argument quotes, want an explicit command line")
			}
			if got := cmd.SysProcAttr.CmdLine; got != test.want {
				t.Errorf("command line = %q, want %q", got, test.want)
			}
		})
	}
}
