#!/bin/bash
# Proofboard Career Agent — full journey against the compiled executable.
# auth -> device key -> connect project -> sync -> background detection -> agent
set -uo pipefail

PB=${PB:?set PB to the compiled executable under test}
WORK=$(mktemp -d)
export HOME="$WORK/home"
export XDG_CONFIG_HOME="$WORK/xdg"
mkdir -p "$HOME" "$XDG_CONFIG_HOME"
FAILED=0
STEP=0

check() {
  STEP=$((STEP+1))
  if [ "$2" = "$3" ]; then
    printf '  PASS  %-58s %s\n' "$1" "$2"
  else
    printf '  FAIL  %-58s got=%s want=%s\n' "$1" "$2" "$3"
    FAILED=1
  fi
}
contains() {
  STEP=$((STEP+1))
  if printf '%s' "$2" | grep -q -- "$3"; then
    printf '  PASS  %-58s\n' "$1"
  else
    printf '  FAIL  %-58s (missing %q in: %s)\n' "$1" "$3" "$(printf '%s' "$2" | head -c 200)"
    FAILED=1
  fi
}

SECRET_SUBJECT="feat: Confidential Ledger Launch"
SECRET_PATH="clients/confidential-ledger/launch.go"
SECRET_EMAIL="engineer@acme-corp.example"
SECRET_ORG="acme-inc"
SECRET_REPO="payments-core"

# ---------------------------------------------------------------- mock backend
MOCK_LOG="$WORK/mock.log"
MOCK_EMAIL="$SECRET_EMAIL" MOCK_REVOKE_FIRST_SYNC=1 \
MOCK_SECRETS="$SECRET_SUBJECT|$SECRET_PATH|$SECRET_EMAIL|$SECRET_ORG/$SECRET_REPO" \
  ${MOCK_BIN:?set MOCK_BIN} > "$MOCK_LOG" 2>&1 &
MOCK_PID=$!
for _ in $(seq 1 50); do grep -q "MOCK READY" "$MOCK_LOG" 2>/dev/null && break; sleep 0.1; done
API=$(grep "MOCK READY" "$MOCK_LOG" | awk '{print $3}')
if [ -z "$API" ]; then echo "mock backend did not start"; cat "$MOCK_LOG"; exit 1; fi
echo "mock backend: $API"

cleanup() { kill $MOCK_PID 2>/dev/null; rm -rf "$WORK"; }
trap cleanup EXIT

export PROOFBOARD_API_BASE_URL="$API"
export PROOFBOARD_AGENT_AUTH_URL="$API/cli-auth"
export PROOFBOARD_APP_BASE_URL="$API"
export NO_BROWSER=1
export PROOFBOARD_DISABLE_STARTUP_CHECKS=1
export PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS=1
export PROOFBOARD_DISABLE_KEYCHAIN=1

# Keep `install` fully inside the temporary account. A successful systemctl
# stub exercises service registration without starting a persistent process.
mkdir -p "$WORK/bin"
printf '#!/bin/sh\nexit 0\n' > "$WORK/bin/systemctl"
chmod 0700 "$WORK/bin/systemctl"
export PATH="$WORK/bin:$PATH"

echo ""
echo "=== 1. INSTALL (isolated current-account installation) ==="
INSTALL_OUT=$("$PB" install 2>&1)
echo "$INSTALL_OUT" | sed 's/^/     | /'
contains "compiled Career Agent installs successfully" "$INSTALL_OUT" "installed and started"
check "installed executable exists only in isolated home" \
  "$([ -x "$HOME/.local/bin/proofboard" ] && echo yes || echo no)" "yes"
check "isolated systemd service was registered" \
  "$([ -f "$HOME/.config/systemd/user/proofboard-career-agent.service" ] && echo yes || echo no)" "yes"

echo ""
echo "=== 2. COMPLETIONS (plural alias and generated output) ==="
COMPLETION_OUT=$("$PB" completions bash 2>&1)
contains "bash completion generated through completions alias" "$COMPLETION_OUT" "__start_proofboard"

echo ""
echo "=== 3. VERSION ==="
VERSION_OUT=$("$PB" version 2>&1)
echo "$VERSION_OUT" | sed 's/^/     | /'
contains "compiled version command reports a version" "$VERSION_OUT" "proofboard version "

# ------------------------------------------------------------------ a project
REPO="$WORK/$SECRET_REPO"
git init -q -b main "$REPO"
git -C "$REPO" config user.email "$SECRET_EMAIL"
git -C "$REPO" config user.name "Proofboard Engineer"
git -C "$REPO" remote add origin "git@github.com:$SECRET_ORG/$SECRET_REPO.git"
mkdir -p "$REPO/$(dirname "$SECRET_PATH")"
echo 'package confidentialledger' > "$REPO/$SECRET_PATH"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "$SECRET_SUBJECT"
HEAD1=$(git -C "$REPO" rev-parse HEAD)
git -C "$REPO" update-ref refs/remotes/origin/main "$HEAD1"
git -C "$REPO" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main

