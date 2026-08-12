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
- Use temporary directories and disposable fixtures by default.
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

### Windows screenshots

- Capture the application window, not the full desktop.
- Wait for a nonzero `MainWindowHandle`, then use Win32 `PrintWindow` through PowerShell/.NET. Avoid `CopyFromScreen`, which can clip windows under display scaling.
- Save screenshots as `docs/screenshots/<phase>/task-<number>-<state>.png`.
- Inspect the saved PNG and close only the process launched for verification.

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
