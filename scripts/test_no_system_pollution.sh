#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
    echo "usage: $0 COMMAND [ARGUMENT ...]" >&2
    exit 2
fi

SYSTEM_EXECUTABLE=/usr/local/bin/proofboard
BEFORE_STATE=absent
BEFORE_HASH=

if [ -e "$SYSTEM_EXECUTABLE" ]; then
    BEFORE_STATE=present
    BEFORE_HASH=$(sha256sum "$SYSTEM_EXECUTABLE" | awk '{print $1}')
fi

"$@"

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
