# Cratebug

Cratebug is an open-source, Windows-first mod manager for Marvel Rivals.

The project is a fresh implementation built with Go, Wails, React, and TypeScript. It aims to preserve useful workflows and compatibility with existing mod libraries while making filesystem operations safe, predictable, and recoverable.

## Project status

Cratebug is in Phase 0: repository and toolchain foundation. The toolchain baseline is established, but the application has not been scaffolded yet, so development and build commands are not available. Canonical validation, build, installation, and uninstall instructions will be added during the remaining Phase 0 tasks.

See:

- [Product specification](SPEC.md)
- [Roadmap](ROADMAP.md)
- [Active tasks](TASKS.md)
- [Contributor and agent guidance](AGENTS.md)
- [Toolchain baseline](docs/decisions/0001-toolchain-baseline.md)

If a hosted GitHub repository is created, it belongs under the `Kuusouu` organization. CI is intended to use the organization's existing Blacksmith integration where applicable.

## Toolchain setup

Install the pinned Go and Bun versions through `mise`:

```powershell
mise install
```

Install the pinned Wails CLI:

```powershell
mise exec -- go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

Verify the installed versions:

```powershell
mise exec -- go version
mise exec -- bun --version
mise exec -- wails version
```

See the [toolchain decision](docs/decisions/0001-toolchain-baseline.md) for exact versions and the upgrade policy.

## License

Cratebug is licensed under the GNU General Public License version 3. See [LICENSE](LICENSE).
