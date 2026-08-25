package uassettool

import "runtime"

const (
	// Upper bound on a worker pool when the caller has no library-size
	// information to size it by (WorkerPoolSize/DefaultWorkerPoolSize).
	// Per docs/decisions/0003-uassettoolrivals-boundary.md, this matches
	// what the benchmark found sufficient for libraries under
	// smallLibraryThreshold entries; larger libraries benefit from a higher
	// cap, which WorkerPoolSizeForLibrary provides.
	maxWorkerPoolSize = 4

	// Smallest allowed worker pool: no pooling at all.
	minWorkerPoolSize = 1

	// Job-count tier boundaries and their pool-size caps, from the task 6.7
	// benchmark table in docs/decisions/0003-uassettoolrivals-boundary.md
	// (measured at 72, 504, 864, and 2592 real mod entries). The boundaries
	// are interpolated between measured sizes, not measured exactly; a
	// wider spread of measured library sizes could refine them further.
	smallLibraryThreshold = 700  // below this, a pool of 4 already captures the measured benefit
	largeLibraryThreshold = 1500 // at or above this, pooling benefits from a cap of 16

	mediumLibraryPoolCap = 8
	largeLibraryPoolCap  = 16
)

// Computes half of availableCores, backing off further if that lands
// exactly on cap (so a machine that just barely qualifies for the ceiling
// is not immediately asked to run at it), floored at minWorkerPoolSize.
func coreAwarePoolSize(availableCores, cap int) int {
	halfCores := availableCores / 2

	size := halfCores
	switch {
	case halfCores > cap:
		size = cap
	case halfCores == cap:
		size = halfCores / 2
	}

	if size < minWorkerPoolSize {
		return minWorkerPoolSize
	}
	return size
}

// Chooses a worker pool size from the number of CPU cores available, capped
// at maxWorkerPoolSize. Use this when the caller does not know how many
// entries it will process; use WorkerPoolSizeForLibrary when it does, since
// larger libraries benefit from a higher cap than this function allows.
// availableCores is a parameter, rather than this function reading
// runtime.NumCPU() itself, so callers and tests can exercise every branch
// deterministically; production callers should use DefaultWorkerPoolSize.
func WorkerPoolSize(availableCores int) int {
	return coreAwarePoolSize(availableCores, maxWorkerPoolSize)
}

// Chooses a worker pool size for the machine Cratebug is running on.
func DefaultWorkerPoolSize() int {
	return WorkerPoolSize(runtime.NumCPU())
}

// Chooses a worker pool size from both the number of CPU cores available
// and the number of entries the pool will process, following the
// entry-count tiers in docs/decisions/0003-uassettoolrivals-boundary.md.
// Larger libraries benefit from a higher pool-size ceiling, but the result
// never exceeds what coreAwarePoolSize allows for the machine either way.
func WorkerPoolSizeForLibrary(availableCores, entryCount int) int {
	return coreAwarePoolSize(availableCores, poolCapForEntryCount(entryCount))
}

// Chooses a worker pool size for the machine Cratebug is running on and a
// library of entryCount mod entries.
func DefaultWorkerPoolSizeForLibrary(entryCount int) int {
	return WorkerPoolSizeForLibrary(runtime.NumCPU(), entryCount)
}

// Returns the pool-size ceiling appropriate for a library of this size, per
// the entry-count tiers above.
func poolCapForEntryCount(entryCount int) int {
	switch {
	case entryCount < smallLibraryThreshold:
		return maxWorkerPoolSize
	case entryCount < largeLibraryThreshold:
		return mediumLibraryPoolCap
	default:
		return largeLibraryPoolCap
	}
}
