package nexus

import (
	"errors"
	"strings"
	"testing"
)

func TestParseDownloadURL(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		want        DownloadRequest
		wantSecrets DownloadSecrets
		wantErr     error
		wantErrSub  string
	}{
		{
			name: "premium nxm",
			raw:  "nxm://marvelrivals/mods/5678/files/90",
			want: DownloadRequest{Game: GameDomain, ModID: 5678, FileID: 90, Premium: true},
		},
		{
			name:        "free nxm with key and expires",
			raw:         "nxm://marvelrivals/mods/5678/files/90?key=abc&expires=1700000000&user_id=12",
			want:        DownloadRequest{Game: GameDomain, ModID: 5678, FileID: 90, Premium: false},
			wantSecrets: DownloadSecrets{Key: "abc", Expires: "1700000000"},
		},
		{
			name:       "wrong scheme",
			raw:        "https://www.nexusmods.com/marvelrivals/mods/1/files/2",
			wantErrSub: "scheme",
		},
		{
			name:    "wrong game",
			raw:     "nxm://skyrim/mods/1/files/2",
			wantErr: ErrWrongGame,
		},
		{
			name:       "collections",
			raw:        "nxm://marvelrivals/collections/cool-pack/revisions/1",
			wantErrSub: "collection",
		},
		{
			name:       "oauth",
			raw:        "nxm://oauth/callback",
			wantErrSub: "OAuth",
		},
		{
			name:       "premium host",
			raw:        "nxm://premium",
			wantErrSub: "premium",
		},
		{
			name:       "missing segments",
			raw:        "nxm://marvelrivals/mods/1",
			wantErrSub: "missing",
		},
		{
			name:       "non-numeric ids",
			raw:        "nxm://marvelrivals/mods/abc/files/def",
			wantErrSub: "missing",
		},
		{
			name:       "absurd length",
			raw:        "nxm://marvelrivals/mods/1/files/2?" + strings.Repeat("x", maxRawURLBytes),
			wantErrSub: "exceeds",
		},
		{
			name:       `embedded "`,
			raw:        `nxm://marvelrivals/mods/1/files/2"?key=x`,
			wantErrSub: "unsafe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, secrets, err := ParseDownloadURL(tt.raw)

			// Assert
			if tt.wantErr != nil || tt.wantErrSub != "" {
				if err == nil {
					t.Fatal("ParseDownloadURL() succeeded, want an error")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErrSub)
				}
				if tt.wantErr == ErrWrongGame && !strings.Contains(err.Error(), "skyrim") {
					t.Fatalf("ErrWrongGame text = %q, want it to name the domain", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDownloadURL() = %v, want no error", err)
			}
			if got != tt.want {
				t.Errorf("DownloadRequest = %+v, want %+v", got, tt.want)
			}
			if secrets != tt.wantSecrets {
				t.Errorf("DownloadSecrets = %+v, want %+v", secrets, tt.wantSecrets)
			}
		})
	}
}

func TestParseModPageURL(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		want       ModPage
		wantErr    error
		wantErrSub string
	}{
		{
			name: "short path without file_id",
			raw:  "https://www.nexusmods.com/marvelrivals/mods/5678",
			want: ModPage{Game: GameDomain, ModID: 5678},
		},
		{
			name: "games path without file_id",
			raw:  "https://www.nexusmods.com/games/marvelrivals/mods/5678",
			want: ModPage{Game: GameDomain, ModID: 5678},
		},
		{
			name: "short path with file_id query",
			raw:  "https://www.nexusmods.com/marvelrivals/mods/5678?tab=files&file_id=90",
			want: ModPage{Game: GameDomain, ModID: 5678, FileID: 90},
		},
		{
			name: "games path with files segment",
			raw:  "https://www.nexusmods.com/games/marvelrivals/mods/5678/files/90",
			want: ModPage{Game: GameDomain, ModID: 5678, FileID: 90},
		},
		{
			name:    "wrong game on short path",
			raw:     "https://www.nexusmods.com/skyrim/mods/1",
			wantErr: ErrWrongGame,
		},
		{
			name:    "wrong game on games path",
			raw:     "https://www.nexusmods.com/games/skyrim/mods/1",
			wantErr: ErrWrongGame,
		},
		{
			name:       "http is rejected",
			raw:        "http://www.nexusmods.com/marvelrivals/mods/5678",
			wantErrSub: "HTTPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := ParseModPageURL(tt.raw)

			// Assert
			if tt.wantErr != nil || tt.wantErrSub != "" {
				if err == nil {
					t.Fatal("ParseModPageURL() succeeded, want an error")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseModPageURL() = %v, want no error", err)
			}
			if got != tt.want {
				t.Errorf("ModPage = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFirstDownloadURL(t *testing.T) {
	valid := "nxm://marvelrivals/mods/1/files/2"
	second := "nxm://marvelrivals/mods/3/files/4"

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty", args: nil, want: ""},
		{name: "one", args: []string{valid}, want: valid},
		{name: "not-at-index-0", args: []string{`C:\Program Files\Cratebug\Cratebug.exe`, valid}, want: valid},
		{name: "two", args: []string{valid, second}, want: valid},
		{name: "Windows path", args: []string{`C:\Users\mew\mod.zip`}, want: ""},
		{name: "--uninstall-cleanup", args: []string{`C:\Cratebug\Cratebug.exe`, "--uninstall-cleanup"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := FirstDownloadURL(tt.args)

			// Assert
			if got != tt.want {
				t.Fatalf("FirstDownloadURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactURL(t *testing.T) {
	t.Run("unparseable returns placeholder", func(t *testing.T) {
		// Arrange
		raw := "http://[::1"

		// Act
		got := redactURL(raw)

		// Assert
		if got != unparseableURL {
			t.Fatalf("redactURL() = %q, want %q", got, unparseableURL)
		}
		if strings.Contains(got, raw) {
			t.Fatal("redactURL echoed the unparseable input")
		}
	})

	t.Run("query is replaced by structure", func(t *testing.T) {
		// Arrange
		raw := "nxm://marvelrivals/mods/1/files/2?key=SECRET&expires=99"

		// Act
		got := redactURL(raw)

		// Assert
		if got != "nxm://marvelrivals/mods/1/files/2?<redacted>" {
			t.Fatalf("redactURL() = %q, want scheme://host/path?<redacted>", got)
		}
		if strings.Contains(got, "SECRET") || strings.Contains(got, "expires=99") || strings.Contains(got, "key=") {
			t.Fatalf("redactURL leaked query values: %q", got)
		}
	})

	t.Run("no query keeps path only", func(t *testing.T) {
		// Act
		got := redactURL("nxm://marvelrivals/mods/1/files/2")

		// Assert
		if got != "nxm://marvelrivals/mods/1/files/2" {
			t.Fatalf("redactURL() = %q, want the path without a query marker", got)
		}
	})
}
