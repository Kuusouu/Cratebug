package modtype

import (
	"log"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// PoolWorker defines the operations required by a pooled worker process.
// *uassettool.Worker satisfies this interface structurally.
type PoolWorker interface {
	caller
	Alive() bool
	Close() error
}

// WorkerLauncher is a factory function that produces a new PoolWorker.
type WorkerLauncher func() (PoolWorker, error)

// DefaultWorkerLauncher creates workers using the pinned UAssetTool binary.
func DefaultWorkerLauncher(logger *log.Logger) WorkerLauncher {
	return func() (PoolWorker, error) {
		return uassettool.NewPinnedWorker(logger)
	}
}

type classifyJob struct {
	root   string
	entry  discovery.Entry
	mtime  time.Time
	table  CharacterTable
	result chan<- classifyOutcome
}

type classifyOutcome struct {
	entryID  string
	mtime    time.Time
	identity Identity
}

// WorkerPool manages a fixed-size pool of worker processes executing
// classification jobs pulled from a shared buffered channel.
type WorkerPool struct {
	size      int
	launcher  WorkerLauncher
	jobs      chan classifyJob
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewWorkerPool creates and starts a pool of worker goroutines.
func NewWorkerPool(size int, launcher WorkerLauncher) *WorkerPool {
	if size < 1 {
		size = 1
	}

	pool := &WorkerPool{
		size:     size,
		launcher: launcher,
		jobs:     make(chan classifyJob, size*2),
	}

	for i := 0; i < size; i++ {
		pool.wg.Add(1)
		go pool.workerLoop()
	}

	return pool
}

func (p *WorkerPool) workerLoop() {
	defer p.wg.Done()

	var w PoolWorker
	if p.launcher != nil {
		w, _ = p.launcher()
	}
	defer func() {
		if w != nil {
			_ = w.Close()
		}
	}()

	for job := range p.jobs {
		// If worker is dead or failed to start initially, try to replace it
		if (w == nil || !w.Alive()) && p.launcher != nil {
			if w != nil {
				_ = w.Close()
			}
			newW, err := p.launcher()
			if err == nil {
				w = newW
			} else {
				w = nil
			}
		}

		var identity Identity
		if w != nil && w.Alive() {
			id, err := DetermineIdentity(w, job.root, job.entry, job.table)
			if err != nil {
				identity = Identity{Category: CategoryUnknown}
			} else {
				identity = id
			}
		} else {
			identity = Identity{Category: CategoryUnknown}
		}

		job.result <- classifyOutcome{
			entryID:  job.entry.ID,
			mtime:    job.mtime,
			identity: identity,
		}
	}
}

// Size returns the configured worker count of the pool.
func (p *WorkerPool) Size() int {
	return p.size
}

// Submit enqueues a classification job to the worker pool.
func (p *WorkerPool) Submit(job classifyJob) {
	p.jobs <- job
}

// Close closes the job queue and waits for all worker goroutines and processes to cleanly terminate.
func (p *WorkerPool) Close() error {
	p.closeOnce.Do(func() {
		close(p.jobs)
		p.wg.Wait()
	})
	return nil
}
