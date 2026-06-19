# Compliance Challenge Report — M2

## Challenge Summary

**Overall risk assessment**: LOW

All tested compliance checks met the requirements of SPEC.md. Our empirical tests confirmed that:
1. **Startup checks** run under the 2-second timeout, do not block CLI execution if the update server is offline or extremely slow, and successfully auto-update the dictionary when appropriate.
2. **Status pending flags** correctly transition between `pending=yes`, `pending=no`, and `pending=unknown` depending on the state of the local git repository and matching HEAD.
3. **Tier display naming** maps internal names such as `Tier2` and `Tier2-skipped` to `SHA Proof` and `SHA Proof — handshake skipped` respectively.

---

## Challenges

### [Low] Challenge 1: Sequential Blocking of Startup Checks

- **Assumption challenged**: Running multiple update checks sequentially under a single 2-second context is adequate.
- **Attack scenario**: If the first network check (CLI version) takes 1.95 seconds to time out or respond due to a slow network, only 0.05 seconds remains for the dictionary update check. The second check will almost certainly time out/fail, preventing dictionary updates even if the dictionary server is reachable and fast.
- **Blast radius**: The dictionary auto-update fails to run under degraded network conditions that slowly resolve.
- **Mitigation**: Perform update checks concurrently using goroutines, or run the entire startup check suite asynchronously so it never blocks command execution at all.

### [Low] Challenge 2: Disk Littering on Sudden Termination

- **Assumption challenged**: Stale temporary files (`dictionary.json.tmp`) will always be removed on completion.
- **Attack scenario**: If the CLI process is terminated abruptly (e.g. `SIGKILL` or power failure) during dictionary download, the `dictionary.json.tmp` file will persist indefinitely.
- **Blast radius**: Stale `.tmp` files may remain on the disk. However, since the file is opened with `os.O_TRUNC` on the next run, it does not leak memory/disk unboundedly.
- **Mitigation**: Clean up any pre-existing `.tmp` files in `~/.proofboard` upon CLI startup.

---

## Stress Test Results

| Scenario | Expected Behavior | Actual Behavior | Pass/Fail |
|---|---|---|---|
| **Slow Network Check** | Startup check terminates in < 2 seconds, command proceeds without error. | Terminated in 2.00s, state remained intact, command exited cleanly. | **PASS** |
| **Offline Network Check** | Startup check terminates immediately with error handled, command proceeds. | Terminated in 0.00s, command executed without blocking or error. | **PASS** |
| **Invalid Dict Schema** | Downloaded dictionary with empty version fails validation, local version unchanged. | Rejected by `dictionary.Validate`, temporary file deleted, local version remained `1.0.0`. | **PASS** |
| **Status Pending Matching HEAD** | Current git repository matches `LastHeadSHA` in state. | Outputs `pending=no` and tier maps to `SHA Proof`. | **PASS** |
| **Status Pending Differing HEAD** | Current git repository differs from `LastHeadSHA` in state. | Outputs `pending=yes` and tier maps to `SHA Proof`. | **PASS** |
| **Status Pending Not in Git** | Directory is not a git repository. | Outputs `pending=unknown` for all linked repositories. | **PASS** |
| **Status Pending Other Repo** | Current repository is not the one linked (or a different git repository). | Outputs `pending=unknown` for the unmatching repositories. | **PASS** |
| **Tier Naming Display Mapping** | Displays mapped names for `Tier2` and `Tier2-skipped`. | Mapped `Tier2` to `SHA Proof` and `Tier2-skipped` to `SHA Proof — handshake skipped`. | **PASS** |

---

## Unchallenged Areas

- **System-level file permission locks**: Did not test situations where `~/.proofboard/dictionary.json` is write-locked or owned by `root`, which might cause download/rename commands to fail. This is considered out of scope for general command compliance.
