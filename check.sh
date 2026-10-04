#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"
FRONTEND_ROOT="${REPO_ROOT}/frontend"

if [ -d "${HOME}/.local/bin" ]; then
    export PATH="${HOME}/.local/bin:${PATH}"
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

echo "==> Go tests"
go test ./...

echo "All checks passed."
