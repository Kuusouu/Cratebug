# Cratebug Coding Guidelines

These guidelines define the shared readability and maintainability standards for Cratebug. They complement the project workflow and safety rules in `AGENTS.md`.

## Automated checks

Run `./check.ps1` on Windows or `./check.sh` on Linux before submitting changes.
Both CI jobs use these scripts. `bun run check` from `frontend` runs the frontend
portion, including tests. Install the pinned tools listed in `CONTRIBUTING.md`.

| Convention | Enforcement |
| --- | --- |
| Go formatting | `gofmt` |
| Frontend formatting, import order, Tailwind class order | `frontend/.oxfmtrc.json` |
| TypeScript types, imports, variables, suppressions | `frontend/.oxlintrc.json` and `frontend/tsconfig.json` |
| Hook rules and dependencies, JSX accessibility | Oxlint React and jsx-a11y plugins |
| Floating/misused promises, invalid awaits, exhaustive union switches | Oxlint with pinned `oxlint-tsgolint` |
| Import cycles, duplicate imports, primitive import boundaries | Oxlint import rules and overrides |
| Literal hex and CSS color functions in Tailwind arbitrary values | `frontend/lint/style-tokens.js`; integration-tested with Bun |
| Go correctness, unused code, assignments, unchecked errors, error identity | `go vet` and `.golangci.yml` |
| Wails imports in domain packages, explained Go suppressions | golangci-lint `depguard` and `nolintlint` |
| Behavior covered by tests | Bun tests and `go test ./...` |

Passing checks does not prove visual quality, accessibility, filesystem safety,
or complete test coverage. The remaining conventions below are review criteria
unless the table names an automated check. GitHub required merge checks are a
separate repository setting; they are not configured by these files.

## Shared principles

- Keep changes small, direct, and easy to review.
- Prefer clear names and straightforward control flow over clever abstractions.
- When a file-level declaration is justified, group types, interfaces, constants, and configuration before functions.
- Avoid file-level globals. Keep single-use values local, and introduce a narrowly scoped shared definition only when multiple consumers need it.
- Name domain limits, timeouts, sizes with units, and other literals whose meaning is unclear. Explain non-obvious constraints. Obvious values and local Tailwind layout metrics may stay inline.
- Use blank lines to separate distinct validation and control-flow decisions when that makes the code easier to scan.
- Comment why a decision, constraint, or workaround exists. Do not narrate the next line of code.
- Write documentation comments as a natural description of behavior or purpose; do not repeat the identifier they document.
- Keep public APIs narrow. Do not export helpers or types unless another package or module needs them.
- Tests use explicit `Arrange`, `Act`, and `Assert` markers. Combine stages only when an operation must be checked immediately.

## Go

- Follow idiomatic Go and format every Go file with `gofmt`.
- Use concrete types and simple functions before introducing interfaces.
- Keep domain and filesystem behavior independent of Wails and React.
- Wrap errors with useful operation context and preserve the original error using `%w` where callers need it.
- Use `errors.Is` and `errors.As` for error identity. Preserve the primary error; wrapping secondary rollback details is a deliberate API decision. A failed rollback must never look like a safe cancellation.
- Check write, flush, and close errors before reporting success. A deferred close for an input, or cleanup after an existing error, may explicitly discard its result with a reason. Do not use `_ =` merely to silence errcheck. Test cleanup failures should fail the test.
- Pass `context.Context` through cancellable work and release owned resources. Review these lifetimes; the configured linters do not prove them correct.
- Use `t.TempDir` and disposable fixtures for filesystem tests. Never use a real Marvel Rivals mod directory in automated tests.
- Keep package-level declarations at the top of the file. Prefer small, focused unexported helpers over a large function with unrelated responsibilities.

## TypeScript and React

- Use lowercase primitive types: `string`, `number`, and `boolean`.
- Prefer `type` for object shapes, aliases, and unions. Use `interface` only for class shapes, declaration merging, or module augmentation.
- Mark imports used only as types with `import type` or an inline `type` specifier.
- Do not use `any`. Use `unknown` when a value is genuinely unknown, then narrow it before use.
- Use `const` by default. Use `let` only when reassignment is necessary. Never use `var`.
- Prefer `async` and `await` to promise chains.
- Use optional chaining and nullish coalescing when they express the intended nullability behavior.
- Use `camelCase` for variables, functions, and non-component filenames. Use `PascalCase` for React components, their filenames, and types. Use `UPPER_SNAKE_CASE` for environment variables and regular-expression patterns.
- Keep React components focused on rendering and interaction. Filesystem policy and mutations belong in Go.
- Use oxfmt for formatting and oxlint for linting; do not manually fight their output.
- Let oxfmt order imports. Side-effect imports keep their order; CSS imports are allowed for their effects.
- Call hooks unconditionally and include their reactive dependencies. Stabilize derived hook inputs when necessary; do not omit dependencies to force an effect to run once.
- Await promises or handle rejection. Use `void` for intentionally detached work only when the called operation handles failures or the chain has a rejection handler. `void` alone does not handle an error.
- Give controls accessible names and equivalent keyboard actions. Preserve intentional focus behavior and verify composed controls in the running app.
- Oxlint enforces primitive types, type aliases, type-only imports, no explicit `any`, and `const`/`let` usage. For an allowed `interface` exception, suppress only `typescript/consistent-type-definitions` at the declaration and explain why it is needed.
- Do not use `@ts-ignore` or `@ts-nocheck`. If an error suppression is necessary, use `@ts-expect-error` with an explanation. Remove unused lint suppressions; `bun run lint` rejects them.
- TypeScript checks return paths, rejects unreachable code and unused labels, and preserves value imports with `verbatimModuleSyntax`.
- Prefer one exported application component per file. Shared shadcn primitive modules may export their related parts together; unexported helpers may stay with the component. This is a review convention.
- Prefer Tailwind utilities over CSS modules for new components. Where a module already exists, keep it colocated as `Component.module.css` next to `Component.tsx`; skip the module file when utilities cover the component.

