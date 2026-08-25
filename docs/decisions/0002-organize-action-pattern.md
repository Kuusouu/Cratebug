# 0002: Organize action pattern

- Status: Accepted
- Date: 2026-08-21

## Context

Task 4.5.4 added move controls for mods and folders alongside the rename and
create controls from earlier in Phase 4. The first pass put every action in
persistent UI: header buttons for folder rename and move, and a growing row
of buttons on the selected-mod panel for rename, priority, and move.

Comparing this against BentoMod surfaced the actual tradeoff. BentoMod puts
folder and mod actions behind a right-click context menu, so you can act on
any folder or mod without first navigating into it or selecting it. Persistent
buttons only work for whatever is currently selected, and they take up screen
space whether or not you need them right now. The concern going in was
whether adding a context menu on top of the existing buttons would just make
the UI busier instead of cleaner.

## Decision

Right-click (or the keyboard equivalent, Shift+F10 or the Menu key on a
focused row) is the primary way to reach mod and folder organize actions:
rename, set priority, and move, for both mods and folders. This replaced the
header buttons and most of the selected-mod panel buttons rather than sitting
alongside them.

What stays as a persistent, always-visible control:

- Enable and disable, on the selected-mod panel. It is the single most
  frequent action and deserves a one-click control instead of a menu trip.
- New folder, as one button in the sidebar heading. Unlike rename or move, it
  has no natural single target when nothing is selected, so a context menu
  entry point alone would not cover creating a folder at the library root.
  The same action is also reachable by right-clicking any folder row, which
  creates the new folder inside it.

The context menu is built as a small reusable component
(`frontend/src/library/ContextMenu.tsx`) rather than one-off menus per
surface, so later phases can extend it (recoverable deletion in 4.5.5, tags
in Phase 5, conflict inspection in Phase 9) without inventing a second
pattern.

Keyboard access did not need custom key handling. Attaching the
`onContextMenu` handler to the actual interactive row button, not a wrapping
`div`, is enough: browsers already dispatch a `contextmenu` event at the
focused element when the user presses Shift+F10 or the Menu key. This was
verified directly in the running app, not just assumed.

## Consequences

- Future organize actions should extend the existing context menu rather than
  adding more header or panel buttons, unless the action has no single
  target the way New Folder does, or is frequent enough to earn a dedicated
  control the way Enable and Disable are.
- The selected-mod panel is now a status readout plus Enable and Disable, not
  a growing action bar. Anything added there should meet the same bar those
  two actions meet.
- A future action added to the menu without a keyboard-reachable trigger
  element would silently lose keyboard access. The context menu itself does
  not enforce this, so the row or card triggering it must stay a real
  interactive element.
