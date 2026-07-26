#!/bin/sh
set -e

if [ "$#" -ne 3 ]; then
    echo "usage: $0 VERSION BINARY OUTPUT-setup.exe" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
OUTPUT=$3

if [ ! -f "$BINARY" ]; then
    echo "binary not found: $BINARY" >&2
    exit 1
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(dirname "$SCRIPT_DIR")

ABS_OUTPUT=$(CDPATH= cd -- "$(dirname -- "$OUTPUT")" && pwd)/$(basename -- "$OUTPUT")

if command -v iscc >/dev/null 2>&1; then
    iscc "/DBinaryPath=$BINARY" "/DMyAppVersion=$VERSION" "/DInstallerOutputDir=$(dirname "$OUTPUT")" "$PROJECT_DIR/packaging/windows/ProofboardCareerAgent.iss"
else
    # Fallback for Linux cross-compilation environments without Inno Setup
    PACKAGE_TMP=$(mktemp -d)
    trap 'rm -rf "$PACKAGE_TMP"' EXIT
    mkdir -p "$PACKAGE_TMP/Proofboard" "$(dirname "$OUTPUT")"
    cp "$BINARY" "$PACKAGE_TMP/Proofboard/proofboard.exe"
    (cd "$PACKAGE_TMP" && zip -r -9 "$ABS_OUTPUT" Proofboard)
fi
