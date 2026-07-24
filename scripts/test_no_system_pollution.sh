#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
    echo "usage: $0 COMMAND [ARGUMENT ...]" >&2
    exit 2
fi

ORIGINAL_GO_CACHE=${GOCACHE:-}
ORIGINAL_GO_MOD_CACHE=${GOMODCACHE:-}
ORIGINAL_GO_PATH=${GOPATH:-}
if command -v go >/dev/null 2>&1; then
    [ -n "$ORIGINAL_GO_CACHE" ] || ORIGINAL_GO_CACHE=$(go env GOCACHE)
    [ -n "$ORIGINAL_GO_MOD_CACHE" ] || ORIGINAL_GO_MOD_CACHE=$(go env GOMODCACHE)
    [ -n "$ORIGINAL_GO_PATH" ] || ORIGINAL_GO_PATH=$(go env GOPATH)
fi

ISOLATED_HOME=$(mktemp -d)
trap 'rm -rf "$ISOLATED_HOME"' EXIT
export HOME="$ISOLATED_HOME/home"
export XDG_CONFIG_HOME="$ISOLATED_HOME/xdg-config"
export XDG_DATA_HOME="$ISOLATED_HOME/xdg-data"
export XDG_STATE_HOME="$ISOLATED_HOME/xdg-state"
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME"
[ -z "$ORIGINAL_GO_CACHE" ] || export GOCACHE="$ORIGINAL_GO_CACHE"
[ -z "$ORIGINAL_GO_MOD_CACHE" ] || export GOMODCACHE="$ORIGINAL_GO_MOD_CACHE"
[ -z "$ORIGINAL_GO_PATH" ] || export GOPATH="$ORIGINAL_GO_PATH"

SYSTEM_EXECUTABLE=/usr/local/bin/proofboard
BEFORE_STATE=absent
BEFORE_HASH=

if [ -e "$SYSTEM_EXECUTABLE" ]; then
    BEFORE_STATE=present
    BEFORE_HASH=$(sha256sum "$SYSTEM_EXECUTABLE" | awk '{print $1}')
fi

set +e
"$@"
COMMAND_STATUS=$?
set -e

if [ "$BEFORE_STATE" = absent ]; then
    if [ -e "$SYSTEM_EXECUTABLE" ]; then
        echo "test pollution detected: $SYSTEM_EXECUTABLE was created" >&2
        exit 1
    fi
else
    if [ ! -e "$SYSTEM_EXECUTABLE" ]; then
        echo "test pollution detected: $SYSTEM_EXECUTABLE was removed" >&2
        exit 1
    fi
    AFTER_HASH=$(sha256sum "$SYSTEM_EXECUTABLE" | awk '{print $1}')
    if [ "$BEFORE_HASH" != "$AFTER_HASH" ]; then
        echo "test pollution detected: $SYSTEM_EXECUTABLE was modified" >&2
        exit 1
    fi
fi

echo "system installation pollution: none"
exit "$COMMAND_STATUS"
