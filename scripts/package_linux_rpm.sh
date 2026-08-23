#!/bin/sh
# Build the RPM package from an already-built binary.
#
# Uses rpmbuild directly rather than a converter so the spec is readable and
# the scriptlets are the ones we wrote. Mirrors package_linux_deb.sh: install
# the executable to /usr/bin and register the background agent on install.
set -e

if [ "$#" -ne 3 ]; then
    echo "usage: $0 VERSION BINARY OUTPUT.rpm" >&2
    exit 2
fi

VERSION=${1#v}
BINARY=$2
OUTPUT=$3

[ -f "$BINARY" ] || { echo "binary not found: $BINARY" >&2; exit 1; }
case "$VERSION" in
    ''|*[!0-9A-Za-z.+~]*) echo "invalid package version: $VERSION" >&2; exit 1 ;;
esac

BUILD_TMP=$(mktemp -d)
trap 'rm -rf "$BUILD_TMP"' EXIT
# Listed one per line rather than with brace expansion, which is a bash
# feature: under POSIX sh this silently creates a directory literally named
# "{BUILD,RPMS,...}" and the build then fails on a missing SOURCES.
for d in BUILD RPMS SOURCES SPECS BUILDROOT; do
    mkdir -p "$BUILD_TMP/rpmbuild/$d"
done
install -m 0755 "$BINARY" "$BUILD_TMP/rpmbuild/SOURCES/proofboard"

cat > "$BUILD_TMP/rpmbuild/SPECS/proofboard.spec" <<SPEC
Name:           proofboard
Version:        ${VERSION}
Release:        1
Summary:        Proofboard Career Agent
License:        MIT
URL:            https://proofboard.io
BuildArch:      x86_64
# The binary is statically linked and built elsewhere; rpmbuild must not try
# to derive dependencies from it or strip it, which would invalidate the
# release signature the updater verifies.
AutoReqProv:    no
%global __os_install_post %{nil}
%global debug_package %{nil}

%description
Proofboard builds your engineering career record locally. The Career Agent
detects repositories, captures milestones privately on this machine, and
synchronises only anonymised metadata.

%install
mkdir -p %{buildroot}/usr/bin
install -m 0755 %{_sourcedir}/proofboard %{buildroot}/usr/bin/proofboard

%files
/usr/bin/proofboard

%post
# Best effort: a package install must not fail because the agent could not be
# registered, which can happen in a container or an image build with no user
# session at all.
/usr/bin/proofboard agent enable >/dev/null 2>&1 || true

%preun
if [ \$1 -eq 0 ]; then
    /usr/bin/proofboard agent disable >/dev/null 2>&1 || true
fi

%changelog
SPEC

rpmbuild --define "_topdir $BUILD_TMP/rpmbuild" -bb "$BUILD_TMP/rpmbuild/SPECS/proofboard.spec" >/dev/null
BUILT=$(find "$BUILD_TMP/rpmbuild/RPMS" -name '*.rpm' -print -quit)
[ -n "$BUILT" ] || { echo "rpmbuild produced no package" >&2; exit 1; }
mkdir -p "$(dirname "$OUTPUT")"
cp "$BUILT" "$OUTPUT"
echo "built $OUTPUT"
