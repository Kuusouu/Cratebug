# Cratebug Agent Instructions

## Scope

This repository contains Cratebug, an open-source Marvel Rivals mod manager built with Go, Wails, React, and TypeScript.

## Operating principles

- Working code only. Verify before reporting completion.
- Never fabricate repository state, behavior, files, command output, screenshots, or test results.
- Prefer repository evidence, official documentation, and small experiments over assumptions.
- Say when a premise appears incorrect before implementing around it.
- Touch only what the active task requires.
- Avoid drive-by refactors, dependency upgrades, formatting sweeps, and speculative abstractions.
- Keep changes small, readable, and reviewable.
- Compilation alone does not prove that behavior or UI is correct.

## Documentation routing

Read only what the task requires:

- `SPEC.md` for product behavior and boundaries.
- `ROADMAP.md` for phase order and review gates.
- `TASKS.md` for the active numbered task and required validation.
- `docs/decisions/` for accepted technical decisions.
- `README.md` for canonical development commands.
- `CODING_GUIDELINES.md` for language, test, frontend, and CSS conventions.

Before editing, identify the active task and confirm that the work belongs in the current roadmap phase. Do not implement later-phase work without approval.

## Before editing

- State the intended change and how it will be verified.
- Read the files being changed and nearby code or tests that define their behavior.
- Resolve uncertainty by inspecting code, fixtures, or official documentation when practical.
- Ask only when multiple reasonable choices would materially change the result and repository evidence cannot resolve them.

## Editing

- Use the minimum change that completely solves the task.
- Match established project structure, naming, and style.
- Follow `CODING_GUIDELINES.md` for code, test, frontend, and CSS conventions.
- Keep Go domain and filesystem behavior independent of Wails and React.
- Keep filesystem operations out of React components.
- Do not add dependencies unless the active task requires them.
- Do not upgrade pinned dependencies during unrelated work.
- Use Bun for frontend dependencies and scripts.
- Use Biome for frontend formatting and linting.
- Clean up unused code created by the current change, but do not remove unrelated existing code.

## Safety

- Never test mutations against the user's real Marvel Rivals mod directory without explicit permission.
- Use temporary directories and disposable fixtures by default. For manually driving the
  running app (not automated Go tests, which use `t.TempDir()`), `C:\ModsFixtures` is the
  standing fixture library — see "Driving and screenshotting the running app" below.
- Never weaken a filesystem safety check merely to make a test pass.
- Do not modify the BentoMod repository or its persisted state.
- Treat BentoMod as a behavioral and visual reference, not an architectural template.

## Verification

- Run the smallest meaningful checks during implementation.
- Run all validation required by the active task before reporting completion.
- Fix failing checks rather than skipping or weakening them.
- Do not claim a command passed unless it was run successfully.

For user-visible changes:

1. Launch the application when possible.
2. Navigate to the affected state.
3. Capture a screenshot.
4. Compare it with any available reference.
5. Fix visible layout, clipping, spacing, typography, color, or state problems.

If visual verification cannot be performed, say so clearly.

### Driving and screenshotting the running app

Run `mise exec -c "wails dev"`. Wails serves a fully-bound dev URL alongside the
native window — the startup log prints it (normally `http://localhost:34115`).
Navigating a plain browser tab there loads the real frontend with a working
`window.go.main.App` bridge to the Go backend, identical to the native window.
Use the `playwright` MCP tools (configured in this repo's `.mcp.json`) to navigate
there, click, type, read the DOM/console, and screenshot — no native-window capture
or WebView2 debug-port setup needed.

The `playwright` server launches its own bundled Chromium (`--browser chromium`)
rather than real Chrome, since installing Chrome requires admin rights this machine
doesn't have. That bundled build is a separate download from the `playwright` npm
package's own copy — on a fresh machine, first run
`mise exec -- bunx @playwright/mcp@latest install-browser chrome-for-testing` once,
or MCP calls fail with "Chromium distribution ... is not found".

**If the `playwright` MCP tools aren't available** (observed in background/headless
sessions, where MCP servers needing an interactive subprocess spawn don't connect —
check with a ToolSearch query like `"playwright browser navigate"` before assuming),
fall back to driving a real browser directly instead of giving up on live
verification:

