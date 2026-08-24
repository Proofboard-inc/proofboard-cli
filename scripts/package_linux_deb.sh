#!/bin/sh
set -e

# ARCH is the Debian architecture name (amd64, arm64) and defaults to amd64 so
# existing callers keep working. It must match the binary being packaged: a
# package claiming the wrong architecture installs and then cannot run.
if [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; then
    echo "usage: $0 VERSION BINARY OUTPUT.deb [ARCH]" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
OUTPUT=$3
ARCH=${4:-amd64}

if [ ! -f "$BINARY" ]; then
    echo "binary not found: $BINARY" >&2
    exit 1
fi
case "$VERSION" in
    ''|*[!0-9A-Za-z.+~-]*) echo "invalid package version: $VERSION" >&2; exit 1 ;;
esac

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(dirname "$SCRIPT_DIR")
PACKAGE_TMP=$(mktemp -d)
trap 'rm -rf "$PACKAGE_TMP"' EXIT

mkdir -p "$PACKAGE_TMP/DEBIAN" "$PACKAGE_TMP/usr/bin" "$PACKAGE_TMP/etc/xdg/autostart"
chmod 0755 \
    "$PACKAGE_TMP" \
    "$PACKAGE_TMP/DEBIAN" \
    "$PACKAGE_TMP/usr" \
    "$PACKAGE_TMP/usr/bin" \
    "$PACKAGE_TMP/etc" \
    "$PACKAGE_TMP/etc/xdg" \
    "$PACKAGE_TMP/etc/xdg/autostart"
sed -e "s/@VERSION@/$VERSION/g" -e "s/@ARCH@/$ARCH/g" "$PROJECT_DIR/packaging/linux/control.in" > "$PACKAGE_TMP/DEBIAN/control"
install -m 0755 "$PROJECT_DIR/packaging/linux/postinst" "$PACKAGE_TMP/DEBIAN/postinst"
install -m 0755 "$PROJECT_DIR/packaging/linux/prerm" "$PACKAGE_TMP/DEBIAN/prerm"
install -m 0755 "$BINARY" "$PACKAGE_TMP/usr/bin/proofboard"
install -m 0644 "$PROJECT_DIR/packaging/linux/proofboard-career-agent.desktop" "$PACKAGE_TMP/etc/xdg/autostart/proofboard-career-agent.desktop"

mkdir -p "$(dirname "$OUTPUT")"
dpkg-deb --root-owner-group --build "$PACKAGE_TMP" "$OUTPUT"
