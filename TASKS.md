# Cratebug Active Tasks

**Phase:** 0 - Repository and toolchain foundation  
**Status:** In progress

This file contains only the active phase. Replace it when Phase 0 is complete.

## Phase objective

Create a clean, reproducible repository and prove that Cratebug can develop, validate, build, install, launch, and uninstall on Windows.

No mod-management behavior belongs in this phase.

## Exit criteria

- Development app launches.
- Canonical Go and frontend checks pass.
- Production build succeeds.
- NSIS installer installs, launches, and uninstalls.
- CI passes from a clean environment.
- A clean checkout reproduces the documented workflow.
- Human review approves the foundation.

## 0.1 Initialize the repository

- Create a fresh Git repository.
- If a hosted GitHub repository is created, create it under the `Kuusouu` organization.
- Add `README.md`, `LICENSE`, `SPEC.md`, `ROADMAP.md`, `TASKS.md`, and `AGENTS.md`.
- Add an appropriate `.gitignore`.
- Confirm that no Pakkit, WinUI, BentoMod, local-cache, or build-output files were copied accidentally.

**Verify:** Review the repository tree and `git status`.

## 0.2 Pin the toolchain

- Select current stable, non-preview versions of Go, Wails v2, Bun, React, TypeScript, Vite 8, and Biome.
- Record the exact versions and upgrade policy in `docs/decisions/0001-toolchain-baseline.md`.
- Record installation and version-check commands.
- Use exact versions or lockfiles in project configuration and CI.

**Verify:** Version commands, package declarations, and the Bun lockfile match the decision record.

## 0.3 Scaffold the application

- Initialize the Go module and Wails v2 project.
- Use React and TypeScript with Vite 8.
- Configure Bun for frontend installation and scripts.
- Remove demo behavior and branding.
- Add only a minimal Cratebug shell and one small frontend-to-Go call.
- Do not add mod logic or migrate BentoMod UI yet.

**Verify:** Development mode launches, the Go call succeeds, and a screenshot shows the minimal Cratebug window without template branding.

## 0.4 Configure validation

- Configure Biome formatting and linting.
- Enable appropriate TypeScript strictness.
- Add clear Bun scripts for development, build, formatting, linting, type checking, and combined checks.
- Add canonical Go formatting, `go vet`, and `go test` commands.
- Add at least one small Go test.
- Provide one root-level command or script that runs all required checks.

**Verify:** All checks pass, and deliberate sample failures are detected before being reverted.

## 0.5 Add continuous integration

- Use clean Windows CI.
- Use the `Kuusouu` GitHub organization's existing Blacksmith CI integration where applicable.
- Install the pinned Go and Bun versions.
- Install frontend dependencies from the lockfile.
- Run all canonical frontend and Go checks.
- Build the frontend and Windows application where supported.
- Do not publish releases or artifacts automatically in Phase 0.

**Verify:** CI passes and correctly rejects a deliberate temporary failure.

## 0.6 Build and package

- Configure application metadata and temporary branding.
- Produce a Windows AMD64 production build.
- Confirm the app runs without a development server or unexpected console.
- Configure Wails NSIS packaging.
- Install, launch, and uninstall in a disposable Windows environment where practical.
- Do not finalize signing, upgrade migration, or release branding yet.

**Verify:** Production app and installer work; installed files are removed on uninstall; unrelated files remain untouched. Capture production and installed-app screenshots.

## 0.7 Document and review

- Document prerequisites, setup, development, checks, production build, and installer commands in `README.md`.
- Reproduce the workflow from a clean checkout.
- Create `docs/reviews/phase-0-review.md` with versions, commands, CI result, installer result, screenshot paths, limitations, and human approval.
- Confirm that no mod logic, persistence, UAssetToolRivals integration, or BentoMod UI migration entered Phase 0.

**Verify:** Human approval grants permission to begin Phase 1.

## Phase 0 completion report

For each task, report:

- What changed
- Files changed
- Validation and results
- Manual checks and screenshot paths
- Known limitations
- Deferred findings
- Suggested commit message

Do not begin the next task or phase automatically.
