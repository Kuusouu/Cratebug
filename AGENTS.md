# Cratebug Agent Instructions

Cratebug is an open-source Marvel Rivals mod manager built with Go, Wails, React, and TypeScript.

## How to work here

- The user's request sets the scope. You do not need a roadmap phase, a task entry, or a decision record before starting work, and you should not ask for one.
- The project docs record intent and history, and they can fall behind the code. If a request conflicts with them, say so once and do what the user asked. If the change makes a doc wrong, fix the doc in the same change.
- Ask when a choice would materially change the result and the code cannot settle it. Otherwise pick a sensible option and say which one you picked.
- Keep changes focused on the request. Skip drive-by refactors, formatting sweeps, and dependency upgrades unless asked.
- Say when a premise looks wrong before building around it.
- Never fabricate repository state, behavior, command output, screenshots, or test results.

## Where things are

- `README.md` and `CONTRIBUTING.md`: setup and development commands.
- `CODING_GUIDELINES.md`: code, test, frontend, and CSS conventions. Follow these.
- `SPEC.md`: what Cratebug does and the filesystem safety rules it keeps.
- `docs/decisions/`: the reasoning behind non-obvious choices. Read the relevant record before reversing one of those choices.
- `ROADMAP.md`: a log of what has shipped and ideas for later. It does not gate work.
- `CHANGELOG.md`: user-facing release notes.

## Code

- Follow `CODING_GUIDELINES.md`. `./check.ps1` and `./check.sh` enforce most of it.
- Keep Go domain and filesystem behavior independent of Wails and React.
- Keep filesystem operations out of React components.
- Use Bun for frontend dependencies and scripts, and oxfmt and oxlint for formatting and linting.
- Add a dependency only when the change needs one. Do not upgrade pinned dependencies as a side effect.
- Remove unused code your change creates, but leave unrelated existing code alone.

## Safety

- Never test mutations against the user's real Marvel Rivals mod directory without explicit permission.
- Use temporary directories and disposable fixtures by default. Automated Go tests use `t.TempDir()`. For manually driving the running app, use `C:\ModsFixtures` (see below).
- Never weaken a filesystem safety check to make a test pass.
- Do not modify the BentoMod repository or its persisted state.
- Treat BentoMod as a behavioral and visual reference, not an architectural template.

## Decision records

Decision records are optional. Write one when a choice would be easy to undo by mistake: a pinned external tool, a security or filesystem boundary, or an alternative rejected for a reason that is not obvious from the code. Routine work does not need one. If you are unsure, offer to write one, and never hold up work waiting for one.

Name a record `docs/decisions/NNNN-short-name.md` with the next number. Give it a status, a date, the decision, and the reasoning, and keep it short. A record that replaces an earlier one names the record it replaces.

## Verification

- Run the smallest meaningful checks while you work. Before reporting a code change as done, run `./check.ps1` on Windows or `./check.sh` on Linux.
- Fix failing checks rather than skipping or weakening them.
- Do not claim a command passed unless you ran it and it passed.
- Compilation alone does not prove behavior or UI is correct.

For user-visible changes, launch the app, go to the affected state, take a screenshot, and fix any layout, clipping, spacing, typography, color, or state problems you see. If you cannot verify visually, say so.

### Driving and screenshotting the running app

Run `wails dev`. Wails serves a fully-bound dev URL alongside the
native window — the startup log prints it (normally `http://localhost:34115`).
Drive that URL with playwright-cli:

```powershell
npm i -g @playwright/cli   # only if playwright-cli is not installed yet (requires Node 24+)
playwright-cli install-browser chromium   # only if no browser is installed yet
playwright-cli open http://localhost:34115
playwright-cli snapshot   # accessibility tree with element refs
playwright-cli click <ref>
playwright-cli screenshot
```

The CLI is installed globally through npm. Every command prints the
current page state; take the `<ref>` for the next action from the latest
snapshot. End sessions with `close-all`.

The CLI's browser is a separate process from the native WebView2 window, not a
view into it. Calls that reach the shared Go backend (any exported Wails
binding) affect real application state; in-memory frontend state does not
live-sync, and DOM/CSS behavior can differ by window size and DPI. Say when a
check ran in the browser only, and ask the person running the native window to
verify anything size- or DPI-sensitive directly.

- Save verification screenshots under `.playwright-cli/screenshots/`, named for what they show.
  This directory is ignored by Git. Do not commit these screenshots.
  Track only the application screenshots used by `README.md`: `main.png` and the three Linux launch views.
- Fixtures: `C:\ModsFixtures` is the standing library for manually driving the app —
  a small, varied set of classic and IoStore bundles (enabled, disabled via each
  recognized suffix, and one with a missing sidecar) across a few folders. Point the
  app's mod root there instead of creating throwaway fixtures; add to it if
  a change needs a shape it doesn't already cover, but don't delete what's there
  afterward. It is disposable, synthetic data, not a real Marvel Rivals installation,
  so the real-mod-directory restriction above doesn't apply to it. If a change
  genuinely needs a real or complex library beyond what synthetic fixtures can
  represent, ask the user to point at one rather than fabricating something that
  pretends to be real data.
- BentoMod can be launched the same way as a live comparison reference when that
  helps; see `archive/BentoMod/AGENTS.md`. A second playwright-cli session
  (`-s=<name>`) can hold it side by side with the Cratebug tab.
- Close the dev process (and its Vite/bindings child processes) once verification
  is done.

## Git

- Review `git status` and the relevant diff before reporting completion.
- Do not commit, push, publish, tag, alter remotes, or discard user changes unless explicitly instructed.
- Keep generated files, secrets, caches, logs, local paths, and build output out of version control.

## When you finish

Tell the user what changed, how you verified it, and anything you could not verify. Mention problems you noticed outside the request instead of fixing them unasked. When there is something to commit, suggest a Conventional Commits message.
