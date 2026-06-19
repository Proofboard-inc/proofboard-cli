# Compliance and SPEC v1.4 Review Report

## Quality Review Summary

**Verdict**: APPROVE

All reviewed features strictly comply with the requirements of `SPEC.md` v1.4 and satisfy Go 1.21+ coding guidelines. The changes are fully verified, robust, and correctly covered by tests.

---

## Findings

No critical or major findings were discovered during this review. 

### Minor Observation 1
- **What**: The HTTP client used in `ReleaseClient` (in `internal/api/update.go`) is initialized as `&http.Client{}`.
- **Where**: `internal/api/update.go:24`
- **Why**: An empty `http.Client` uses `http.DefaultTransport`, which lacks explicit dial/keep-alive timeouts. However, since the requests are wrapped in a 2-second timeout context (`checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)`), they will be cancelled properly.
- **Suggestion**: In a future refactoring, it might be beneficial to explicitly configure dial timeouts on the transport level for completeness.

---

## Verified Claims

- **Version Constant `"1.4.0"`**
  - *Verification Method*: Inspected `internal/version/version.go`. Checked `Version` is declared as a constant `"1.4.0"`.
  - *Result*: PASS.

- **Non-blocking Startup update checks**
  - *Verification Method*: Inspected `runStartupUpdateChecks` in `internal/commands/root.go`. Verified context timeout is set to 2 seconds and errors are ignored (returns `nil`). Tested with `TestStartupUpdateChecks` in `internal/commands/compliance_test.go`.
  - *Result*: PASS.

- **Milestone Print Gating**
  - *Verification Method*: Inspected `internal/commands/sync.go` lines 389–394. Verified only `✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard` is printed on successful sync, and other milestone highlights are held in-app as per SPEC section 5A.2.
  - *Result*: PASS.

- **Tier Name Display Mapping**
  - *Verification Method*: Inspected `mapTierName` function and verified mapped names `SHA Proof` and `SHA Proof — handshake skipped`. Verified unit tests in `TestMapTierName` in `internal/commands/compliance_test.go`.
  - *Result*: PASS.

- **Pending Status Check in `status`**
  - *Verification Method*: Checked `internal/commands/status.go` logic comparing `localHeadSHA` vs `repoState.LastHeadSHA`. Tested under different repository states (matching HEAD, differing HEAD, non-git directory) using `TestStatusPendingCheck` in `compliance_test.go`.
  - *Result*: PASS.

---

## Coverage Gaps
None. All listed compliance areas are fully tested by the test suite under `/workspaces/proofboard-cli/internal/commands/`.

---

## Unverified Items
None. All features have been verified via manual code inspection, static analysis (`go vet`), and automated tests (`go test`).

---
---

## Adversarial Review Summary

**Overall risk assessment**: LOW

The compliance changes are highly robust, leveraging atomic file renames, strict timeout boundaries, and defensive checks against nil states and invalid configurations.

---

## Challenges

### [Low] Challenge 1: CDN Response Corruption during Dictionary Update
- **Assumption challenged**: The CDN responds with valid/invalid JSON structures, which are successfully validated.
- **Attack scenario**: The CDN returns a response that has a valid JSON format but represents an empty dictionary or deletes existing mapping categories.
- **Blast radius**: If the dictionary is successfully validated but contains empty rules, future local git ingests will result in zero scores.
- **Mitigation**: `dictionary.Validate` checks for mandatory categories and rules. In case of failure, it does not overwrite the active dictionary file. If the downloaded dictionary is corrupted but validates, it still doesn't block local runs entirely, and users can opt out of auto-updates using `proofboard config set auto-update-dictionary false`.

### [Low] Challenge 2: Command execution in non-Git contexts
- **Assumption challenged**: Executing `status` or `sync` in environments where `git` is absent or the workspace is missing.
- **Attack scenario**: A user runs the CLI in an environment where the `git` executable is missing from `$PATH`.
- **Blast radius**: The `status` command could fail to run `git discover` and raise errors.
- **Mitigation**: In `status.go`, the command intercepts the error from discovery and defaults `pending` status gracefully to `"unknown"`, printing all registered repository states correctly rather than crashing.

---

## Stress Test Results

- **Run sync command without network connection** -> Verify LSRemoteHandshake handles network failure within timeout -> Handshake times out (10s) and fails, correctly prompting the user to retry with `--skip-handshake` -> PASS.
- **Auto-Update dictionary JSON syntax error** -> Verify error is recovered and active dictionary remains untouched -> Handled by `TestUpdateDictionaryCommand_SchemaCheckFailure` -> PASS.
