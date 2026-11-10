package patchupdate

import "testing"

func TestBundleStatusCountsAmbiguousSeparately(t *testing.T) {
	tests := []struct {
		name      string
		updated   int
		blocked   int
		ambiguous int
		want      BundleStatus
	}{
		{name: "nothing to do", want: BundleCurrent},
		{name: "some updated", updated: 1, want: BundleUpdated},
		{name: "some updated and some blocked", updated: 1, blocked: 1, want: BundlePartial},
		{name: "only blocked", blocked: 1, want: BundleBlocked},
		{name: "only ambiguous", ambiguous: 1, want: BundleAmbiguous},
		{name: "ambiguous does not hide an update", updated: 1, ambiguous: 1, want: BundleUpdated},
		{name: "blocked wins over ambiguous", blocked: 1, ambiguous: 1, want: BundleBlocked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := bundleStatus(test.updated, test.blocked, test.ambiguous); got != test.want {
				t.Errorf("bundleStatus(%d, %d, %d) = %q, want %q", test.updated, test.blocked, test.ambiguous, got, test.want)
			}
		})
	}
}
