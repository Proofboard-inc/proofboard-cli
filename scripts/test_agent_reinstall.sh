#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
    echo "usage: $0 COMPILED_BINARY" >&2
    exit 2
fi

COMPILED_BINARY=$(CDPATH= cd -- "$(dirname -- "$1")" && pwd)/$(basename -- "$1")
if [ ! -x "$COMPILED_BINARY" ]; then
    echo "compiled executable not found: $COMPILED_BINARY" >&2
    exit 1
fi

TEST_ROOT=$(mktemp -d)
export HOME="$TEST_ROOT/home"
export XDG_CONFIG_HOME="$TEST_ROOT/xdg-config"
export XDG_DATA_HOME="$TEST_ROOT/xdg-data"
export XDG_STATE_HOME="$TEST_ROOT/xdg-state"
export PROOFBOARD_DISABLE_STARTUP_CHECKS=1
export PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS=1
export NO_BROWSER=1
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME"

INSTALLED_BINARY="$HOME/.local/bin/proofboard"
PID_FILE="$HOME/.proofboard/agent.pid"

cleanup() {
    if [ -x "$INSTALLED_BINARY" ]; then
        "$INSTALLED_BINARY" agent stop >/dev/null 2>&1 || true
    fi
    rm -rf "$TEST_ROOT"
}
trap cleanup EXIT HUP INT TERM

wait_for_agent_pid() {
    previous_pid=${1:-}
    attempts=0
    while [ "$attempts" -lt 100 ]; do
        if [ -s "$PID_FILE" ]; then
            current_pid=$(tr -d '[:space:]' < "$PID_FILE")
            if [ -n "$current_pid" ] &&
                [ "$current_pid" != "$previous_pid" ] &&
                kill -0 "$current_pid" 2>/dev/null; then
                printf '%s\n' "$current_pid"
                return 0
            fi
        fi
        attempts=$((attempts + 1))
        sleep 0.05
    done
    echo "Career Agent did not start with a new live PID" >&2
    return 1
}

"$COMPILED_BINARY" install >/dev/null
first_pid=$(wait_for_agent_pid)
first_executable=$(readlink "/proc/$first_pid/exe")
if [ "$first_executable" != "$INSTALLED_BINARY" ]; then
    echo "first installed agent runs unexpected executable: $first_executable" >&2
    exit 1
fi

"$COMPILED_BINARY" install >/dev/null
second_pid=$(wait_for_agent_pid "$first_pid")
second_executable=$(readlink "/proc/$second_pid/exe")
if [ "$second_executable" != "$INSTALLED_BINARY" ]; then
    echo "reinstalled agent runs unexpected executable: $second_executable" >&2
    exit 1
fi
if kill -0 "$first_pid" 2>/dev/null; then
    echo "old Career Agent PID $first_pid survived reinstall" >&2
    exit 1
fi
if ! cmp -s "$COMPILED_BINARY" "$INSTALLED_BINARY"; then
    echo "installed executable differs from compiled executable" >&2
    exit 1
fi

echo "installed Career Agent replacement and restart: verified"
