# Changes Summary - 2026-06-17

This document outlines the v1.4 spec compliance changes implemented in the codebase.

## 1. Startup Update Checks
- Added `PersistentPreRunE` to the root command in `internal/commands/root.go`.
- Implemented `runStartupUpdateChecks` to run non-blocking CLI version checks against `/latest.json` on the release server and dictionary version checks against `/dictionary/latest.json`.
- Excluded `update`, `update-dictionary`, and `help` commands (as well as parentless/root command calls) from the startup checks.
- Bound all startup network and download operations within a 2-second timeout context.
- Implemented auto-updating of the dictionary if `AutoUpdateDictionary` configuration in state is true and a newer dictionary version exists. Prints `"Dictionary updated successfully to version %s.\n"` on success.
- Handled all network connection errors and timeouts gracefully without blocking or failing command execution.

## 2. Proof-of-Ship Notification Echo
- Modified `internal/commands/sync.go` to print the milestone capture message (`✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard`) on every successful sync that transmits one or more commits (`len(payload.SHAs) > 0`).

## 3. Tier Naming Display & Mapping
- Updated `internal/commands/sync.go` to assign `"SHA Proof"` instead of `"Tier2"`, and `"SHA Proof — handshake skipped"` instead of `"Tier2-skipped"` to the repository tier in local state.
- Implemented `mapTierName` mapping function in `internal/commands/sync.go` to convert server/old state tier representations like `"Tier2"` and `"Tier2-skipped"` to `"SHA Proof"` and `"SHA Proof — handshake skipped"`.
- Applied mapping to the printed status and sync results.

## 4. Status Pending Sync Check
- Updated `internal/commands/status.go` to run repository discovery and parse the current directory repository.
- If the current directory is a linked git repository, compares local `HEAD` SHA against `LastHeadSHA` in state.
- Outputs the pending flag: `pending=yes` if local HEAD differs, `pending=no` if it matches, and `pending=unknown` otherwise.
- Updated status output format to: `repoHash tier={tier} lastSync={lastSync} lastHead={lastHead} pending={pending}`.

## 5. Unit Tests
- Added `internal/commands/compliance_test.go` with comprehensive unit tests for `mapTierName`, status command with pending status flags, and `runStartupUpdateChecks` verifying auto-updates and network failure tolerance.
