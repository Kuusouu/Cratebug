# Phase 2 Review

**Date:** 2026-08-03  
**Status:** Ready for review

## Outcome

Phase 2 adds a read-only Wails and React library screen on top of the Phase 1
scanner. Users can scan a folder, browse its discovered entries by physical
folder, search locally, and switch among compact, large, and list views. No
Phase 2 UI action writes to the selected folder.

## Binding and scanning behavior

- `App.ScanLibrary` exposes the existing Go discovery scanner to the frontend.
- Scans return loading, populated, empty, and actionable error states without
  removing the surrounding application shell.
- Empty scan results now serialize as an empty entry list rather than `null`,
  preventing the frontend crash previously found while scanning an empty
  folder.
- Folder selection, search, and view changes operate on the current scan
  result; they do not call the Wails binding again.
- A prebuilt subtree index makes small-folder navigation responsive even when
  the scanned library contains thousands of entries.

## Manual interaction coverage

The running development application was checked with synthetic discovery
fixtures and a real library expanded to approximately 2,400 entries for stress
testing. The following behavior was observed:

- Scan and Refresh show the loading state and refresh the displayed result.
- Search filters immediately within the currently selected folder.
- All Mods, nested folder selection, expanded/collapsed hierarchy, descendant
  counts, and compact/large/list views display the expected entries.
- Empty folder scans show the empty state. An access-denied scan of `C:\\`
  reports the inaccessible path in the error state while preserving the shell.
- Large All Mods views remain inherently more expensive because every card is
  rendered; changing between small folders and their views is responsive after
  folder-indexing and render fan-out fixes.
- System, light, and dark appearance modes were manually checked.
- Tab, Shift+Tab, Enter, Space, select keyboard controls, search input, focus
  outlines, and focus-triggered folder tooltips were manually checked.
- The 1000 by 650 minimum window and a maximized wide desktop window were
  checked for clipping, overflow, and useful use of available space.

## Visual review

The populated-library layout was compared with BentoMod as a structural
reference. Cratebug uses a folder hierarchy, counts, icon-only view controls,
and a scrollable catalog while retaining its own smaller, read-only component
structure. During the comparison, issues in list row sizing, scrollbar
contrast, long card text, wide-window width use, folder navigation feedback,
and tooltip presentation were corrected.

## Validation

The following command passed after the final Phase 2 interaction and layout
changes:

```powershell
.\check.ps1
```

It ran Go formatting, frontend formatting and linting, TypeScript checking,
the Vite production build, Go vet, and all Go tests.

## Screenshots

- [Initial library state](../screenshots/phase-2/task-2-initial-library.png)
- [Populated library](../screenshots/phase-2/task-2-populated-library.png)
- [Compact cards](../screenshots/phase-2/task-2-compact-cards.png)
- [Large cards](../screenshots/phase-2/task-2-large-cards.png)
- [List view](../screenshots/phase-2/task-2-list.png)
- [Minimum window size](../screenshots/phase-2/task-2-window-size.png)
- [Final 2,400-entry library](../screenshots/phase-2/task-2-final-phase-2-library.png)

## Limitations and deferred findings

- All Mods renders every matching card. The current 2,400-entry stress library
  is usable, but virtualization is the future solution if much larger
  libraries make that view unacceptable.
- Folder labels remain single-line and ellipsized to preserve a scan-friendly
  tree. Their full value is available through an immediate styled tooltip and
  keyboard focus. A persisted user-resizable sidebar is deferred until there
  is evidence it is needed.
- Phase 2 is intentionally read-only. Enable/disable, installation, deletion,
  tags, and conflict workflows remain out of scope for later phases.
- Keyboard behavior was manually verified, but screen-reader output was not
  tested with a dedicated assistive-technology run.

## Review approval

**Decision:** Approved by user on 2026-08-11
**Notes:** Phase 3 may begin.
