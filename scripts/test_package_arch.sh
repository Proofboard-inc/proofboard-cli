#!/bin/sh
# Verify the Linux packagers produce packages for the architecture they are
# asked for, in both the metadata and the payload.
#
# This exists because the failure mode here is silent. Asking appimagetool for
# aarch64 by setting ARCH alone produced an AppImage that reported success,
# carried the arm64 binary, and embedded an x86-64 runtime — a file that cannot
# start on the machine it is built for. Nothing about the build said so. The
# equivalent mistake in a .deb or .rpm installs and then fails to execute.
#
# Skips cleanly when a packaging tool is absent so it can run anywhere; when a
# tool is present the assertions are hard.
set -e

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# RPM versions may not contain a hyphen, so this stays a plain version number.
VERSION=v0.0.0
FAILED=0

pass() { echo "  ok   $1"; }
skip() { echo "  skip $1"; }
fail() { echo "  FAIL $1" >&2; FAILED=1; }

# e_machine is a 2-byte little-endian field at offset 0x12 of an ELF header.
elf_machine() { od -A n -t x1 -j 18 -N 2 "$1" | tr -d ' \n'; }

echo "building test binaries"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
    -o "$WORK/proofboard-arm64" "$PROJECT_DIR/cmd/proofboard"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -o "$WORK/proofboard-amd64" "$PROJECT_DIR/cmd/proofboard"

echo "deb"
if command -v dpkg-deb >/dev/null 2>&1; then
    for pair in "arm64 arm64" "amd64 amd64"; do
        DEB_ARCH=${pair% *}; GO_ARCH=${pair#* }
        sh "$PROJECT_DIR/scripts/package_linux_deb.sh" "$VERSION" \
            "$WORK/proofboard-$GO_ARCH" "$WORK/out-$DEB_ARCH.deb" "$DEB_ARCH" >/dev/null
        GOT=$(dpkg-deb -f "$WORK/out-$DEB_ARCH.deb" Architecture)
        [ "$GOT" = "$DEB_ARCH" ] \
            && pass "deb declares $DEB_ARCH" \
            || fail "deb declares $GOT, wanted $DEB_ARCH"
    done
else
    skip "deb (dpkg-deb not installed)"
fi

echo "rpm"
if command -v rpmbuild >/dev/null 2>&1; then
    for pair in "aarch64 arm64" "x86_64 amd64"; do
        RPM_ARCH=${pair% *}; GO_ARCH=${pair#* }
        sh "$PROJECT_DIR/scripts/package_linux_rpm.sh" "$VERSION" \
            "$WORK/proofboard-$GO_ARCH" "$WORK/out-$RPM_ARCH.rpm" "$RPM_ARCH" >/dev/null
        GOT=$(rpm -qp --qf '%{ARCH}' "$WORK/out-$RPM_ARCH.rpm" 2>/dev/null)
        [ "$GOT" = "$RPM_ARCH" ] \
            && pass "rpm declares $RPM_ARCH" \
            || fail "rpm declares $GOT, wanted $RPM_ARCH"
    done
else
    skip "rpm (rpmbuild not installed)"
fi

echo "appimage"
APPIMAGETOOL=${APPIMAGETOOL:-appimagetool}
if command -v "$APPIMAGETOOL" >/dev/null 2>&1; then
    # x86_64 needs no runtime override: appimagetool embeds its own. Clearing
    # APPIMAGE_RUNTIME matters — it is set for the aarch64 case below, and
    # leaving it set here builds an x86_64-labelled AppImage on an ARM runtime.
    APPIMAGE_RUNTIME= sh "$PROJECT_DIR/scripts/package_linux_appimage.sh" "$VERSION" \
        "$WORK/proofboard-amd64" "$WORK/out-x86_64.AppImage" x86_64 >/dev/null
    [ "$(elf_machine "$WORK/out-x86_64.AppImage")" = "3e00" ] \
        && pass "AppImage runtime is x86-64" \
        || fail "AppImage runtime is not x86-64"

    # Asking for a foreign architecture without a matching runtime is the
    # silent failure this test is here for: it must be refused, not built.
    if APPIMAGE_RUNTIME= sh "$PROJECT_DIR/scripts/package_linux_appimage.sh" "$VERSION" \
        "$WORK/proofboard-arm64" "$WORK/bad.AppImage" aarch64 >/dev/null 2>&1; then
        fail "aarch64 AppImage built with an x86-64 runtime instead of being refused"
    else
        pass "aarch64 AppImage without a runtime is refused"
    fi

    if [ -n "${APPIMAGE_RUNTIME:-}" ]; then
        sh "$PROJECT_DIR/scripts/package_linux_appimage.sh" "$VERSION" \
            "$WORK/proofboard-arm64" "$WORK/out-aarch64.AppImage" aarch64 >/dev/null
        [ "$(elf_machine "$WORK/out-aarch64.AppImage")" = "b700" ] \
            && pass "AppImage runtime is aarch64" \
            || fail "AppImage runtime is not aarch64"
    else
        skip "aarch64 AppImage (set APPIMAGE_RUNTIME to the aarch64 runtime)"
    fi
else
    skip "appimage (appimagetool not installed)"
fi

[ "$FAILED" -eq 0 ] || { echo "packaging architecture test failed" >&2; exit 1; }
echo "packaging architecture test passed"
