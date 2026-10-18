#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"
FRONTEND_ROOT="${REPO_ROOT}/frontend"

if [ -d "${HOME}/.local/bin" ]; then
    export PATH="${HOME}/.local/bin:${PATH}"
fi

EXPECTED_LINT_VERSION=$(cat "${REPO_ROOT}/.golangci-lint-version")
if ! command -v golangci-lint >/dev/null 2>&1; then
    echo "Install golangci-lint ${EXPECTED_LINT_VERSION} on PATH (see CONTRIBUTING.md)." >&2
    exit 1
fi
if [ "$(golangci-lint version --short)" != "${EXPECTED_LINT_VERSION}" ]; then
    echo "Expected golangci-lint ${EXPECTED_LINT_VERSION}. See CONTRIBUTING.md." >&2
    exit 1
fi

echo "==> Go formatting"
UNFORMATTED=$(find "${REPO_ROOT}" -name "*.go" \
    ! -path "${REPO_ROOT}/.git/*" \
    ! -path "${REPO_ROOT}/build/*" \
    ! -path "${FRONTEND_ROOT}/node_modules/*" \
    -exec gofmt -l {} +)

if [ -n "${UNFORMATTED}" ]; then
    echo "Go formatting check failed:" >&2
    echo "${UNFORMATTED}" >&2
    exit 1
fi

echo "==> Frontend checks"
cd "${FRONTEND_ROOT}"
bun run check

echo "==> Go vet"
cd "${REPO_ROOT}"
go vet ./...

echo "==> Go lint"
golangci-lint run ./...

echo "==> Go tests"
go test ./...

echo "All checks passed."
