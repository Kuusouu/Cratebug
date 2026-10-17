# Cratebug Coding Guidelines

These guidelines define the shared readability and maintainability standards for Cratebug. They complement the project workflow and safety rules in `AGENTS.md`.

## Shared principles

- Keep changes small, direct, and easy to review.
- Prefer clear names and straightforward control flow over clever abstractions.
- When a file-level declaration is justified, group types, interfaces, constants, and configuration before functions.
- Avoid file-level globals. Keep single-use values local, and introduce a narrowly scoped shared definition only when multiple consumers need it.
- Do not use magic numbers. Put unexplained literals in named constants or variables, with a comment explaining what the value means. Trivial literals such as `0`, `1`, or an empty string may stay inline.
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
- Use oxfmt for formatting and oxlint for linting; do not manually fight their output.
- One exported React component per file. Unexported helpers may stay in that file.
- Prefer Tailwind utilities over CSS modules for new components. Where a module already exists, keep it colocated as `Component.module.css` next to `Component.tsx`; skip the module file when utilities cover the component.

## Tailwind and shadcn/ui

- Design tokens live in `App.css` on `:root`, so portaled UI (dialogs, menus, tooltips) inherits the active theme and accent preset.
- `tailwind.css` maps those tokens to Tailwind names. Use the mapped utilities (`bg-background`, `bg-popover`, `text-foreground`, `text-muted-foreground`, `border-border`, `bg-primary`, `text-primary-foreground`, `bg-destructive`, `rounded-md|lg|xl`) and never hardcode hex, oklch, or `color-mix` values in classes.
- Interactive primitives (dialog, dropdown menu, context menu, select, tooltip, popover) come from the vendored shadcn components in `frontend/src/components/ui/`, built on Base UI. Compose them at call sites; change shared defaults in the component file instead of stacking per-call overrides. Base UI composition uses the `render` prop, not Radix's `asChild`.
- oxfmt sorts Tailwind classes through the `sortTailwindcss` setting; write classes in any order and let the formatter normalize them.

## CSS

- `style.css` and `App.css` are global: reset, fonts, theme tokens, shared primitives.
- Component-specific rules go in that component's `*.module.css`. Import as `styles` and apply with `className={styles["local-name"]}`. Combine with Tailwind utilities on the same element when needed.
- Keep one declaration per line and group related selectors together.
- Use custom properties for repeated visual values within a component or theme.
- Keep comments limited to non-obvious layout, browser, accessibility, or scaling constraints.
- Preserve responsive behavior when changing layout rules.
