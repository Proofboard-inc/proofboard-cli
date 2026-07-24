#!/bin/sh
set -e

if [ "$#" -ne 3 ]; then
    echo "usage: $0 VERSION BINARY OUTPUT.pkg" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
OUTPUT=$3

if [ ! -f "$BINARY" ]; then
    echo "binary not found: $BINARY" >&2
    exit 1
fi
if ! command -v pkgbuild >/dev/null 2>&1; then
    echo "pkgbuild is required" >&2
    exit 1
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(dirname "$SCRIPT_DIR")
PACKAGE_TMP=$(mktemp -d)
trap 'rm -rf "$PACKAGE_TMP"' EXIT

mkdir -p "$PACKAGE_TMP/root/usr/local/bin" "$PACKAGE_TMP/scripts" "$(dirname "$OUTPUT")"
install -m 0755 "$BINARY" "$PACKAGE_TMP/root/usr/local/bin/proofboard"
install -m 0755 "$PROJECT_DIR/packaging/macos/postinstall" "$PACKAGE_TMP/scripts/postinstall"

pkgbuild \
    --root "$PACKAGE_TMP/root" \
    --scripts "$PACKAGE_TMP/scripts" \
    --identifier io.proofboard.career-agent \
    --version "$VERSION" \
    --install-location / \
    "$OUTPUT"
