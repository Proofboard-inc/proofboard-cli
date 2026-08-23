#!/bin/sh
# Build the AppImage from an already-built binary.
#
# An AppImage is a single self-contained file the user marks executable and
# runs — no package manager, no root. That makes it the right format for
# distributions the .deb and .rpm do not cover, and for users without admin
# rights on their machine.
set -e

if [ "$#" -ne 3 ]; then
    echo "usage: $0 VERSION BINARY OUTPUT.AppImage" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
OUTPUT=$3

[ -f "$BINARY" ] || { echo "binary not found: $BINARY" >&2; exit 1; }

BUILD_TMP=$(mktemp -d)
trap 'rm -rf "$BUILD_TMP"' EXIT
APPDIR="$BUILD_TMP/Proofboard.AppDir"
mkdir -p "$APPDIR/usr/bin"
install -m 0755 "$BINARY" "$APPDIR/usr/bin/proofboard"

# AppRun is what the AppImage executes. Forwarding arguments matters here:
# this is a command-line tool, so `./Proofboard.AppImage sync` has to reach
# the executable intact rather than launching a bare session.
cat > "$APPDIR/AppRun" <<'RUN'
#!/bin/sh
HERE=$(dirname "$(readlink -f "$0")")
exec "$HERE/usr/bin/proofboard" "$@"
RUN
chmod 0755 "$APPDIR/AppRun"

cat > "$APPDIR/proofboard.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Proofboard Career Agent
Comment=Builds your engineering career record locally
Exec=proofboard
Icon=proofboard
Categories=Development;
Terminal=true
DESKTOP

# appimagetool requires an icon to exist. A minimal valid PNG keeps the
# package self-consistent without pulling design assets into the build.
printf '\211PNG\r\n\032\n\0\0\0\rIHDR\0\0\0\1\0\0\0\1\10\6\0\0\0\37\25\304\211\0\0\0\nIDATx\234c\370\17\0\1\1\1\0\30\335\215\260\0\0\0\0IEND\256B`\202' > "$APPDIR/proofboard.png"

APPIMAGETOOL=${APPIMAGETOOL:-appimagetool}
command -v "$APPIMAGETOOL" >/dev/null 2>&1 || {
    echo "appimagetool not found on PATH" >&2
    exit 1
}

mkdir -p "$(dirname "$OUTPUT")"
# --appimage-extract-and-run avoids needing FUSE, which is unavailable on most
# CI runners and in containers.
ARCH=x86_64 "$APPIMAGETOOL" --appimage-extract-and-run "$APPDIR" "$OUTPUT" >/dev/null 2>&1 || \
    ARCH=x86_64 "$APPIMAGETOOL" "$APPDIR" "$OUTPUT"
chmod 0755 "$OUTPUT"
echo "built $OUTPUT"
