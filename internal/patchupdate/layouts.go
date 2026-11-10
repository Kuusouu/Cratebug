package patchupdate

import (
	"sort"
	"strings"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/usmap"
)

// Layout is one game build's property layout and the mappings file that
// describes it.
type Layout struct {
	Build    int
	Path     string
	Mappings *usmap.Mappings
}

// Layouts holds the current game's layout and the earlier ones a mod may have
// been cooked against.
type Layouts struct {
	Current  Layout
	Previous []Layout

	mu             sync.Mutex
	candidateCache map[string][]Layout
}

// NewLayouts sorts previous newest first. Layouts at or after the current
// build are dropped.
func NewLayouts(current Layout, previous []Layout) *Layouts {
	kept := make([]Layout, 0, len(previous))
	for _, layout := range previous {
		if layout.Build < current.Build {
			kept = append(kept, layout)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Build > kept[j].Build })
	return &Layouts{Current: current, Previous: kept, candidateCache: map[string][]Layout{}}
}

// Earlier layouts, newest first, that read classes differently from the
// current one, keeping one layout for each distinct way they read them. An
// empty result means every known layout reads those classes the same.
func (l *Layouts) olderCandidates(classes []string) []Layout {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := strings.Join(sortedUnique(classes), "\x00")
	if cached, ok := l.candidateCache[key]; ok {
		return cached
	}

	current := l.Current.Mappings
	currentNames := map[string]bool{}
	for _, class := range classes {
		for name := range current.Closure(class) {
			currentNames[name] = true
		}
	}

	var candidates []Layout
	seen := map[string]bool{}
	for _, layout := range l.Previous {
		names := make(map[string]bool, len(currentNames))
		for name := range currentNames {
			names[name] = true
		}
		for _, class := range classes {
			for name := range layout.Mappings.Closure(class) {
				names[name] = true
			}
		}
		fingerprint := layout.Mappings.Fingerprint(names)
		if fingerprint == current.Fingerprint(names) || seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		candidates = append(candidates, layout)
	}
	l.candidateCache[key] = candidates
	return candidates
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
