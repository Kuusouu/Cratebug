package watcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	defaultDebounceDuration = 300 * time.Millisecond
	defaultResumeQuietDelay = 250 * time.Millisecond
)

var recognizedExtensions = []string{
	".pak_crateoff",
	".bak_bento",
	".pak_disabled",
	".pak",
	".utoc",
	".ucas",
}

// Options configures a Watcher instance.
type Options struct {
	Debounce time.Duration
	OnChange func()
}

// Watcher monitors a mod directory recursively and emits debounced change notifications.
type Watcher struct {
	mu           sync.Mutex
	fsWatcher    *fsnotify.Watcher
	root         string
	debounce     time.Duration
	onChange     func()
	timer        *time.Timer
	trackedDirs  map[string]struct{}
	closed       bool
	paused       bool
	ignoredUntil time.Time
}

// New creates a new filesystem watcher.
func New(opts Options) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = defaultDebounceDuration
	}

	w := &Watcher{
		fsWatcher:   fsw,
		debounce:    debounce,
		onChange:    opts.OnChange,
		trackedDirs: make(map[string]struct{}),
	}

	go w.readLoop()
	return w, nil
}

// SetRoot changes the watched root directory. Passing an empty string stops watching.
func (w *Watcher) SetRoot(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("watcher is closed")
	}

	w.clearWatchesLocked()

	cleanRoot := strings.TrimSpace(root)
	if cleanRoot == "" {
		w.root = ""
		return nil
	}

	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return fmt.Errorf("resolve watch root: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			w.root = ""
			return nil
		}
		return fmt.Errorf("stat watch root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch root is not a directory: %s", absRoot)
	}

	w.root = absRoot
	return w.watchSubtreeLocked(absRoot)
}

// Root returns the current watched directory.
func (w *Watcher) Root() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.root
}

// Pause temporarily suspends event notifications.
func (w *Watcher) Pause() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paused = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}

// Resume resumes event notifications after the quiet delay to discard trailing OS events.
func (w *Watcher) Resume() {
	w.ResumeAfter(defaultResumeQuietDelay)
}

// ResumeAfter resumes event notifications after the specified quiet delay.
func (w *Watcher) ResumeAfter(quietDelay time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paused = false
	w.ignoredUntil = time.Now().Add(quietDelay)
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}

// Suppress executes fn with watcher notifications paused and discards trailing mutation events.
func (w *Watcher) Suppress(fn func()) {
	w.Pause()
	defer w.Resume()
	fn()
}

// Close closes the underlying filesystem watcher.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.mu.Unlock()

	return w.fsWatcher.Close()
}

func (w *Watcher) clearWatchesLocked() {
	for dir := range w.trackedDirs {
		_ = w.fsWatcher.Remove(dir)
	}
	w.trackedDirs = make(map[string]struct{})
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}

func (w *Watcher) watchSubtreeLocked(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if shouldIgnoreDirName(d.Name()) && path != root {
			return filepath.SkipDir
		}
		if err := w.fsWatcher.Add(path); err == nil {
			w.trackedDirs[path] = struct{}{}
		}
		return nil
	})
}

func (w *Watcher) readLoop() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed || w.paused || w.root == "" {
		return
	}
	if time.Now().Before(w.ignoredUntil) {
		return
	}

	path := event.Name
	base := filepath.Base(path)
	if shouldIgnoreName(base) {
		return
	}

	// Dynamic directory handling
	if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			_ = w.watchSubtreeLocked(path)
			w.scheduleDebounceLocked()
			return
		}
	}

	if event.Op&(fsnotify.Remove) != 0 {
		if _, tracked := w.trackedDirs[path]; tracked {
			delete(w.trackedDirs, path)
			_ = w.fsWatcher.Remove(path)
			w.scheduleDebounceLocked()
			return
		}
	}

	// Check if this is a recognized mod file or an operation on a tracked directory
	if _, tracked := w.trackedDirs[path]; tracked {
		w.scheduleDebounceLocked()
		return
	}

	if isRecognizedModFile(base) {
		w.scheduleDebounceLocked()
	}
}

func (w *Watcher) scheduleDebounceLocked() {
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		if w.closed || w.paused || time.Now().Before(w.ignoredUntil) {
			w.mu.Unlock()
			return
		}
		fn := w.onChange
		w.mu.Unlock()

		if fn != nil {
			fn()
		}
	})
}

func shouldIgnoreDirName(name string) bool {
	return strings.HasPrefix(name, ".") || strings.EqualFold(name, "$RECYCLE.BIN")
}

func shouldIgnoreName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	return lower == "metadata.json" || lower == "nexus.key" || strings.HasSuffix(lower, ".tmp")
}

func isRecognizedModFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range recognizedExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
