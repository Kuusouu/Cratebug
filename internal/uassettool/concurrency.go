package uassettool

import "runtime"

const (
	// Upper bound on any UAssetToolRivals worker pool. Per the concurrency
	// policy recorded in docs/decisions/0003-uassettoolrivals-boundary.md,
	// pooling stopped helping past this many workers for both benchmarked
	// operation classes, and made the cheap, header-only operations this
	// package exposes actively worse beyond it.
	maxWorkerPoolSize = 4

	// Smallest allowed worker pool: no pooling at all.
	minWorkerPoolSize = 1
)

// Chooses a worker pool size from the number of CPU cores available,
// following the policy in docs/decisions/0003-uassettoolrivals-boundary.md:
// never pool past maxWorkerPoolSize, and back off further on machines with
// few cores so the pool does not dominate the whole CPU. Below the cap, the
// pool tracks half the available cores; at exactly the cap, it backs off to
// half of that instead of hammering a modest machine with the maximum pool
// size. availableCores is a parameter, rather than this function reading
// runtime.NumCPU() itself, so callers and tests can exercise every branch
// deterministically; production callers should use DefaultWorkerPoolSize.
func WorkerPoolSize(availableCores int) int {
	halfCores := availableCores / 2

	size := halfCores
	switch {
	case halfCores > maxWorkerPoolSize:
		size = maxWorkerPoolSize
	case halfCores == maxWorkerPoolSize:
		size = halfCores / 2
	}

	if size < minWorkerPoolSize {
		return minWorkerPoolSize
	}
	return size
}

// Chooses a worker pool size for the machine Cratebug is running on.
func DefaultWorkerPoolSize() int {
	return WorkerPoolSize(runtime.NumCPU())
}
