## Challenge Summary

**Overall risk assessment**: LOW

The compliance changes implemented in the Go codebase for Startup Checks, Status Pending Flags, and Tier Display Naming are robust and function correctly.

## Challenges

### [Low] Challenge 1: Clock skew or goroutine scheduling delays in timeout context

- **Assumption challenged**: The startup checks always complete in under 2 seconds.
- **Attack scenario**: Under heavy system load, goroutine context switching or scheduling latency could cause the elapsed time between start and end of `runStartupUpdateChecks` to slightly exceed the 2-second threshold, even if the underlying HTTP request context correctly timeouts at exactly 2 seconds.
- **Blast radius**: Minimal. The CLI commands will still run as soon as the timeout finishes, and the user experiences a slight delay (2.0002s instead of exactly 2.0s).
- **Mitigation**: The current `context.WithTimeout(ctx, 2*time.Second)` is Go's standard mechanism and is appropriate. To prevent scheduling delays under absolute worst-case resource starvation, we could run the startup checks asynchronously in a background goroutine and proceed immediately, though this is currently not necessary as the blocking behavior under 2 seconds is acceptable.

### [Low] Challenge 2: Network failure during dict download leaves temporary file

- **Assumption challenged**: The update clean-up always succeeds.
- **Attack scenario**: If a partial download succeeds but writing/renaming fails, or if a panic occurs, a `.tmp` file might be left behind.
- **Blast radius**: The temporary file `dictionary.json.tmp` is created in `~/.proofboard/`. If the download fails, `os.Remove(tempPath)` is called. If the CLI crashes mid-write, a `.tmp` file may persist.
- **Mitigation**: The code correctly uses `defer os.Remove(tempPath)` or explicitly calls it at the end of the block, which is robust under normal error flows.

---

## Stress Test Results

- **Startup checks timeout test** (`TestStartupChecksTimeout`) → Startup checks must exit in < 3s when endpoint takes 5s → Exited in 2.000247796s, return value is nil, command does not fail → **PASS**
- **Startup checks network failure test** (`TestStartupChecksNetworkFailure`) → Startup checks must return nil and not block execution when endpoint host is invalid → Handled network failure gracefully, returned nil → **PASS**
- **Status pending yes test** (`TestStatusPendingStates`) → Git repository HEAD does not match `LastHeadSHA` → Output contains `pending=yes` → **PASS**
- **Status pending no test** (`TestStatusPendingStates`) → Git repository HEAD matches `LastHeadSHA` → Output contains `pending=no` → **PASS**
- **Status pending unknown test** (`TestStatusPendingStates`) → Not in a git repository or check error occurred → Output contains `pending=unknown` → **PASS**
- **Tier display naming maps Tier2** (`TestStatusPendingStates` & `TestMapTierName`) → Tier `Tier2` must be displayed as `SHA Proof` → `mapTierName("Tier2")` outputs `SHA Proof` → **PASS**
- **Tier display naming maps Tier2-skipped** (`TestStatusPendingStates` & `TestMapTierName`) → Tier `Tier2-skipped` must be displayed as `SHA Proof — handshake skipped` → `mapTierName("Tier2-skipped")` outputs `SHA Proof — handshake skipped` → **PASS**

---

## Unchallenged Areas

- **Binary distribution updates** — Actual downloading and self-replacing of the binary via `proofboard update` has not been tested in live production environments, as we are focusing on verifying startup compliance checks.
- **Other tiers** — Non-Tier2 configurations (e.g. `Tier1`) are output as-is, which is the default behavior of `mapTierName`.
