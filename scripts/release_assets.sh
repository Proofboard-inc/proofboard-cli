#!/bin/sh
# The single source of truth for what a release publishes.
#
# These names used to be spelled out in four separate hand-maintained lists in
# the release workflow — one to sign, one to verify, one to checksum, one to
# upload. Keeping four lists in step by hand failed twice in a way nobody could
# see from the outside: v1.16.0 published binaries that checksums.txt did not
# list, so every public install downloaded successfully and then failed to
# verify; and the ARM targets were added to some lists and not others, so the
# release advertised platforms it had no packages for.
#
# Every list is now derived from the matrix below. Adding a platform or a
# package format means editing one place, and a target cannot reach the release
# page without also being signed, checksummed and verified.
#
# usage: release_assets.sh <group>
#   binaries    product-named executables
#   legacy      lowercase executables (see below)
#   signed      everything the release key signs
#   signatures  the .sig file for each signed artifact
#   installers  the double-click packages
#   scripts     the install scripts published as assets
#   checksums   everything that belongs in checksums.txt
#   all         everything uploaded to the release
set -e

# os:goarch pairs Dartvel builds for. Windows executables carry .exe.
PLATFORMS='linux:amd64 linux:arm64 darwin:amd64 darwin:arm64 windows:amd64 windows:arm64'

# Package formats per OS, as extensions appended to <os>-<arch>.
FORMATS_linux='.deb .rpm .AppImage'
FORMATS_darwin='.pkg .dmg'
FORMATS_windows='-setup.exe .msi .msix'

binaries() {
    for pair in $PLATFORMS; do
        os=${pair%:*}; arch=${pair#*:}
        suffix=''; [ "$os" = windows ] && suffix='.exe'
        echo "Proofboard-Career-Agent-${os}-${arch}${suffix}"
    done
}

# The lowercase names are load-bearing and must not be dropped yet: an
# installed copy of 1.13.2 or earlier builds this exact filename, looks for it
# in the GitHub release's own asset list, and fails outright when it is absent.
# Those copies never go through releases.proofboard.io, so the name translation
# there cannot cover them — removing these strands those users with no way to
# update short of reinstalling by hand. They can go once telemetry shows no
# one is on <= 1.13.2, and not before.
legacy() {
    for pair in $PLATFORMS; do
        os=${pair%:*}; arch=${pair#*:}
        suffix=''; [ "$os" = windows ] && suffix='.exe'
        echo "proofboard-${os}-${arch}${suffix}"
    done
}

installers() {
    for pair in $PLATFORMS; do
        os=${pair%:*}; arch=${pair#*:}
        eval "formats=\$FORMATS_${os}"
        for ext in $formats; do
            echo "Proofboard-Career-Agent-${os}-${arch}${ext}"
        done
    done
}

signed() { binaries; legacy; }
signatures() { signed | sed 's/$/.sig/'; }
scripts_() { echo install.sh; echo install.ps1; echo install.cmd; echo install.bat; }

checksums() { signed; signatures; installers; scripts_; echo latest.json; }
all() { checksums; }

case "${1:-}" in
    binaries)   binaries ;;
    legacy)     legacy ;;
    signed)     signed ;;
    signatures) signatures ;;
    installers) installers ;;
    scripts)    scripts_ ;;
    checksums)  checksums ;;
    all)        all ;;
    *)
        echo "usage: $0 binaries|legacy|signed|signatures|installers|scripts|checksums|all" >&2
        exit 2
        ;;
esac
