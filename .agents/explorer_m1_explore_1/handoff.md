# Handoff Report — Explorer Milestone 1

## 1. Observation
We observed the following code sections and behaviors:
* In `internal/commands/update.go`, the update command uses the latest version endpoint `releases.Latest(ctx, runtimeContext.config.LatestVersionPath)`. However, searching the entrypoint files `cmd/proofboard/main.go` and `internal/commands/root.go` reveals no startup checks targeting this endpoint.
* In `internal/dictionary/updater.go` (lines 5-8):
  ```go
  func Update(ctx context.Context) error {
  	return ctx.Err()
  }
  ```
  The function returns `ctx.Err()`, meaning it is a mock skeleton. No auto-update triggers exist during sync or on startup.
* In `internal/commands/status.go` (line 36):
  ```go
  fmt.Fprintf(out, "%s tier=%s lastSync=%s lastHead=%s\n", repoHash, repo.Tier, repo.LastSyncAt.Format("2006-01-02T15:04:05Z07:00"), repo.LastHeadSHA)
  ```
  The status output only displays database state attributes, lacking comparison with the actual repository HEAD to indicate a pending sync.
* In `internal/commands/sync.go` (lines 390-395):
  ```go
  if linkedThisTime {
  	_, err = fmt.Fprintln(out, "✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard")
  	if err != nil {
  		return err
  	}
  }
  ```
  The "Milestone captured" printout is gated strictly by `linkedThisTime`, which is only set when the repository is freshly linked on that run.
* In `internal/commands/sync.go` (lines 368-370):
  ```go
  repoState.Tier = "Tier2"
  } else {
  repoState.Tier = "Tier2-skipped"
  ```
  The tier representation continues using the old `"Tier2"` nomenclature instead of version 1.4's `"SHA Proof"`.
* In `SPEC.md` OpenAPI paths (lines 929-3300), no paths exist for `/cli/link` or `/cli/sync` or `/cli/...`. The client API default configuration defines:
  ```go
  v.SetDefault("api.link_path", "/cli/link")
  v.SetDefault("api.sync_path", "/cli/sync")
  ```

## 2. Logic Chain
1. Based on the observation of `cmd/proofboard/main.go` and `internal/commands/root.go`, we can deduce that the startup checks specified in `SPEC.md` Section 2.4 (non-blocking warning for new versions) are entirely missing.
2. Based on `internal/dictionary/updater.go` returning mock errors and the absence of updater calls, the dictionary auto-update mechanism is currently un-implemented.
3. Based on the output string in `internal/commands/status.go`, we deduce that "pending sync" status is not calculated or printed as part of `proofboard status`.
4. Based on `linkedThisTime` gating in `sync.go`, the terminal echo notice for successful milestone transmission is not surfaced on subsequent syncs.
5. Based on the default configurations in `internal/config/config.go` resolving to `/cli/link` and `/cli/sync`, and the lack of such paths in `SPEC.md` OpenAPI spec, there is a discrepancy in the backend specification.

## 3. Caveats
- Since the environment is constrained by CODE_ONLY network mode, we were unable to test integration with the live backend at `https://api-dev.proofboard.io` or clone the `proofboard-backend` repository.
- We assumed that Phase 2 IDE watcher commands are not within scope for the initial launch cohort build as they are explicitly labeled Phase 2 in `SPEC.md`.

## 4. Conclusion
There are five main implementation gaps in the Proofboard CLI Go codebase compared to the `SPEC.md` version 1.4, primarily around startup version checks, auto-update dictionary triggers, terminal display naming/status logic, and OpenAPI spec documentation mismatches.

## 5. Verification Method
1. Run `go test ./...` to verify all baseline unit tests pass.
2. Check `internal/commands/status.go:36` to verify it prints `repo.Tier` and only `lastSync` / `lastHead`, confirming "pending sync" is not printed.
3. Check `internal/commands/sync.go:390` to confirm that the `Milestone captured` notification is gated by `linkedThisTime`.
