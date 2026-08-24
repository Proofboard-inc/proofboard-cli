#!/bin/sh
# Build the AppImage from an already-built binary.
#
# An AppImage is a single self-contained file the user marks executable and
# runs — no package manager, no root. That makes it the right format for
# distributions the .deb and .rpm do not cover, and for users without admin
# rights on their machine.
set -e

# ARCH is the AppImage architecture name (x86_64, aarch64) and defaults to
# x86_64. Setting it alone is NOT enough to cross-build: appimagetool embeds
# the runtime of its own architecture and ARCH only labels the metadata, so an
# x86_64 appimagetool asked for aarch64 happily produces an AppImage with an
# x86-64 runtime that cannot start on an ARM machine. Set APPIMAGE_RUNTIME to
# the matching runtime binary when building for a foreign architecture; the
# check at the end of this script enforces that it was done.
if [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; then
    echo "usage: $0 VERSION BINARY OUTPUT.AppImage [ARCH]" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
OUTPUT=$3
ARCH=${4:-x86_64}

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

RUNTIME_ARGS=""
if [ -n "${APPIMAGE_RUNTIME:-}" ]; then
    [ -f "$APPIMAGE_RUNTIME" ] || {
        echo "APPIMAGE_RUNTIME set but not found: $APPIMAGE_RUNTIME" >&2
        exit 1
    }
    RUNTIME_ARGS="--runtime-file $APPIMAGE_RUNTIME"
fi

mkdir -p "$(dirname "$OUTPUT")"
# --appimage-extract-and-run avoids needing FUSE, which is unavailable on most
# CI runners and in containers. It must stay the FIRST argument: it is read by
# appimagetool's own AppImage runtime, which only inspects argv[1], so putting
# --runtime-file ahead of it silently drops back to FUSE and fails.
# shellcheck disable=SC2086 # RUNTIME_ARGS is deliberately word-split
ARCH="$ARCH" "$APPIMAGETOOL" --appimage-extract-and-run $RUNTIME_ARGS "$APPDIR" "$OUTPUT" >/dev/null 2>&1 || \
    ARCH="$ARCH" "$APPIMAGETOOL" $RUNTIME_ARGS "$APPDIR" "$OUTPUT"
chmod 0755 "$OUTPUT"

# Verify the runtime that actually got embedded. This is the failure this
# script exists to prevent: without it a cross-architecture build reports
# success and ships an AppImage that cannot execute on the target machine.
# e_machine is a 2-byte little-endian field at offset 0x12 of the ELF header.
MACHINE=$(od -A n -t x1 -j 18 -N 2 "$OUTPUT" | tr -d ' \n')
case "$ARCH" in
    x86_64)  WANT=3e00 ;;
    aarch64) WANT=b700 ;;
    armhf)   WANT=2800 ;;
    i686)    WANT=0300 ;;
    *)       WANT="" ;;
esac
if [ -n "$WANT" ] && [ "$MACHINE" != "$WANT" ]; then
    echo "built AppImage has the wrong runtime architecture for $ARCH" >&2
    echo "  expected ELF e_machine $WANT, found $MACHINE" >&2
    echo "  set APPIMAGE_RUNTIME to the $ARCH runtime binary" >&2
    exit 1
fi

echo "built $OUTPUT ($ARCH)"
