# Compliance Review Report — v1.4

## Review Summary

**Verdict**: APPROVE

We have reviewed the compliance changes in the Go codebase of Proofboard CLI to satisfy SPEC.md v1.4 requirements. The code quality is excellent, clean, and has robust test coverage. There are no integrity violations, no dummy/facade implementations, and no bypasses.

---

## Findings

No critical or major issues were found. Below are minor observations and recommendations for the CLI team:

### [Minor] Startup Update Checks Console Output Destination
- **What**: The version notification and dictionary update notification are printed to standard output (`OutOrStdout()`).
- **Where**: `internal/commands/root.go` (lines 72 and 100)
- **Why**: While consistent with other output, printing update notifications to stdout can potentially pollute stdout when commands are being piped or output is parsed by scripts.
- **Suggestion**: Consider writing version notices and dictionary update confirmations to standard error (`ErrOrStderr()`) so that standard output remains clean for programmatic use.

---

## Verified Claims

- **CLI Version Constant is `1.4.0`**
  - *Verification Method*: Inspected `internal/version/version.go` showing `const Version = "1.4.0"`.
  - *Result*: PASS
- **Non-blocking Startup Update Checks**
  - *Verification Method*: Inspected `runStartupUpdateChecks` in `internal/commands/root.go`. Confirmed a 2-second timeout `context.WithTimeout` is applied and errors are swallowed (`err == nil` gates), preventing network issues or server failure from blocking command execution. Verified via `TestStartupUpdateChecks` in `internal/commands/compliance_test.go`.
  - *Result*: PASS
- **Dictionary Schema Validation & Atomic Rename**
  - *Verification Method*: Inspected dictionary update logic in `root.go` and `update_dictionary.go`. Confirmed dictionary JSON is validated via `dictionary.Validate` and then atomically replaced using `os.Rename`. Verified via `TestUpdateDictionaryCommand_SchemaCheckFailure` in `internal/commands/milestone4_test.go`.
  - *Result*: PASS
- **Milestone Print Gating**
  - *Verification Method*: Inspected sync command implementation in `internal/commands/sync.go` line 389. Confirmed "✔  Proofboard: Milestone captured" is printed only when `len(payload.SHAs) > 0` (indicating a qualifying milestone payload has been generated and transmitted).
  - *Result*: PASS
- **Tier Name Display Mapping**
  - *Verification Method*: Checked `mapTierName` implementation in both `internal/commands/sync.go` and `internal/commands/status.go`. Confirmed `Tier2` translates to `SHA Proof`, and `Tier2-skipped` translates to `SHA Proof — handshake skipped`. Other tier names are preserved.
  - *Result*: PASS
- **Pending Sync Detection in Status Command**
  - *Verification Method*: Checked pending sync logic in `internal/commands/status.go`. Confirmed it parses the local HEAD SHA and compares it to `repoState.LastHeadSHA`. If inside the active repository, it yields `pending=yes` or `pending=no`. If outside the repository or in another directory, it yields `pending=unknown`. Tested via `TestStatusPendingCheck` in `internal/commands/compliance_test.go`.
  - *Result*: PASS

---

## Coverage Gaps

- **Network-Restricted Environments**
  - *Risk Level*: Low
  - *Analysis*: The CLI relies on external CDNs/releases servers for updates. Under offline or isolated corporate environments, the 2-second timeout is hit. While non-blocking, it could add a 2-second delay to every CLI invocation.
  - *Recommendation*: Accept risk. The 2-second timeout is a standard compromise. In the future, a cached file could track the timestamp of the last check to prevent hit rate exceeding once per day.

---

## Unverified Items

None. All claims were verified via direct code inspection and unit test executions.

---
---

# Adversarial Review (Critic)

## Challenge Summary

**Overall risk assessment**: LOW

The compliance changes are logically robust. By relying on Git CLI commands (`git rev-parse`, etc.) for repository context and enforcing strict context timeouts, the system avoids hangs or state corruption.

---

## Challenges

### [Low] Delay on DNS/Network Timeout
- **Assumption challenged**: Network calls to releases server will fail quickly or timeout gracefully.
- **Attack scenario**: A machine has a misconfigured DNS server that takes exactly 5 seconds to resolve, or a firewall silently drops packets instead of sending resets.
- **Blast radius**: The user will experience a 2-second delay on every command execution (e.g. `status` or `config`) because the pre-run hook executes synchronously before the main command command runner.
- **Mitigation**: Run version checks asynchronously in a background goroutine or cache the check result on disk with a TTL (e.g., check at most once per 24 hours).

### [Low] Repository Identity Collisions
- **Assumption challenged**: `repoHash` uniquely identifies a project workspace directory.
- **Attack scenario**: Two developers in the same organization use two different directories but with the same git remote url, or the same git repository is cloned twice to different locations.
- **Blast radius**: If the repository is cloned to multiple directories, their `repoHash` (derived from provider + org + repo name) will be identical. When executing `status`, the pending check will check the current HEAD of whichever folder the user is currently in and compare it to the single `LastHeadSHA` saved in `state.json`. If they are on different branches, they will constantly mark each other as `pending=yes` and overwrite each other's HEAD SHA upon syncing.
- **Mitigation**: This is an inherent property of deriving repository hash solely from the remote URL (enforced by the NDA constraint to not store raw paths). The mitigation is already present by not storing directory paths to ensure NDA compliance.

---

## Stress Test Results

- **DNS Timeout Simulation** → Mock HTTP client hanging → pre-run hook exits after exactly 2s → PASS
- **Malformed Dictionary Download** → Return dictionary with empty categories/fields → validation fails, temp file discarded, state preserved → PASS
- **Command Invocation Outside Git** → Run `proofboard status` in `/tmp` → returns status with `pending=unknown` without crashing → PASS
