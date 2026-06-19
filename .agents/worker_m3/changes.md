# Changes Log — Milestone 3

## Watched Branches Configuration
- Modified `internal/state/state.go`:
  - Updated `Default()` to set default `WatchedBranches` to `[]string{"main", "master", "develop"}`.
  - Updated `Load()` to initialize `MonthlyCareerSummaryShown` and `SuppressedWorkspaces` if they are nil.
- Modified `internal/model/state.go`:
  - Added `SuppressedWorkspaces` and `MonthlyCareerSummaryShown` to `State` struct.
- Modified `internal/commands/config.go`:
  - Added Cobra subcommands: `add-branch`, `remove-branch`, and `branches` under the `config` command.
- Modified `internal/commands/sync.go`:
  - If `fromHook` is true, check current branch against global `WatchedBranches` in `state.json` instead of repo-specific `ProductionBranches`.

## Unlinked Workspace Prompt & Suppression
- Modified `internal/commands/sync.go`:
  - Added logic when sync is run in an unlinked workspace:
    - If repository absolute path is in `current.SuppressedWorkspaces`, exit silently.
    - Otherwise, perform a quick classification scan (Phase 1 Ingest, Phase 2 Classify, Phase 3 Score) to extract top 3 contribution areas.
    - Output prompt to stdout.
    - Read input (y/n/x) from stdin.
    - If 'y': Link repository, perform sync, and output Proof-of-Ship echo.
    - If 'n': Exit cleanly.
    - If 'x': Add repository path to `SuppressedWorkspaces`, save state, and exit.

## Monthly Career Summary Terminal Trigger
- Modified `internal/commands/runtime.go`:
  - Added helper functions `triggerMonthlyCareerSummary`, `triggerMonthlyCareerSummaryWithTime`, `getReadyCareerSummaryMonth`, and `lastFridayOfMonth` to calculate if the last Friday has passed and display notification.
- Modified `internal/commands/status.go` and `internal/commands/sync.go`:
  - Added calls to check and trigger the career summary notification before exiting.

## Verification
- Added `internal/commands/milestone3_test.go` containing unit tests covering:
  - Config branch subcommands (`add-branch`, `remove-branch`, `branches`).
  - Monthly career summary ready month key calculation (last Friday of month calculation).
  - Monthly career summary notification triggering.
  - Workspace suppression check.
  - Never ask workspace prompt option.
