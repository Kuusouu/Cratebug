# Cratebug Active Tasks

**Chore:** Frontend colocation
**Status:** Active
**Branch:** `chore/frontend-colocation`

This file contains only the active work. It is chore work, not a roadmap phase. Do not edit `ROADMAP.md`.

## Objective

Split the two frontend monoliths without changing behavior. `LibraryScreen.tsx` keeps screen state and wiring. Each exported UI component lives in its own file with a `*.module.css` when it has unique styles. `App.css` keeps only theme tokens and shared primitives.

## Design decisions

* **CSS Modules.** `Component.module.css`, not a second global `Component.css`.
* **Global CSS stays for shared chrome.** Reset, fonts, `--*` tokens on `.app-shell`, and shared primitives (`.quiet-button`, `.icon-button`, `.destructive-button`, `.mutation-dialog*`, `.eyebrow`, `.visually-hidden`).
* **`.app-shell` stays global** so theme tokens inherit everywhere.
* **Scrollbar primitive.** Rules that today list several containers become one global class (`.scroll-y`) applied next to the module class. Do not leave hashed module classes in the old grouped scrollbar selector.
* **One exported component per file.** Unexported helpers may stay in that file (`ConflictCharacterHeading` stays inside `ConflictDetailsDialog.tsx`).
* **No extra splits.** No `useLibraryScreen` hook. No header/toolbar extract. No Tailwind, no CSS-in-JS, no visual redesign.
* **Maintainer verifies the running app.** Agent does not use playwright-cli and does not take screenshots.

## Exit criteria

* `bun run check` from `frontend/` passes.
* `App.css` no longer holds component-private selectors.
* `LibraryScreen.tsx` no longer defines the dialog/panel components listed in C.2.
* Maintainer confirms the UI looks the same.

## Out of scope

* `ROADMAP.md`
* remaining-todos TODO 2: default window size and list-view scrollbar at 1080p / 100% scale
* Review markdown, screenshot docs
* Extracting screen state, header/toolbar shells, Tailwind, CSS-in-JS, visual redesign

## C.1 Docs

Update `CODING_GUIDELINES.md` only. Short additions, not a new section wall.

Under **TypeScript and React**, add two bullets:

* One exported React component per file. Unexported helpers may stay in that file.
* Colocate component styles as `Component.module.css` next to `Component.tsx`. Skip the module file when the component uses only shared primitives.

Under **CSS**, add two bullets above the existing formatting rules:

* `style.css` and `App.css` are global: reset, fonts, theme tokens, shared primitives.
* Component-specific rules go in that component's `*.module.css`. Import as `styles` and apply with `className={styles["local-name"]}`. Combine with a global primitive when needed (`className={`mutation-dialog ${styles.dialog}`}`).

Keep the existing four CSS formatting bullets.

**Verify:** The guidelines name CSS Modules and one exported component per file, and stay short.

## C.2 Extract TSX only

Move these already-propped functions out of `frontend/src/library/LibraryScreen.tsx` into sibling files. Do not change JSX structure or `className` strings yet.

* `MutationToast`
* `SelectedModPanel`
* `ModMutationDialog`
* `FolderMutationDialog`
* `DeleteConfirmDialog`
* `FolderDeleteConfirmDialog`
* `ModTagDialog`
* `ConflictDetailsDialog` (keep its inner heading/card/row unexported)

Move helpers with their owners: `renameValidationError` / `hasWindowsReservedCharacter` with the mutation dialogs; `maximumPriorityFor` / `basename` with whoever calls them; `groupByCharacter` with `ConflictDetailsDialog`.

Leave in `LibraryScreen.tsx`: all `useState` / Wails handlers, `indexLibrary`, `libraryStatusMessage`, `ViewModeButton` (small, unexported).

**Verify:** `bun run check` from `frontend/`.

## C.3 CSS Modules, one component at a time

For each library UI file, cut its private selectors out of `frontend/src/App.css` into `Name.module.css`, switch that file's unique `className="…"` to `styles["…"]`, leave primitive class names as plain strings.

Order (already-split files first, then the new extracts, then the shell):

1. `ContextMenu`, `FolderNavigation`, `TagMenu`, `ModCatalog`
2. `SettingsDialog`, `DetectLibraryDialog`, `UpdateDialog`, `InstallPreviewDialog` (`InstallFromUrlDialog` likely needs no module file)
3. The C.2 extracts
4. `LibraryScreen` shell (header, toolbar, layout, drop overlay)

Keep kebab-case local names so the CSS is a move, not a rename. No Vite `css.modules` config. `vite/client` already types `*.module.css`.

**Verify:** `bun run check` from `frontend/`.

## C.4 Trim `App.css`

What remains in `App.css`: `.app-shell` token blocks, layout width helpers that wrap header/toolbar/layout, primitive buttons/dialog chrome, `.scroll-y`, `.visually-hidden`, `.spinning-loader`. Delete moved rules. `App.tsx` still imports `./App.css`.

**Verify:** `bun run check` from `frontend/`. Stop. Maintainer drives the app.

## Follow-up (not this chore)

* remaining-todos TODO 2: default window size and list-view scrollbar at 1080p / 100% scale
