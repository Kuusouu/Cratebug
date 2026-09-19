package nexus

import (
	"errors"
	"testing"
)

func TestAllowAdultContent(t *testing.T) {
	prefsErr := errors.New("preferences unavailable")
	adultMod := ModInfo{ContainsAdultContent: true}
	safeMod := ModInfo{ContainsAdultContent: false}

	tests := []struct {
		name     string
		mod      ModInfo
		prefs    Preferences
		prefsErr error
		want     error
	}{
		{
			name:     "non-adult ignores preference errors",
			mod:      safeMod,
			prefsErr: prefsErr,
		},
		{
			name:  "adult allowed when preference is on",
			mod:   adultMod,
			prefs: Preferences{Adult: true},
		},
		{
			name:  "adult blocked when preference is off",
			mod:   adultMod,
			prefs: Preferences{Adult: false},
			want:  ErrAdultContentBlocked,
		},
		{
			name:     "adult blocked when preference is unreadable",
			mod:      adultMod,
			prefs:    Preferences{Adult: true},
			prefsErr: prefsErr,
			want:     ErrAdultContentBlocked,
		},
		{
			name:  "adult blocked when content blocking is on",
			mod:   adultMod,
			prefs: Preferences{Adult: true, IsBlockingContent: true},
			want:  ErrAdultContentBlocked,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			mod := test.mod
			prefs := test.prefs
			prefsErr := test.prefsErr

			// Act
			err := AllowAdultContent(mod, prefs, prefsErr)

			// Assert
			if !errors.Is(err, test.want) {
				t.Fatalf("AllowAdultContent() = %v, want %v", err, test.want)
			}
		})
	}
}
