#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"

echo "==> Verifying environment and toolchain"

# Load mise environment if available
if command -v mise >/dev/null 2>&1; then
    eval "$(mise env -s bash)"
elif [ -f "${HOME}/.local/bin/mise" ]; then
    eval "$("${HOME}/.local/bin/mise" env -s bash)"
fi

# Ensure user local bin is in PATH
if [ -d "${HOME}/.local/bin" ]; then
    export PATH="${HOME}/.local/bin:${PATH}"
fi

# Check required commands
for cmd in go bun; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
        echo "Error: required tool '${cmd}' not found in PATH." >&2
        exit 1
    fi
done

# Ensure appimagetool is available
if ! command -v appimagetool >/dev/null 2>&1; then
    echo "==> Downloading appimagetool"
    mkdir -p "${HOME}/.local/bin"
    curl -fsSL -o "${HOME}/.local/bin/appimagetool" \
        https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
    chmod +x "${HOME}/.local/bin/appimagetool"
fi

# Ensure UAssetTool Linux worker and Oodle library are present
if [ ! -f "${REPO_ROOT}/build/uassettool-linux/UAssetTool" ] || [ ! -f "${REPO_ROOT}/build/uassettool-linux/liboo2corelinux64.so.9" ]; then
    echo "==> Fetching UAssetTool worker binary and Oodle library"
    bash "${REPO_ROOT}/fetch-uassettool.sh"
fi

echo "==> Building Cratebug with Wails"
cd "${REPO_ROOT}"
BUILD_LDFLAGS="-w -s"
if [ -n "${APP_VERSION:-}" ]; then
    BUILD_LDFLAGS="${BUILD_LDFLAGS} -X main.AppVersion=${APP_VERSION}"
fi
go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -tags webkit2_41 -trimpath -ldflags "${BUILD_LDFLAGS}" -o Cratebug

BIN_PATH="${REPO_ROOT}/build/bin/Cratebug"
if [ ! -f "${BIN_PATH}" ]; then
    echo "Error: build output binary not found at ${BIN_PATH}" >&2
    exit 1
fi

echo "==> Preparing AppDir"
APPDIR="${REPO_ROOT}/build/linux/AppDir"
rm -rf "${APPDIR}"
mkdir -p "${APPDIR}/usr/bin" \
         "${APPDIR}/usr/share/applications" \
         "${APPDIR}/usr/share/icons/hicolor/256x256/apps" \
         "${APPDIR}/usr/share/doc/cratebug"

# Install main application binary
cp "${BIN_PATH}" "${APPDIR}/usr/bin/Cratebug"
chmod 0755 "${APPDIR}/usr/bin/Cratebug"

# Install AppRun launcher
cp "${REPO_ROOT}/build/linux/AppRun" "${APPDIR}/AppRun"
chmod 0755 "${APPDIR}/AppRun"

# Install desktop entry
cp "${REPO_ROOT}/build/linux/cratebug.desktop" "${APPDIR}/cratebug.desktop"
cp "${REPO_ROOT}/build/linux/cratebug.desktop" "${APPDIR}/usr/share/applications/cratebug.desktop"

# Install icons
cp "${REPO_ROOT}/build/appicon.png" "${APPDIR}/cratebug.png"
cp "${REPO_ROOT}/build/appicon.png" "${APPDIR}/.DirIcon"
cp "${REPO_ROOT}/build/appicon.png" "${APPDIR}/usr/share/icons/hicolor/256x256/apps/cratebug.png"

# Install third-party notices
if [ -f "${REPO_ROOT}/THIRD_PARTY_NOTICES.md" ]; then
    cp "${REPO_ROOT}/THIRD_PARTY_NOTICES.md" "${APPDIR}/usr/share/doc/cratebug/THIRD_PARTY_NOTICES.md"
fi

# Install UAssetTool worker and dependencies into usr/bin/uassettool/
echo "==> Bundling UAssetTool worker"
WORKER_DEST="${APPDIR}/usr/bin/uassettool"
mkdir -p "${WORKER_DEST}"

# Copy all worker files excluding archives, debug symbols, and Windows native dlls
find "${REPO_ROOT}/build/uassettool-linux" -maxdepth 1 -type f \
    ! -name "*.tar.gz" \
    ! -name "*.zip" \
    ! -name "*.pdb" \
    ! -name "oo2core*.dll" \
    -exec cp {} "${WORKER_DEST}/" \;

# Ensure liboo2core.so symlink exists next to worker binary
if [ -f "${WORKER_DEST}/liboo2corelinux64.so.9" ]; then
    ln -sf "liboo2corelinux64.so.9" "${WORKER_DEST}/liboo2core.so"
fi

chmod 0755 "${WORKER_DEST}/UAssetTool"

echo "==> Packaging AppImage"
OUTPUT_APPIMAGE="${REPO_ROOT}/build/bin/Cratebug-x86_64.AppImage"
rm -f "${OUTPUT_APPIMAGE}"

APPIMAGE_ARGS=("${APPDIR}" "${OUTPUT_APPIMAGE}")
if ! ARCH=x86_64 appimagetool "${APPIMAGE_ARGS[@]}" 2>/dev/null; then
    echo "==> Retrying with --appimage-extract-and-run"
    ARCH=x86_64 appimagetool --appimage-extract-and-run "${APPIMAGE_ARGS[@]}"
fi

if [ ! -f "${OUTPUT_APPIMAGE}" ]; then
    echo "Error: AppImage packaging failed to create ${OUTPUT_APPIMAGE}" >&2
    exit 1
fi

chmod +x "${OUTPUT_APPIMAGE}"
echo "==> Successfully created ${OUTPUT_APPIMAGE}"
ls -lh "${OUTPUT_APPIMAGE}"
