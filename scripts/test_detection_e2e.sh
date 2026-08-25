#!/bin/sh
# Reproduce auto project detection the way a developer actually experiences it:
# install the CLI, open a real interactive shell, cd into a repository that is
# not connected, and check that the prompt appears.
#
# The unit tests around this check that the hook is written to an rc file and
# that `detect` returns the right action. Neither proves the thing users care
# about, which is whether cd-ing into a repo in a live terminal produces the
# message. That needs a real shell, a real pty (bash only runs PROMPT_COMMAND
# when it draws a prompt) and a real cd.
#
# usage: test_detection_e2e.sh /path/to/proofboard [shell...]
set -e

[ "$#" -ge 1 ] || { echo "usage: $0 BINARY [shell...]" >&2; exit 2; }
BINARY=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
shift
SHELLS=${*:-bash}

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0
report_pass() { echo "  ok   $1"; PASS=$((PASS + 1)); }
report_fail() { echo "  FAIL $1" >&2; FAIL=$((FAIL + 1)); }

# Two things are required to make bash fire its hook, and both were wrong in
# the obvious version of this test.
#
# A pty, because bash evaluates PROMPT_COMMAND only when it draws a prompt and
# will not draw one onto a plain pipe. BSD and GNU `script` disagree on
# argument order, hence the two forms.
#
# And commands fed on STDIN rather than with -i -c, because `bash -i -c "cd X"`
# runs the command and exits without ever drawing a prompt — so PROMPT_COMMAND
# never runs and the test reports a detection failure that is entirely its own.
# zsh does not care, since its chpwd hook fires on the cd itself; feeding both
# shells the same way keeps the harness honest about what it proved.
run_shell_with_input() {
    _shell=$1
    _input=$2
    if script --version 2>/dev/null | grep -q util-linux; then
        script -qec "$_shell -i" /dev/null < "$_input"
    else
        script -q /dev/null "$_shell" -i < "$_input"
    fi
}

make_repo() {
    _dir=$1
    mkdir -p "$_dir"
    git -C "$_dir" init -q
    git -C "$_dir" config user.email detection@proofboard.io
    git -C "$_dir" config user.name "Detection Test"
    git -C "$_dir" remote add origin https://github.com/acme/detection-probe.git
    echo probe > "$_dir/probe.txt"
    git -C "$_dir" add probe.txt
    git -C "$_dir" commit -qm "probe"
}

for SHELL_NAME in $SHELLS; do
    SHELL_BIN=$(command -v "$SHELL_NAME" 2>/dev/null || true)
    if [ -z "$SHELL_BIN" ]; then
        echo "  skip $SHELL_NAME (not installed)"
        continue
    fi

    HOME_DIR="$WORK/home-$SHELL_NAME"
    REPO_DIR="$WORK/repo-$SHELL_NAME"
    mkdir -p "$HOME_DIR"
    make_repo "$REPO_DIR"

    # install writes the shell hooks for whichever shell it is told about.
    HOME="$HOME_DIR" SHELL="$SHELL_BIN" \
        PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS=1 \
        "$BINARY" install >"$WORK/install-$SHELL_NAME.log" 2>&1 || {
            report_fail "$SHELL_NAME: proofboard install failed"
            sed 's/^/      /' "$WORK/install-$SHELL_NAME.log" >&2
            continue
        }

    # 1. The hook has to be on disk before anything else can be true.
    if grep -rq "_proofboard_chpwd" "$HOME_DIR" 2>/dev/null; then
        report_pass "$SHELL_NAME: directory-change hook installed"
    else
        report_fail "$SHELL_NAME: no directory-change hook written to the rc files"
        continue
    fi

    # 2. The real thing: a live shell, a real cd, the message on the terminal.
    OUTPUT="$WORK/out-$SHELL_NAME.txt"
    INPUT="$WORK/in-$SHELL_NAME.txt"
    # cd away and back so the hook sees a genuine change of repository, then
    # give the shell a moment to draw one more prompt before exiting.
    {
        printf 'cd "%s"\n' "$REPO_DIR"
        printf 'cd /\n'
        printf 'cd "%s"\n' "$REPO_DIR"
        printf 'exit\n'
    } > "$INPUT"

    HOME="$HOME_DIR" SHELL="$SHELL_BIN" \
        PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS=1 \
        PATH="$HOME_DIR/.local/bin:$PATH" \
        run_shell_with_input "$SHELL_BIN" "$INPUT" >"$OUTPUT" 2>&1 || true

    if grep -q "New repository detected" "$OUTPUT"; then
        report_pass "$SHELL_NAME: cd into an unconnected repository prompts"
    else
        report_fail "$SHELL_NAME: cd produced no detection prompt"
        echo "      --- shell output ---" >&2
        sed 's/^/      /' "$OUTPUT" >&2 | head -20
    fi
done

echo "  $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
