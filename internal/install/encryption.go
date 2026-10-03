package install

import (
	"context"
	"fmt"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

func canEncryptStagedMod(mod StagedMod) bool {
	return mod.BundleFormat == discovery.BundleFormatIoStore &&
		mod.Sidecars.UTOC != "" && mod.Sidecars.UCAS != "" && len(mod.Issues) == 0
}

// Rebuilds selected staged bundles before the install can change the destination.
// A false choice preserves the source encryption state. It never decrypts a mod.
func EncryptStagedMods(ctx context.Context, session *StagedSession, items []ApplyItem, caller mutation.ArchiveCaller, progress func(Progress)) error {
	modsByID := make(map[string]StagedMod, len(session.Mods))
	for _, mod := range session.Mods {
		modsByID[mod.ID] = mod
	}

	var targets []StagedMod
	for _, item := range items {
		mod, ok := modsByID[item.ID]
		if !ok {
			return fmt.Errorf("staged mod %q not found in session", item.ID)
		}
		if !item.Encrypt {
			continue
		}
		if !canEncryptStagedMod(mod) {
			return fmt.Errorf("encrypt staged mod %q: %w", mod.DisplayName, mutation.ErrEncryptionIneligible)
		}
		targets = append(targets, mod)
	}
	if len(targets) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	library, err := discovery.Scan(session.Dir)
	if err != nil {
		return fmt.Errorf("scan staged mods before encryption: %w", err)
	}
	entryIDsByPath := make(map[string]string, len(library.Entries))
	for _, entry := range library.Entries {
		entryIDsByPath[entry.PrimaryPath] = entry.ID
	}

	for i, mod := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		if progress != nil {
			progress(Progress{
				Phase: "encrypting", Current: i + 1, Total: len(targets),
				Message: fmt.Sprintf("Encrypt %d of %d: %s", i+1, len(targets), mod.DisplayName),
			})
		}
		// One target avoids the library batch rule that rejects mixed encryption states.
		result, err := mutation.SetModEncryption(session.Dir, []string{entryIDsByPath[mod.RelativePrimaryPath]}, true, caller, nil, ctx.Done())
		if err != nil {
			return fmt.Errorf("encrypt staged mod %q: %w", mod.DisplayName, err)
		}
		if len(result.Failed) > 0 {
			return fmt.Errorf("encrypt staged mod %q: %s", mod.DisplayName, result.Failed[0].Message)
		}
	}
	return nil
}
