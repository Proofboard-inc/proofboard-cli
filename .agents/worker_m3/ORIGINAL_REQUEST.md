## 2026-06-16T18:04:23Z
Identity: teamwork_preview_worker
Working Directory: /workspaces/proofboard-cli/.agents/worker_m3

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Your objective is to implement Milestone 3 (CLI Subcommands & Prompts):
1. Watched Branches Configuration:
   - Modify `internal/state/state.go` so `Default()` sets `WatchedBranches` to `[]string{"main", "master", "develop"}`.
   - Implement these subcommands under `proofboard config`:
     - `add-branch {name}`: Adds a branch to `WatchedBranches` in `state.json` (ensure no duplicates, keep list clean).
     - `remove-branch {name}`: Removes a branch from `WatchedBranches` in `state.json`.
     - `branches`: Prints the currently watched branches, one per line.
   - Update `internal/commands/sync.go` so that if `fromHook` is true, the current branch is verified against the global `WatchedBranches` from `state.json` (instead of `repoState.ProductionBranches`), exiting silently if not matched.

2. Unlinked Workspace Prompt & Suppression:
   - Add `SuppressedWorkspaces []string json:"suppressedWorkspaces"` to the `model.State` struct in `internal/model/state.go`.
   - Update `internal/commands/sync.go` so that if a repository is not linked:
     - Check if the absolute path of the repository is listed in `current.SuppressedWorkspaces`. If yes, exit silently.
     - If not, perform a quick classification scan (Phase 1 Ingest, Phase 2 Classify, Phase 3 Score) on the commits in the workspace.
     - Extract up to the top 3 Contribution Areas (categories with score > 0).
     - Output the prompt verbatim to stdout:
       ```
       Proofboard — unlinked repository detected.

       Project: <repo-name> (use the base directory name of the repository)
       Detected:
       ✓ <Category 1>
       ✓ <Category 2>
       ...
       Add this project to your proofboard?

         y   Sync this project
         n   Not this project
         x   Never ask for this workspace
       ```
     - Read user response (y/n/x) from stdin (use `fmt.Scanf` or a reader).
     - If 'y': Link the repository (like `proofboard link` command) and run `sync` (like `proofboard sync` command), printing the Proof-of-Ship echo `✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard` on success.
     - If 'n': Exit.
     - If 'x': Add the repository path to `SuppressedWorkspaces` in `state.json`, save, and exit.

3. Monthly Career Summary Terminal Trigger:
   - Add `MonthlyCareerSummaryShown map[string]bool json:"monthlyCareerSummaryShown"` to the `model.State` struct in `internal/model/state.go`. Ensure it is initialized when loading state if it is nil.
   - During sync or status execution, check if the monthly career summary is ready.
   - The summary is generated on the last Friday of each month. Compute if the last Friday of the current month has passed. If yes, the current month's summary is ready (key format: "YYYY-MM"). If not, the previous month's summary is ready.
   - If the summary for that month is ready and `!current.MonthlyCareerSummaryShown[key]`, print the quiet line verbatim:
     `Proofboard: Your <MonthName> career summary is ready. proofboard.io/career-summary`
     Update the map `current.MonthlyCareerSummaryShown[key] = true` and save `state.json`.

4. Verification:
   - Write comprehensive unit tests for config branch operations, workspace suppression list checks, the last Friday calculation, and the career summary notification trigger.
   - Run `go test ./...` and `go vet ./...` to ensure clean execution.

5. Deliverables:
   - Log changes in `/workspaces/proofboard-cli/.agents/worker_m3/changes.md`.
   - Write a handoff report at `/workspaces/proofboard-cli/.agents/worker_m3/handoff.md` and notify the parent orchestrator (Conversation ID: 6a501e6d-c16f-44d2-b47d-63b5c2112fc2).
