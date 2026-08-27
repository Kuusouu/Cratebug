package update

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		wantErr    bool
		wantYear   int
		wantMonth  int
		wantDay    int
		wantSuffix string
	}{
		{name: "stable release", tag: "2026.08.27", wantYear: 2026, wantMonth: 8, wantDay: 27},
		{name: "prerelease", tag: "2026.08.27-rc1", wantYear: 2026, wantMonth: 8, wantDay: 27, wantSuffix: "rc1"},
		{name: "multi-word prerelease suffix", tag: "2026.01.05-beta.2", wantYear: 2026, wantMonth: 1, wantDay: 5, wantSuffix: "beta.2"},
		{name: "dev placeholder is not a valid tag", tag: "dev", wantErr: true},
		{name: "unpadded month is rejected", tag: "2026.8.27", wantErr: true},
		{name: "empty string", tag: "", wantErr: true},
		{name: "trailing dot", tag: "2026.08.27.", wantErr: true},
		{name: "leading v is rejected", tag: "v2026.08.27", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.tag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseVersion(%q) = %+v, want error", tt.tag, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q) returned unexpected error: %v", tt.tag, err)
			}
			if got.Year != tt.wantYear || got.Month != tt.wantMonth || got.Day != tt.wantDay || got.Prerelease != tt.wantSuffix {
				t.Fatalf("ParseVersion(%q) = %+v, want Year=%d Month=%d Day=%d Prerelease=%q",
					tt.tag, got, tt.wantYear, tt.wantMonth, tt.wantDay, tt.wantSuffix)
			}
			if got.Tag != tt.tag {
				t.Fatalf("ParseVersion(%q).Tag = %q, want the original tag preserved", tt.tag, got.Tag)
			}
		})
	}
}

func mustParse(t *testing.T, tag string) Version {
	t.Helper()
	v, err := ParseVersion(tag)
	if err != nil {
		t.Fatalf("ParseVersion(%q) failed: %v", tag, err)
	}
	return v
}

func TestVersionIsNewer(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "later day beats earlier day", a: "2026.08.27", b: "2026.08.26", want: true},
		{name: "earlier day loses to later day", a: "2026.08.26", b: "2026.08.27", want: false},
		{name: "later month beats earlier month regardless of day", a: "2026.09.01", b: "2026.08.30", want: true},
		{name: "later year beats everything", a: "2027.01.01", b: "2026.12.31", want: true},
		{name: "identical stable tags are not newer", a: "2026.08.27", b: "2026.08.27", want: false},
		{name: "stable beats prerelease of same date", a: "2026.08.27", b: "2026.08.27-rc1", want: true},
		{name: "prerelease loses to stable of same date", a: "2026.08.27-rc1", b: "2026.08.27", want: false},
		{name: "higher rc beats lower rc of same date", a: "2026.08.27-rc2", b: "2026.08.27-rc1", want: true},
		{name: "identical prereleases are not newer", a: "2026.08.27-rc1", b: "2026.08.27-rc1", want: false},
		{name: "stable of later date beats prerelease of earlier date", a: "2026.08.28", b: "2026.08.27-rc9", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustParse(t, tt.a)
			b := mustParse(t, tt.b)
			if got := a.IsNewer(b); got != tt.want {
				t.Fatalf("Version(%q).IsNewer(%q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
