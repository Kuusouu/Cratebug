package uassettool

import "testing"

func TestWorkerPoolSizeFollowsCoreCountPolicy(t *testing.T) {
	cases := []struct {
		name            string
		availableCores  int
		wantPoolSize    int
		wantDescription string
	}{
		{"no reported cores clamps to the minimum", 0, 1, "half is 0, below the cap, clamped to the floor"},
		{"single core clamps to the minimum", 1, 1, "half is 0, below the cap, clamped to the floor"},
		{"two cores", 2, 1, "half is 1, below the cap"},
		{"four cores", 4, 2, "half is 2, below the cap"},
		{"six cores", 6, 3, "half is 3, below the cap"},
		{"eight cores hits the cap exactly", 8, 2, "half is 4, exactly the cap, backs off to half of that"},
		{"ten cores exceeds the cap", 10, 4, "half is 5, above the cap, clamped to the cap"},
		{"sixteen cores exceeds the cap", 16, 4, "half is 8, above the cap, clamped to the cap"},
		{"negative core count clamps to the minimum", -4, 1, "half is negative, clamped to the floor"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Act
			got := WorkerPoolSize(testCase.availableCores)

			// Assert
			if got != testCase.wantPoolSize {
				t.Errorf("WorkerPoolSize(%d) = %d, want %d (%s)", testCase.availableCores, got, testCase.wantPoolSize, testCase.wantDescription)
			}
		})
	}
}

func TestWorkerPoolSizeNeverExceedsTheDocumentedCap(t *testing.T) {
	for cores := -8; cores <= 256; cores++ {
		if size := WorkerPoolSize(cores); size > maxWorkerPoolSize || size < minWorkerPoolSize {
			t.Fatalf("WorkerPoolSize(%d) = %d, want a value between %d and %d inclusive", cores, size, minWorkerPoolSize, maxWorkerPoolSize)
		}
	}
}

func TestDefaultWorkerPoolSizeMatchesRuntimeCores(t *testing.T) {
	// Act
	got := DefaultWorkerPoolSize()

	// Assert
	if got < minWorkerPoolSize || got > maxWorkerPoolSize {
		t.Errorf("DefaultWorkerPoolSize() = %d, want a value between %d and %d inclusive", got, minWorkerPoolSize, maxWorkerPoolSize)
	}
}

func TestPoolCapForEntryCountFollowsSizeTiers(t *testing.T) {
	cases := []struct {
		name       string
		entryCount int
		wantCap    int
	}{
		{"tiny library", 72, maxWorkerPoolSize},
		{"just under the small threshold", smallLibraryThreshold - 1, maxWorkerPoolSize},
		{"at the small threshold", smallLibraryThreshold, mediumLibraryPoolCap},
		{"medium library", 864, mediumLibraryPoolCap},
		{"just under the large threshold", largeLibraryThreshold - 1, mediumLibraryPoolCap},
		{"at the large threshold", largeLibraryThreshold, largeLibraryPoolCap},
		{"large library", 2592, largeLibraryPoolCap},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Act
			got := poolCapForEntryCount(testCase.entryCount)

			// Assert
			if got != testCase.wantCap {
				t.Errorf("poolCapForEntryCount(%d) = %d, want %d", testCase.entryCount, got, testCase.wantCap)
			}
		})
	}
}

func TestWorkerPoolSizeForLibraryScalesWithEntryCountOnAPlentifulMachine(t *testing.T) {
	// A high core count so the core-aware ceiling never binds first, isolating
	// the entry-count tier's own effect on the result.
	const plentifulCores = 128

	cases := []struct {
		name       string
		entryCount int
		wantSize   int
	}{
		{"tiny library capped at 4", 72, maxWorkerPoolSize},
		{"medium library capped at 8", 864, mediumLibraryPoolCap},
		{"large library capped at 16", 2592, largeLibraryPoolCap},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Act
			got := WorkerPoolSizeForLibrary(plentifulCores, testCase.entryCount)

			// Assert
			if got != testCase.wantSize {
				t.Errorf("WorkerPoolSizeForLibrary(%d, %d) = %d, want %d", plentifulCores, testCase.entryCount, got, testCase.wantSize)
			}
		})
	}
}

// A large library's higher cap must never push the result past what the
// machine's own core count justifies; a modest machine stays modest
// regardless of how much work is queued.
func TestWorkerPoolSizeForLibraryNeverExceedsTheCoreAwareCeiling(t *testing.T) {
	// Act
	got := WorkerPoolSizeForLibrary(4, 2592)

	// Assert
	want := coreAwarePoolSize(4, largeLibraryPoolCap)
	if got != want {
		t.Errorf("WorkerPoolSizeForLibrary(4, 2592) = %d, want %d", got, want)
	}
	if got >= largeLibraryPoolCap {
		t.Errorf("WorkerPoolSizeForLibrary(4, 2592) = %d, want it bounded well below the large-library cap of %d on a 4-core machine", got, largeLibraryPoolCap)
	}
}

func TestDefaultWorkerPoolSizeForLibraryMatchesRuntimeCores(t *testing.T) {
	// Act
	got := DefaultWorkerPoolSizeForLibrary(2592)

	// Assert
	if got < minWorkerPoolSize || got > largeLibraryPoolCap {
		t.Errorf("DefaultWorkerPoolSizeForLibrary(2592) = %d, want a value between %d and %d inclusive", got, minWorkerPoolSize, largeLibraryPoolCap)
	}
}