## Tailwind and shadcn/ui

- Design tokens live in `App.css` on `:root`, so portaled UI (dialogs, menus, tooltips) inherits the active theme and accent preset.
- `tailwind.css` maps those tokens to Tailwind names. Use the mapped utilities (`bg-background`, `bg-popover`, `text-foreground`, `text-muted-foreground`, `border-border`, `bg-primary`, `text-primary-foreground`, `bg-destructive`, `rounded-md|lg|xl`) and never hardcode hex, oklch, or `color-mix` values in classes.
- Interactive primitives (dialog, dropdown menu, context menu, select, tooltip, popover) come from the vendored shadcn components in `frontend/src/components/ui/`, built on Base UI. Compose them at call sites; change shared defaults in the component file instead of stacking per-call overrides. Base UI composition uses the `render` prop, not Radix's `asChild`.
- oxfmt sorts Tailwind classes through the `sortTailwindcss` setting; write classes in any order and let the formatter normalize them.
- Vendor primitive imports belong in `components/ui`. A narrowly documented exception is allowed when a wrapper lacks a required API, such as the pointer-anchored context menu's virtual anchor and portal container.
- The color rule checks static string literals and template segments in `src`. It permits CSS variable references, spacing values, and accent-picker color data. Runtime-computed classes, named palette colors, inline styles, and CSS token choices still need review.

## CSS

- `style.css` and `App.css` are global: reset, fonts, theme tokens, shared primitives.
- Component-specific rules go in that component's `*.module.css`. Import as `styles` and apply with `className={styles["local-name"]}`. Combine with Tailwind utilities on the same element when needed.
- Keep one declaration per line and group related selectors together.
- Use custom properties for repeated visual values within a component or theme.
- Keep comments limited to non-obvious layout, browser, accessibility, or scaling constraints.
- Preserve responsive behavior when changing layout rules.

## Exceptions and rule changes

- Fix the cause before suppressing a diagnostic. Suppress the specific rule at the smallest useful scope and explain the constraint. Go `//nolint` comments must name the linter and include a reason; unused lint suppressions fail checks.
- Generated Wails bindings, dependencies, and build output are excluded from frontend formatting and linting. Change the source that generates them.
- React's automatic JSX runtime does not require a `React` binding. React Compiler effect/memo/ref optimization checks are deferred until their state-management implications receive a separate review. Hook order and dependency checks remain enabled.
- Type-aware linting targets promise handling and exhaustiveness. `consistent-return` is off because TypeScript checks return paths and supports intentional `undefined` results. The broad `no-misused-spread` and `no-unsafe-type-assertion` rules remain off: generated Wails DTO classes are copied as data, Unicode validation counts code points, and boundary assertions require contextual review. None of these exceptions permits explicit `any`.
- Valid ARIA roles are permitted even when an alternative HTML element exists. The label checker searches three levels to recognize nested Nexus file labels. Pointer hit areas and user-requested autofocus retain line-scoped explanations.
- Go Staticcheck enables its `SA*` correctness checks. Style and complexity quotas are not enabled. Errorlint requires a wrapped primary error but does not require every secondary error to be exposed through unwrapping.
- Add rules for a concrete recurring failure, check their findings, and test custom rules against both valid and invalid examples. Do not turn on every available rule or exempt an entire application file to make a gate pass.

## References

- [TypeScript strict compiler checks](https://www.typescriptlang.org/tsconfig/#strict)
- [React hook dependency guidance](https://react.dev/reference/eslint-plugin-react-hooks/lints/exhaustive-deps)
- [Oxlint type-aware linting](https://oxc.rs/docs/guide/usage/linter/type-aware)
- [Oxfmt sorting](https://oxc.rs/docs/guide/usage/formatter/sorting)
- [Go code review comments](https://go.dev/wiki/CodeReviewComments)
- [Go error handling and wrapping](https://go.dev/blog/go1.13-errors)
- [golangci-lint settings](https://golangci-lint.run/docs/linters/configuration/)
