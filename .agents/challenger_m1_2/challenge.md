# Challenge Report — proofboard CLI

This document details the adversarial review and stress testing results performed on the compiled `proofboard-linux-amd64` binary.

## Challenge Summary

**Overall risk assessment**: LOW

The proofboard CLI binary conforms to standard execution parameters, does not panic or crash under stress-testing conditions, and handles offline or network-isolated states gracefully. NDA compliance is strictly maintained across memory management, state saving, and logging.

---

## Stress Test Results

A test suite containing various permutations of subcommands and environment conditions was executed against the statically linked `dist/proofboard-linux-amd64` binary:

| Subcommand / Scenario | Input / Environment | Expected Behavior | Actual Behavior | Result |
|---|---|---|---|---|
| `help` | Isolated HOME, no network | Print help, exit 0. No network connection | Printed help, exit 0, no network | PASS |
| `--help` / `-h` | Isolated HOME, no network | Print help, exit 0 | Printed help, exit 0 | PASS |
| `status` | Isolated HOME, unlinked | Print "No linked repositories", exit 0 | Printed "No linked repositories", exit 0 | PASS |
| `config branches` | Isolated HOME | Print default watched branches | Printed main, master, develop | PASS |
| `config add-branch` | Isolated HOME, `test-branch` | Add to state.json | Added to state.json successfully | PASS |
| `config remove-branch` | Isolated HOME, `test-branch` | Remove from state.json | Removed from state.json successfully | PASS |
| `config set auto-update-dictionary` | Isolated HOME, `false` | Disable auto-update dictionary | Modified state.json successfully | PASS |
| `logs` | Isolated HOME | Print log output from sync.log | Printed sync logs successfully | PASS |
| `sync` | Isolated HOME, unlinked repo | Run locally, print unlinked notice | Printed unlinked notice, exit 0 | PASS |
| `update-dictionary` | Isolated HOME, offline | Gracefully fail network with status 1 | Exited 1 with connection refused err | PASS |
| `update` | Isolated HOME, offline | Gracefully fail network with status 1 | Exited 1 with connection refused err | PASS |
| First-run setup | Isolated HOME, non-interactive (EOF) | Proceed to command help without blocking | Completed setup, printed help, exit 0 | PASS |
| `install` / `uninstall` | Isolated HOME, dry-run no-root | Gracefully exit 1 on write permission failure | Exited 1 without panicking/crashing | PASS |

---

## NDA Verification Findings

### 1. In-Memory Safety & Shredder (Phase 5)
In `internal/pipeline/phase5/shredder.go`, the `Shred` function is executed in Phase 5, which resides *before* payload assembly (Phase 7) and transmission (Phase 8).
The shredder:
- Mutates original commit arrays: zeroing bytes of `Subject` (`crypto.ZeroBytes`), setting to `nil`.
- Blanks out strings for `FilePaths` (`crypto.DropStrings`), `AuthorEmail`, `Repository`, and `Organization`.
- Extracts only anonymized fields (`SHA`, `TimestampUnix`, `Additions`, `Deletions`, `FilesChanged`, `Category`, `ImpactType`, `NoiseScore`, `AuthorEmailHash`, `SignatureValid`) into the safe array.

This ensures zero residual proprietary string data exists in memory when Phase 8 transmits the payload to the API server.

### 2. State & Log Leak Check
- **Credentials** (~/.proofboard/credentials.json) stores only `token`, `username`, `refreshToken`, and `emailHash`. No plain emails, cleartext passwords, or repo URLs. Permissions are properly restricted to `0600`.
- **State** (~/.proofboard/state.json) stores metadata such as `linkedRepos` map (indexed by hashed repo identifier), `watchedBranches`, `autoUpdateDictionary` flag, etc. Repo-specific info uses `orgHash` and `repoHash`. Cleartext repo names or org names are never saved.
- **Sync log** (~/.proofboard/sync.log) uses format `[timestamp] — [repoHash] — [trigger] — [phase] — [status] — [message]`. No sensitive data or filenames appear in logs.

---

## Network Isolation Findings

### 1. Version and Dictionary Checks on Startup
- Any subcommand running under `proofboard` that is *not* `help`, `update`, or `update-dictionary` will execute `runStartupUpdateChecks`.
- This fires two GET requests:
  1. `GET /latest.json` to `PROOFBOARD_RELEASE_BASE_URL` (checks for CLI updates).
  2. `GET /api/v1/cli/dictionary` to `PROOFBOARD_API_BASE_URL` (checks for dictionary updates).
- **Graceful degradation**: These startup check calls run under a strict 2-second timeout context. If they fail due to DNS, VPN block, or firewall isolation, the error is swallowed and execution proceeds immediately to the local handler without blocking.
- **Opt-out dictionary check**: When `auto-update-dictionary` is set to `false` in config, the `GET /api/v1/cli/dictionary` request is completely skipped, and only the version check is executed.

---

## Challenges

### [Low] Challenge 1: Version Update Check is Always Triggered
- **Assumption challenged**: Standard local commands (like `status`, `config`) run completely offline and make zero network connections.
- **Attack/Failure scenario**: In high-security isolated air-gapped systems, any outbound DNS/HTTP request (even non-blocking ones) can trigger alarms.
- **Blast radius**: No crash occurs, but an unintended network connection is attempted to `releases.proofboard.io` on every run of `status` or `config`.
- **Mitigation**: Introduce a config flag or environment variable (e.g. `PROOFBOARD_NO_UPDATE_CHECK=1`) to allow engineers in strict air-gapped environments to completely skip version checks during startup.

---

## Unchallenged Areas

- **Windows, macOS binaries**: The stress test and network isolation verification was performed only on the `linux-amd64` platform due to local workspace constraints.
