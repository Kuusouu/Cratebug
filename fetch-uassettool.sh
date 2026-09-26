#!/usr/bin/env bash
set -euo pipefail

# Fetches and verifies the pinned UAssetToolRivals worker release and Oodle library for Linux.
# Pinned version and revision are kept in sync with docs/decisions/0004-pin-uassettool-worker.md.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_REPO="mewclouds/UAssetToolRivals"
RELEASE_TAG="v1.5.9"
ASSET_NAME="UAssetTool-linux-x64.tar.gz"
EXPECTED_SHA256="5e70809452baf667c1b0b9604896bc4de439144684b7204bab0a1386b5910d52"
EXPECTED_SOURCE_REVISION="7c185ae5da2ac446cf58db75ebd34f9402b8b5dc"

OODLE_ASSET_NAME="liboo2corelinux64.so.9"
OODLE_DOWNLOAD_URL="https://github.com/new-world-tools/go-oodle/releases/download/v0.2.3-files/${OODLE_ASSET_NAME}"
OODLE_EXPECTED_SHA256="7354655eb25b587dc34cbf98696b91e30e6d7a3f0eefad3872e6c1b76ef86a6e"

DOWNLOAD_URL="https://github.com/${RELEASE_REPO}/releases/download/${RELEASE_TAG}/${ASSET_NAME}"
TARGET_DIR="${SCRIPT_DIR}/build/uassettool-linux"
TARGET_EXE="${TARGET_DIR}/UAssetTool"
ARCHIVE_PATH="${TARGET_DIR}/${ASSET_NAME}"
OODLE_TARGET_PATH="${TARGET_DIR}/${OODLE_ASSET_NAME}"

verify_checksum() {
    local file="$1"
    local expected="$2"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    local actual
    actual="$(sha256sum "$file" | awk '{print $1}')"
    [[ "$actual" == "$expected" ]]
}

mkdir -p "$TARGET_DIR"

if [[ -f "$TARGET_EXE" ]] && verify_checksum "$ARCHIVE_PATH" "$EXPECTED_SHA256"; then
    echo "==> Pinned worker already present and verified: $TARGET_EXE"
else
    echo "==> Downloading $ASSET_NAME from ${RELEASE_REPO}@${RELEASE_TAG}"
    curl -sSL "$DOWNLOAD_URL" -o "$ARCHIVE_PATH"

    echo "==> Verifying SHA-256 against the pinned checksum"
    if ! verify_checksum "$ARCHIVE_PATH" "$EXPECTED_SHA256"; then
        actual="$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')"
        rm -f "$ARCHIVE_PATH"
        echo "Checksum mismatch for ${ASSET_NAME}: expected ${EXPECTED_SHA256}, got ${actual}. File deleted." >&2
        exit 1
    fi

    echo "==> Extracting to $TARGET_DIR"
    tar -xzf "$ARCHIVE_PATH" -C "$TARGET_DIR"
    chmod 0755 "$TARGET_EXE"
fi

# Fetch and verify Linux Oodle native compression library
if [[ -f "$OODLE_TARGET_PATH" ]] && verify_checksum "$OODLE_TARGET_PATH" "$OODLE_EXPECTED_SHA256"; then
    echo "==> Native Oodle library already present and verified: $OODLE_TARGET_PATH"
else
    echo "==> Downloading ${OODLE_ASSET_NAME}"
    curl -sSL "$OODLE_DOWNLOAD_URL" -o "$OODLE_TARGET_PATH"

    echo "==> Verifying SHA-256 for ${OODLE_ASSET_NAME}"
    if ! verify_checksum "$OODLE_TARGET_PATH" "$OODLE_EXPECTED_SHA256"; then
        actual="$(sha256sum "$OODLE_TARGET_PATH" | awk '{print $1}')"
        rm -f "$OODLE_TARGET_PATH"
        echo "Checksum mismatch for ${OODLE_ASSET_NAME}: expected ${OODLE_EXPECTED_SHA256}, got ${actual}. File deleted." >&2
        exit 1
    fi
    chmod 0755 "$OODLE_TARGET_PATH"
fi
ln -sf "$OODLE_ASSET_NAME" "${TARGET_DIR}/liboo2core.so"

echo "==> Confirming the worker reports the pinned source revision"
VERSION_OUTPUT="$("$TARGET_EXE" --version)"
if [[ "$VERSION_OUTPUT" != *"$EXPECTED_SOURCE_REVISION"* ]]; then
    echo "Worker reported version '$VERSION_OUTPUT', which does not contain $EXPECTED_SOURCE_REVISION" >&2
    exit 1
fi

echo "Pinned worker verified: $VERSION_OUTPUT"