echo ""
echo "=== 4. AUTHENTICATE (device code -> poll -> approve) ==="
AUTH_OUT=$(cd "$REPO" && "$PB" auth 2>&1)
echo "$AUTH_OUT" | sed 's/^/     | /'
contains "device code shown to the engineer"        "$AUTH_OUT" "WXYZ-9876"
contains "authorization page offered"               "$AUTH_OUT" "cli-auth?code=WXYZ-9876"
contains "authentication confirmed"                 "$AUTH_OUT" "Authenticated as"
check    "credentials stored" "$([ -f "$HOME/.proofboard/credentials.json" ] && echo yes || echo no)" "yes"
check    "credentials are private (0600)" "$(stat -c '%a' "$HOME/.proofboard/credentials.json" 2>/dev/null)" "600"
check    "device key stored" "$([ -f "$HOME/.proofboard/device.key" ] && echo yes || echo no)" "yes"
check    "device key is private (0600)" "$(stat -c '%a' "$HOME/.proofboard/device.key" 2>/dev/null)" "600"
check    "device key id recorded" \
  "$(python3 -c "import json;print(json.load(open('$HOME/.proofboard/device.key')).get('deviceKeyId',''))")" "device-key-e2e-1"
PRIVFRAG=$(python3 -c "import json;print(json.load(open('$HOME/.proofboard/device.key'))['privateKey'][:24])")
check    "private key never leaves the machine" \
  "$(grep -c -- "$PRIVFRAG" "$MOCK_LOG" 2>/dev/null | head -1)" "0"

echo ""
echo "=== 5. CONNECT THE PROJECT ==="
LINK_OUT=$(cd "$REPO" && "$PB" link --non-interactive 2>&1)
echo "$LINK_OUT" | sed 's/^/     | /'
check "project recorded in state" \
  "$(python3 -c "import json;s=json.load(open('$HOME/.proofboard/state.json'));print(len(s.get('linkedRepos',{})))")" "1"
check "per-project email HMAC key stored" \
  "$(python3 -c "import json;s=json.load(open('$HOME/.proofboard/state.json'));print(len(next(iter(s['linkedRepos'].values())).get('emailHashKey','')))")" "64"

echo ""
echo "=== 6. FIRST SYNC (revoked key rotates without another login) ==="
SYNC_OUT=$(cd "$REPO" && "$PB" sync 2>&1)
echo "$SYNC_OUT" | sed 's/^/     | /'
REPORT=$(curl -sS "$API/__report")
python3 - "$REPORT" "$HEAD1" <<'PY'
import json, sys
report = json.loads(sys.argv[1]); head = sys.argv[2]
payloads = report["payloads"]
def check(label, got, want):
    print(f"  {'PASS' if got == want else 'FAIL'}  {label:<58} " + (str(got) if got == want else f"got={got} want={want}"))
    return got == want
ok = True
ok &= check("exactly one sync transmitted", len(payloads), 1)
if payloads:
    p = payloads[0]
    ok &= check("the commit sha was sent", p["shas"], [head])
    ok &= check("previousHead absent on a first sync", p.get("previousHead", ""), "")
    ok &= check("signedCommitRatio present", "signedCommitRatio" in p["antiFraudSignals"], True)
    ok &= check("rotated deviceKeyId attached", p["deviceKeyId"], "device-key-e2e-2")
    ok &= check("cliVersion reported", bool(p["cliVersion"]), True)
    blob = json.dumps(p)
    for label, field in {"commit messages": "subject", "file paths": "filePaths",
                         "repo name": "repoName", "diffs": "diff"}.items():
        ok &= check(f"no {label} in payload", field in blob, False)
    ok &= check("identity sent only as a sha256 hash", len(p.get("emailHash", "")), 64)
    ok &= check("organisation sent only as a sha256 hash", len(p.get("orgHash", "")), 64)
sys.exit(0 if ok else 1)
PY
[ $? -ne 0 ] && FAILED=1
check "revoked key did not launch a second login" \
  "$(grep -c '^CALL device-code$' "$MOCK_LOG")" "1"
check "revoked key was automatically re-registered" \
  "$(grep -c '^CALL device-key$' "$MOCK_LOG")" "2"
check "sync did not print an expired-session reconnect prompt" \
  "$(printf '%s' "$SYNC_OUT" | grep -c 'session has expired' || true)" "0"

echo ""
echo "=== 7. UPDATE DICTIONARY ==="
DICTIONARY_OUT=$("$PB" update-dictionary 2>&1)
echo "$DICTIONARY_OUT" | sed 's/^/     | /'
contains "dictionary command completed" "$DICTIONARY_OUT" "Dictionary is up to date"

