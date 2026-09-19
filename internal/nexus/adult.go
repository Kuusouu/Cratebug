package nexus

import (
	"context"
	"errors"
)

// Reports whether this mod may be shown or downloaded for the signed-in
// account. Non-adult mods always pass, even when prefsErr is set. Adult
// mods pass only when prefs.Adult is a confirmed true, Content Blocking
// is off, and prefsErr is nil.
func AllowAdultContent(mod ModInfo, prefs Preferences, prefsErr error) error {
	if !mod.ContainsAdultContent {
		return nil
	}
	if prefsErr != nil || !prefs.Adult || prefs.IsBlockingContent {
		return ErrAdultContentBlocked
	}
	return nil
}

// Refuses adult mods the way the Nexus website does. v1 REST and signed
// nxm:// downloads still return those files when the site hides them, so
// GraphQL is checked even if contains_adult_content is missing or false.
// A GraphQL failure is treated as adult: REST omitted the flag on mods
// the website still blocks.
func (c *Client) CheckAdultContent(ctx context.Context, mod ModInfo) error {
	found, gqlAdult, err := c.graphQLModAdult(ctx, mod.ModID)
	if err != nil {
		if errors.Is(err, ErrAdultContentBlocked) {
			return err
		}
		mod.ContainsAdultContent = true
		prefs, prefsErr := c.Preferences(ctx)
		return AllowAdultContent(mod, prefs, prefsErr)
	}
	if !found {
		return ErrAdultContentBlocked
	}

	if gqlAdult {
		mod.ContainsAdultContent = true
	}
	if !mod.ContainsAdultContent {
		return nil
	}
	prefs, prefsErr := c.Preferences(ctx)
	return AllowAdultContent(mod, prefs, prefsErr)
}
