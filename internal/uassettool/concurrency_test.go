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
