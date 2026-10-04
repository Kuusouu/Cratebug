# 0001: Toolchain baseline

- Status: Accepted
- Date: 2026-07-30

## Context

Cratebug needs a stable, reproducible Windows toolchain before the application is scaffolded. Versions must be explicit so local development, CI, and production builds do not drift independently.

The baseline uses current stable, non-preview releases available on 2026-07-30. Bun `1.3.14` is pinned via `frontend/package.json` (`packageManager`).

## Decision

Pin the following runtime and application-tool versions:

| Tool | Version |
| --- | --- |
| Go | `1.26.5` |
| Wails | `v2.13.0` |
| Bun | `1.3.14` |
| Node | `24.20.0` |
| React | `19.2.8` |
| React DOM | `19.2.8` |
| TypeScript | `7.0.2` |
| Vite | `8.1.5` |
| `@vitejs/plugin-react` | `6.0.4` |
| Biome | `2.5.6` |
| `@types/react` | `19.2.17` |
| `@types/react-dom` | `19.2.3` |

Go is pinned by `go.mod`, Bun by `frontend/package.json` (`packageManager`), and the Wails CLI by the `v2.13.0` install/build commands. Node was added on 2026-08-30 because the agentic-development workflow (see `AGENTS.md`) installs the Playwright CLI globally through npm (Node 24+); Cratebug's own frontend tooling remains Bun-only. ImageMagick is not part of the build toolchain: only `fetch-hero-portraits.ps1` needs it, installed on Windows via `winget install -e --id ImageMagick.ImageMagick --version 7.1.2.31`.

Task 0.3 must pin Wails in `go.mod`, use exact versions without range prefixes in `package.json`, and commit the resulting `bun.lock`. If the Wails CLI is represented as a Go tool dependency, it must use `v2.13.0`. Task 0.5 must install the same Go and Bun versions in CI.

TypeScript 7 is the stable native compiler line. It does not expose the legacy TypeScript programmatic API in 7.0. Cratebug's initial React and Vite setup uses the compiler command rather than that API. If a required tool proves incompatible during scaffolding, stop and revise this decision instead of silently adding a second compiler or downgrading.

## Installation

Install Go `1.26.5` and Bun `1.3.14` directly (see `CONTRIBUTING.md`), then install the pinned Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

Task 0.3 will add the frontend installation command after `package.json` exists. It will use Bun and the committed lockfile.

## Version checks

Run from the repository root:

```powershell
go version
bun --version
node --version
wails version
```

Expected version values are:

```text
go1.26.5
1.3.14
v24.20.0
v2.13.0
```

After task 0.3, verify that `go.mod`, `package.json`, and `bun.lock` agree with this record.

## Upgrade policy

- Use stable releases only. Do not pin alpha, beta, release-candidate, nightly, canary, or preview builds.
- Use exact versions for direct frontend dependencies; do not use `^`, `~`, `latest`, or floating major tags.
- Commit `bun.lock` and use frozen-lockfile installation in CI.
- Treat upgrades as deliberate maintenance work. Update this decision, runtime configuration, dependency declarations, lockfiles, and CI together.
- Run the full canonical checks and production build before accepting a toolchain upgrade.
- Do not upgrade dependencies incidentally while implementing unrelated work.

## Sources

- [Go release history](https://go.dev/doc/devel/release)
- [Wails v2.13.0 release](https://github.com/wailsapp/wails/releases/tag/v2.13.0)
- [Bun stable release](https://bun.com/)
- [React versions](https://react.dev/versions)
- [TypeScript 7.0 announcement](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/)
- [Vite 8.1 announcement](https://vite.dev/blog/announcing-vite8-1)
- [Biome 2.5 announcement](https://biomejs.dev/blog/biome-v2-5/)
- Exact frontend patch versions were checked against each package's `latest` metadata from the npm registry on 2026-07-30.

## Consequences

- Local runtime selection is reproducible through the pinned `go.mod`, `packageManager`, and Wails install commands.
- The later scaffold and CI have one authoritative version list.
- Patch upgrades require an explicit reviewed change.
- Package and lockfile agreement cannot be validated until task 0.3 creates those files.
