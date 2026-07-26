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

# The test changes into an isolated temporary directory for one command. Make
# the executable path absolute up front so callers can pass e.g.
# `dist/proofboard-linux-amd64` without that later directory change breaking
# the test.
COMPILED_BINARY=$(CDPATH= cd -- "$(dirname -- "$COMPILED_BINARY")" && pwd)/$(basename -- "$COMPILED_BINARY")

export PROOFBOARD_DISABLE_STARTUP_CHECKS=1
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

"$COMPILED_BINARY" auth logout --help > "$TEST_ARTIFACT_DIR/auth-logout-help.txt"
grep -q 'Usage:' "$TEST_ARTIFACT_DIR/auth-logout-help.txt"

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

"$COMPILED_BINARY" status > "$TEST_ARTIFACT_DIR/status.txt"
grep -q '^Proofboard Career Agent$' "$TEST_ARTIFACT_DIR/status.txt"
grep -q '^Tracking 0 repositories$' "$TEST_ARTIFACT_DIR/status.txt"
grep -q '^Authentication: Not connected$' "$TEST_ARTIFACT_DIR/status.txt"

"$COMPILED_BINARY" status --json > "$TEST_ARTIFACT_DIR/status.json"
grep -q '"product":"Proofboard Career Agent"' "$TEST_ARTIFACT_DIR/status.json"
grep -q '"repositoriesTracked":0' "$TEST_ARTIFACT_DIR/status.json"

"$COMPILED_BINARY" agent status > "$TEST_ARTIFACT_DIR/agent-status.txt"
grep -q '^Proofboard Career Agent: Stopped$' "$TEST_ARTIFACT_DIR/agent-status.txt"
"$COMPILED_BINARY" agent > "$TEST_ARTIFACT_DIR/agent.txt"
cmp "$TEST_ARTIFACT_DIR/agent-status.txt" "$TEST_ARTIFACT_DIR/agent.txt"
"$COMPILED_BINARY" agent stop > "$TEST_ARTIFACT_DIR/agent-stop.txt"
grep -q '^Proofboard Career Agent is not running.$' "$TEST_ARTIFACT_DIR/agent-stop.txt"

"$COMPILED_BINARY" logs --lines 10 > "$TEST_ARTIFACT_DIR/logs.txt"
grep -q '^No Proofboard logs found.$' "$TEST_ARTIFACT_DIR/logs.txt"
mkdir -p "$HOME/.proofboard"
printf '%s\n' 'current log entry' > "$HOME/.proofboard/sync.log"
printf '%s\n' 'rotated log entry' > "$HOME/.proofboard/sync.log.1"
chmod 600 "$HOME/.proofboard/sync.log" "$HOME/.proofboard/sync.log.1"
"$COMPILED_BINARY" logs clear > "$TEST_ARTIFACT_DIR/logs-clear.txt"
grep -q '^Proofboard logs cleared.$' "$TEST_ARTIFACT_DIR/logs-clear.txt"
test ! -s "$HOME/.proofboard/sync.log"
test ! -e "$HOME/.proofboard/sync.log.1"
test "$(stat -c '%a' "$HOME/.proofboard/sync.log")" = 600

"$COMPILED_BINARY" config set auto-update-dictionary false > "$TEST_ARTIFACT_DIR/config-set.txt"
grep -q '^auto-update-dictionary=false$' "$TEST_ARTIFACT_DIR/config-set.txt"
"$COMPILED_BINARY" config add-branch release
"$COMPILED_BINARY" config branches > "$TEST_ARTIFACT_DIR/config-branches.txt"
grep -q '^release$' "$TEST_ARTIFACT_DIR/config-branches.txt"
"$COMPILED_BINARY" config remove-branch release
if "$COMPILED_BINARY" config branches | grep -q '^release$'; then
    echo "compiled config remove-branch did not remove release" >&2
    exit 1
fi
"$COMPILED_BINARY" config add-ide proofboard-test-ide
"$COMPILED_BINARY" config ides > "$TEST_ARTIFACT_DIR/config-ides.txt"
grep -q '^proofboard-test-ide$' "$TEST_ARTIFACT_DIR/config-ides.txt"
"$COMPILED_BINARY" config remove-ide proofboard-test-ide
if "$COMPILED_BINARY" config ides | grep -q '^proofboard-test-ide$'; then
    echo "compiled config remove-ide did not remove proofboard-test-ide" >&2
    exit 1
fi

(cd "$TEST_ARTIFACT_DIR" && "$COMPILED_BINARY" detect --json > "$TEST_ARTIFACT_DIR/detect.txt")
test ! -s "$TEST_ARTIFACT_DIR/detect.txt"
"$COMPILED_BINARY" notify
"$COMPILED_BINARY" notify-activate

touch "$HOME/.bashrc" "$HOME/.bash_profile"
SHELL=/bin/bash "$COMPILED_BINARY" hook-maintain
grep -qF '(proofboard detect >/dev/null 2>&1 &)' "$HOME/.bashrc"
grep -qF '(proofboard detect >/dev/null 2>&1 &)' "$HOME/.bash_profile"
rm -f "$HOME/.bashrc" "$HOME/.bash_profile"

for profile in \
    "$HOME/.profile" "$HOME/.bashrc" "$HOME/.bash_profile" \
    "$HOME/.zshrc" "$HOME/.zprofile" "$HOME/.config/fish/config.fish"
do
    if [ -e "$profile" ]; then
        echo "compiled command unexpectedly modified shell profile: $profile" >&2
        exit 1
    fi
done

echo "compiled command surface: verified"
