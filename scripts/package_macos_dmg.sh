#!/bin/sh
# Build the macOS disk image from an already-built binary.
#
# The .pkg installs; the .dmg is the drag-and-open format macOS users expect
# to download. It carries the executable plus the installer package, so a user
# who opens the image can either run the binary directly or double-click the
# package.
set -e

if [ "$#" -ne 4 ]; then
    echo "usage: $0 VERSION BINARY PKG OUTPUT.dmg" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
PKG=$3
OUTPUT=$4

[ -f "$BINARY" ] || { echo "binary not found: $BINARY" >&2; exit 1; }
[ -f "$PKG" ] || { echo "package not found: $PKG" >&2; exit 1; }

STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
ROOT="$STAGE/Proofboard Career Agent"
mkdir -p "$ROOT"
install -m 0755 "$BINARY" "$ROOT/proofboard"
cp "$PKG" "$ROOT/$(basename "$PKG")"

cat > "$ROOT/README.txt" <<TXT
Proofboard Career Agent ${VERSION}

Double-click the .pkg to install, which also registers the background agent.

To install the executable by hand instead:
  sudo cp proofboard /usr/local/bin/proofboard
  proofboard install
TXT

mkdir -p "$(dirname "$OUTPUT")"
rm -f "$OUTPUT"
hdiutil create -volname "Proofboard ${VERSION}" -srcfolder "$ROOT" -ov -format UDZO "$OUTPUT" >/dev/null
echo "built $OUTPUT"
