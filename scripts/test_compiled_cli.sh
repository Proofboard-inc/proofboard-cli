#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
    echo "usage: $0 COMPILED_BINARY EXPECTED_VERSION" >&2
    exit 2
fi

COMPILED_BINARY=$1
EXPECTED_VERSION=$2

if [ ! -x "$COMPILED_BINARY" ]; then
    echo "compiled executable not found: $COMPILED_BINARY" >&2
    exit 1
fi

export PROOFBOARD_DISABLE_STARTUP_CHECKS=1
export PROOFBOARD_DISABLE_SHELL_HOOK_MAINTENANCE=1
export PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS=1
export NO_BROWSER=1

TEST_ARTIFACT_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_ARTIFACT_DIR"' EXIT
export HOME="$TEST_ARTIFACT_DIR/home"
export XDG_CONFIG_HOME="$TEST_ARTIFACT_DIR/xdg-config"
export XDG_DATA_HOME="$TEST_ARTIFACT_DIR/xdg-data"
export XDG_STATE_HOME="$TEST_ARTIFACT_DIR/xdg-state"
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME"

"$COMPILED_BINARY" > "$TEST_ARTIFACT_DIR/root-help.txt"
grep -q 'Proofboard Career Agent' "$TEST_ARTIFACT_DIR/root-help.txt"

for command_name in \
    agent auth completion config detect help install link logs status sync \
    uninstall unlink update update-dictionary version
do
    "$COMPILED_BINARY" "$command_name" --help > "$TEST_ARTIFACT_DIR/$command_name-help.txt"
    grep -q 'Usage:' "$TEST_ARTIFACT_DIR/$command_name-help.txt"
done

for internal_command in hook-maintain milestone-action notify notify-activate; do
    "$COMPILED_BINARY" "$internal_command" --help > "$TEST_ARTIFACT_DIR/$internal_command-help.txt"
    grep -q 'Usage:' "$TEST_ARTIFACT_DIR/$internal_command-help.txt"
done

for agent_command in run enable disable start stop status; do
    "$COMPILED_BINARY" agent "$agent_command" --help > "$TEST_ARTIFACT_DIR/agent-$agent_command-help.txt"
    grep -q 'Usage:' "$TEST_ARTIFACT_DIR/agent-$agent_command-help.txt"
done

for config_command in set add-branch remove-branch branches add-ide remove-ide ides; do
    "$COMPILED_BINARY" config "$config_command" --help > "$TEST_ARTIFACT_DIR/config-$config_command-help.txt"
    grep -q 'Usage:' "$TEST_ARTIFACT_DIR/config-$config_command-help.txt"
done

for shell_name in bash zsh fish powershell; do
    "$COMPILED_BINARY" completion "$shell_name" > "$TEST_ARTIFACT_DIR/completion-$shell_name.txt"
    test -s "$TEST_ARTIFACT_DIR/completion-$shell_name.txt"
done

version_output=$("$COMPILED_BINARY" version)
test "$version_output" = "proofboard version $EXPECTED_VERSION"
test "$("$COMPILED_BINARY" --version)" = "proofboard version $EXPECTED_VERSION"

echo "compiled command surface: verified"