echo ""
echo "=== 8. SECOND SYNC CARRIES previousHead ==="
echo 'package confidentialledger // extended' > "$REPO/$SECRET_PATH"
git -C "$REPO" add -A
git -C "$REPO" -c core.hooksPath=/dev/null commit -qm "fix: settle rounding in the confidential ledger"
HEAD2=$(git -C "$REPO" rev-parse HEAD)
git -C "$REPO" update-ref refs/remotes/origin/main "$HEAD2"
SYNC2_OUT=$(cd "$REPO" && "$PB" sync 2>&1)
echo "$SYNC2_OUT" | sed 's/^/     | /'
REPORT2=$(curl -sS "$API/__report")
python3 - "$REPORT2" "$HEAD1" <<'PY'
import json, sys
report = json.loads(sys.argv[1]); head1 = sys.argv[2]
payloads = report["payloads"]
def check(label, got, want):
    print(f"  {'PASS' if got == want else 'FAIL'}  {label:<58} " + (str(got) if got == want else f"got={got} want={want}"))
    return got == want
ok = True
ok &= check("a second sync was transmitted", len(payloads) >= 2, True)
if len(payloads) >= 2:
    ok &= check("previousHead is the previously synced head", payloads[1].get("previousHead"), head1)
sys.exit(0 if ok else 1)
PY
[ $? -ne 0 ] && FAILED=1

echo ""
echo "=== 9. SIGNATURE AND PRIVACY VERDICT FROM THE BACKEND ==="
grep -E "^(PASS|FAIL)" "$MOCK_LOG" | sed 's/^/  /'
SIGOK=$(grep -c "payload signature verified" "$MOCK_LOG")
check "every sync signature verified server-side" "$SIGOK" "2"
LEAKS=$(python3 -c "import json,sys;print(len(json.loads(sys.argv[1])['failures'] or []))" "$REPORT2")
check "no privacy or protocol failures" "$LEAKS" "0"

echo ""
echo "=== 10. BACKGROUND PROJECT DETECTION ==="
NEWREPO="$WORK/ledger-service"
git init -q -b main "$NEWREPO"
git -C "$NEWREPO" config user.email "$SECRET_EMAIL"
git -C "$NEWREPO" config user.name "Proofboard Engineer"
git -C "$NEWREPO" remote add origin "git@github.com:$SECRET_ORG/ledger-service.git"
echo hello > "$NEWREPO/a.txt"; git -C "$NEWREPO" add -A; git -C "$NEWREPO" commit -qm "feat: start"
for i in $(seq 1 10); do (cd "$NEWREPO" && "$PB" detect --editor vscode >/dev/null 2>&1); done
DETECT_JSON=$(cd "$NEWREPO" && "$PB" detect --json 2>/dev/null)
check "later opens report no action" \
  "$(printf '%s' "$DETECT_JSON" | python3 -c 'import json,sys;print(json.load(sys.stdin)["action"])')" "none"
check "prompt recorded once per project" \
  "$(python3 -c "import json;print(len(json.load(open('$HOME/.proofboard/state.json')).get('promptedWorkspaces',{})))")" "1"

echo ""
echo "=== 11. NEVER ASK AGAIN ==="
(cd "$NEWREPO" && "$PB" notify-activate "proofboard://notify-action?kind=suppress&workspace=$NEWREPO" >/dev/null 2>&1)
check "workspace suppressed" \
  "$(cd "$NEWREPO" && "$PB" detect --json 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin)["action"])')" "suppressed"

echo ""
echo "=== 12. BACKGROUND AGENT LIFECYCLE ==="
AGENT_STATUS=$("$PB" agent status 2>&1); echo "$AGENT_STATUS" | sed 's/^/     | /'
STATUS_OUT=$(cd "$REPO" && "$PB" status 2>&1); echo "$STATUS_OUT" | sed 's/^/     | /'
LOGS_ALL=$("$PB" logs 2>&1)
printf '%s\n' "$LOGS_ALL" | tail -3 | sed 's/^/     | /'
contains "sync recorded in the log" "$LOGS_ALL" "manual — start — success"
contains "detection recorded in the log" "$LOGS_ALL" "detect"

echo ""
echo "=== 13. CALL SEQUENCE OBSERVED BY THE BACKEND ==="
python3 -c "import json,sys;print('  ' + ' -> '.join(json.loads(sys.argv[1])['calls']))" "$REPORT2"

echo ""
if [ "$FAILED" -eq 0 ]; then echo "JOURNEY RESULT: ALL CHECKS PASSED"; else echo "JOURNEY RESULT: FAILURES PRESENT"; fi
exit "$FAILED"