* Write a standalone `.mjs` script in a scratch directory (not the repo) that
  imports `playwright-core` — `npm add`/`bun add playwright-core` there first. Don't
  add it to `frontend/package.json`; it's a throwaway verification tool, not a
  project dependency.
* Launch with `chromium.launch({ channel: "msedge", headless: false })`. Real Edge
  ships built into Windows already, so `channel: "msedge"` needs no install step
  (unlike Chrome). `headless: false` opens a visible window — prefer this over
  headless when a person is available to watch, since they can confirm the result
  directly and it costs nothing extra.
* Navigate to the `wails dev` URL exactly as the MCP flow would, then drive it with
  normal Playwright APIs (`page.goto`, `page.click`, `page.evaluate`, etc.).
* This is a genuinely separate browser process from the native WebView2 window, not
  a view into it. Two consequences: (1) DOM/CSS/scroll behavior can differ by real
  window size and DPI, not just engine — a check that passes here is not proof it
  holds in the native window at its actual size, so say so explicitly rather than
  reporting it as confirmed; ask the person running the native window to verify
  anything size/DPI-sensitive (scroll containment, layout overflow) directly. (2)
  Application state (e.g. theme, any in-memory React state) does not live-sync
  between this browser and the native window — only calls that hit the shared Go
  backend (any exported Wails binding) have an effect the native window could ever
  observe, and only after its own next refresh or restart, not live.
* CDP-attaching to the native window's own WebView2 instance (the same pattern
  `playwright-bentomod` uses for BentoMod) would avoid all of the above since it's
  the literal live window, not a proxy — but has had unresolved issues on this
  machine. Worth another attempt if the MCP route is unavailable and the above
  proxy-browser caveats matter for the task, but don't assume it'll work.

- Save screenshots as `docs/screenshots/<phase>/task-<number>-<state>.png`.
- Fixtures: `C:\ModsFixtures` is the standing library for manually driving the app —
  a small, varied set of classic and IoStore bundles (enabled, disabled via each
  recognized suffix, and one with a missing sidecar) across a few folders. Point the
  app's mod root there instead of creating throwaway fixtures per task; add to it if
  a task needs a shape it doesn't already cover, but don't delete what's there
  afterward. It is disposable, synthetic data, not a real Marvel Rivals installation,
  so the real-mod-directory restriction above doesn't apply to it. If a task
  genuinely needs a real or complex library beyond what synthetic fixtures can
  represent, ask the user to point at one rather than fabricating something that
  pretends to be real data.
- BentoMod can be launched the same way as a live comparison reference when a task
  calls for it; see `archive/BentoMod/AGENTS.md`. This repo's `.mcp.json` also
  defines a `playwright-bentomod` server that attaches over CDP to BentoMod's debug
  port (`http://localhost:9223`) instead of launching its own browser, so both
  `playwright` and `playwright-bentomod` tools are available in the same session for
  side-by-side comparison — no directory switching needed. BentoMod must already be
  running (`pnpm tauri dev` from `bentomod/`) for that port to have anything to
  attach to.
- Close the dev process (and its Vite/bindings child processes) once verification
  is done.

## Git

- Review `git status` and the relevant diff before reporting completion.
- Do not commit, push, publish, tag, alter remotes, or discard user changes unless explicitly instructed.
- Keep generated files, secrets, caches, logs, local paths, and build output out of version control.

## Completion report

Report:

- What changed
- Files changed
- Validation performed and results
- Manual checks and screenshot paths
- Known limitations or unverified behavior
- Deferred findings outside the task
- A concise suggested commit message

Stop at review gates. Do not begin the next task or phase automatically.
