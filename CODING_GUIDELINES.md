# Cratebug Coding Guidelines

These guidelines define the shared readability and maintainability standards for Cratebug. They complement the project workflow and safety rules in `AGENTS.md`.

## Shared principles

- Keep changes small, direct, and easy to review.
- Prefer clear names and straightforward control flow over clever abstractions.
- When a file-level declaration is justified, group types, interfaces, constants, and configuration before functions.
- Avoid file-level globals. Keep single-use values local, and introduce a narrowly scoped shared definition only when multiple consumers need it.
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
- Use `t.TempDir` and disposable fixtures for filesystem tests. Never use a real Marvel Rivals mod directory in automated tests.
- Keep package-level declarations at the top of the file. Prefer small, focused unexported helpers over a large function with unrelated responsibilities.

## TypeScript and React

- Use lowercase primitive types: `string`, `number`, and `boolean`.
- Prefer `type` for object shapes, aliases, and unions. Use `interface` only for class shapes, declaration merging, or module augmentation.
- Do not use `any`. Use `unknown` when a value is genuinely unknown, then narrow it before use.
- Use `const` by default. Use `let` only when reassignment is necessary. Never use `var`.
- Prefer `async` and `await` to promise chains.
- Use optional chaining and nullish coalescing when they express the intended nullability behavior.
- Use `camelCase` for variables, functions, and non-component filenames. Use `PascalCase` for React components, their filenames, and types. Use `UPPER_SNAKE_CASE` for environment variables and regular-expression patterns.
- Keep React components focused on rendering and interaction. Filesystem policy and mutations belong in Go.
- Use Biome for formatting and linting; do not manually fight its output.

## CSS

- Keep one declaration per line and group related selectors together.
- Use custom properties for repeated visual values within a component or theme.
- Keep comments limited to non-obvious layout, browser, accessibility, or scaling constraints.
- Preserve responsive behavior when changing layout rules.
