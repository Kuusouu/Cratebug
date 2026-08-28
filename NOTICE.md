# NOTICE

## Origin

Cratebug is a ground-up reimplementation of [Repak-X](https://github.com/XzantGaming/Repak-X) by XzantGaming. Repak-X had the initial idea: a mod manager for Marvel Rivals that treats mod files carefully and gives players real control over their library. All credit for that idea goes to Repak-X and its authors.

Cratebug is not a fork. It started from a clean codebase and contains no Repak-X source code. Where the two projects meet is in intent: Repak-X set the direction, and Cratebug follows the same goal with its own design.

## Why a separate project

Repak-X is built with Tauri and React. Cratebug is built with Wails (Go) and React. That difference is deliberate: as Cratebug's direction and implementation diverged from Repak-X (a filesystem-safety-first design, staged installs, a Go backend owning every mutation, etc.), it became its own project rather than a contribution or a fork. The result is what Cratebug is today: a fresh take on the same idea, built to be a better, safer implementation of it.

## License and credit

Cratebug is released under the GNU General Public License version 3, the same license Repak-X is released under. See [LICENSE](LICENSE), whose attribution block names both projects:

- Cratebug - Copyright (C) 2026 mewclouds (Kuusouu)
- Based on Repak-X - Copyright (C) 2026 XzantGaming

If you enjoy Cratebug, then credit where it is due: none of this exists without the original.
