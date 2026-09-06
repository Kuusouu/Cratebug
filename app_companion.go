package main

import (
	"fmt"
	"path/filepath"

	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

type companionPakCaller interface {
	Call(action string, params map[string]any, result any) error
}

func markStagedCompanionPaks(session *install.StagedSession, caller companionPakCaller) {
	root, err := filepath.Abs(session.Dir)
	if err != nil {
		return
	}
	for i := range session.Mods {
		mod := &session.Mods[i]
		pakAbs := filepath.Join(root, filepath.FromSlash(mod.RelativePrimaryPath))
		utocAbs := ""
		if mod.Sidecars.UTOC != "" {
			utocAbs = filepath.Join(root, filepath.FromSlash(mod.Sidecars.UTOC))
		}
		needed, err := mutation.CompanionPakNeedsCleanup(pakAbs, utocAbs, caller)
		if err != nil {
			continue
		}
		mod.UnsupportedCompanionPak = needed
	}
}

func stripStagedCompanionPaks(session *install.StagedSession, items []install.ApplyItem, caller companionPakCaller) error {
	selected := make(map[string]struct{}, len(items))
	for _, item := range items {
		selected[item.ID] = struct{}{}
	}
	root, err := filepath.Abs(session.Dir)
	if err != nil {
		return fmt.Errorf("resolve staging directory: %w", err)
	}
	for _, mod := range session.Mods {
		if _, ok := selected[mod.ID]; !ok {
			continue
		}
		if mod.RelativePrimaryPath == "" {
			continue
		}
		pakAbs := filepath.Join(root, filepath.FromSlash(mod.RelativePrimaryPath))
		utocAbs := ""
		if mod.Sidecars.UTOC != "" {
			utocAbs = filepath.Join(root, filepath.FromSlash(mod.Sidecars.UTOC))
		}
		needed, err := mutation.CompanionPakNeedsCleanup(pakAbs, utocAbs, caller)
		if err != nil {
			// A file that is not a readable Unreal PAK is not the
			// chunknames problem. Leave it for Apply to copy as-is.
			continue
		}
		if !needed {
			continue
		}
		if err := mutation.RewriteCompanionPak(root, pakAbs, utocAbs, caller); err != nil {
			return fmt.Errorf("rewrite staged companion PAK %q: %w", mod.DisplayName, err)
		}
	}
	return nil
}
