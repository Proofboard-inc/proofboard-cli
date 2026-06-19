# Proofboard CLI Technical Compliance Analysis

This report documents the detailed investigation and compliance audit of the Proofboard CLI repository against `SPEC.md` (v1.4), `README.md`, and `GEMINI.md`.

---

## 1. Build and Test Verification

Both compilation and tests were executed locally. All commands completed successfully without warnings, failures, or errors.

### Command Execution Details

1. **Static Analysis (`go vet ./...`)**
   - Command: `go vet ./...`
   - Output: Success (No errors or output)

2. **Compilation Build**
   - Command: `go build -o proofboard ./cmd/proofboard`
   - Output: Success (Binary compiled successfully)

3. **Test Suite execution (Uncached)**
   - Command: `go test -count=1 ./...`
   - Output:
     ```
     ?       github.com/proofboard/proofboard/cmd/proofboard [no test files]
     ok      github.com/proofboard/proofboard/internal/api   0.010s
     ?       github.com/proofboard/proofboard/internal/auth  [no test files]
     ok      github.com/proofboard/proofboard/internal/commands      0.005s
     ?       github.com/proofboard/proofboard/internal/config        [no test files]
     ok      github.com/proofboard/proofboard/internal/crypto        0.003s
     ok      github.com/proofboard/proofboard/internal/dictionary    0.006s
     ok      github.com/proofboard/proofboard/internal/git   0.002s
     ?       github.com/proofboard/proofboard/internal/hooks [no test files]
     ?       github.com/proofboard/proofboard/internal/logging       [no test files]
     ?       github.com/proofboard/proofboard/internal/model [no test files]
     ?       github.com/proofboard/proofboard/internal/notifications  [no test files]
     ok      github.com/proofboard/proofboard/internal/pipeline      0.005s
     ?       github.com/proofboard/proofboard/internal/pipeline/phase1       [no test files]
     ?       github.com/proofboard/proofboard/internal/pipeline/phase2       [no test files]
     ?       github.com/proofboard/proofboard/internal/pipeline/phase3       [no test files]
     ?       github.com/proofboard/proofboard/internal/pipeline/phase4       [no test files]
     ok      github.com/proofboard/proofboard/internal/pipeline/phase5       0.003s
     ?       github.com/proofboard/proofboard/internal/pipeline/phase6       [no test files]
     ?       github.com/proofboard/proofboard/internal/pipeline/phase7       [no test files]
     ?       github.com/proofboard/proofboard/internal/pipeline/phase8       [no test files]
     ok      github.com/proofboard/proofboard/internal/state 0.004s
     ?       github.com/proofboard/proofboard/internal/version       [no test files]
     ```

---

## 2. Command Set Compliance Review

The CLI implements the required basic command structure using Cobra. However, multiple subcommands, flags, and critical behaviors are either unimplemented or stubbed.

| Command | Status | Specification Requirement | Current Implementation / Gaps |
| :--- | :--- | :--- | :--- |
| `proofboard auth` | **Fully Compliant** | Opens default browser, receives JWT via callback on port 9876, saves credentials. | Functional; successfully integrates browser authentication callback. |
| `proofboard link` | **Fully Compliant** | Links repository, installs git hooks, registers repo hash with API. | Functional; correctly hashes and links the repository. |
| `proofboard unlink` | **Fully Compliant** | Removes hooks and clears repository from `state.json`. | Functional; cleanly deletes post-merge and post-rewrite hooks. |
| `proofboard sync` | **Partially Compliant** | Runs 8-phase pipeline and transmits payload. | Functional, but **Phase 4** is flawed, **Phase 7A** is missing, and the **Trivial Commit Filter** is completely omitted. |
| `proofboard status` | **Fully Compliant** | Prints state of linked repositories. | Functional; parses `state.json` and outputs repository details. |
| `proofboard logs` | **Non-Compliant** | Prints last 100 lines of `~/.proofboard/sync.log`. | **Broken**: Attempts to open `~/.proofboard/daemon.log`. In addition, no logging occurs (file writing is not implemented). |
| `proofboard update` | **Stub Only** | Downloads and replaces the binary. | **Incomplete**: Only prints version check output and download URL; does not perform any file download or binary replacement. |
| `proofboard update-dictionary` | **Stub Only** | Checks, validates, and replaces `dictionary.json`. | **Incomplete**: Only prints CDN check info; does not download, validate, or replace the local dictionary. |
| `proofboard config` | **Partially Compliant** | Manage watched branches, IDEs, and dictionary auto-update. | **Broken / Missing Subcommands**: Only implements `set auto-update-dictionary`. The required subcommands `add-branch`, `remove-branch`, `branches` are completely missing. |

---

## 3. Pipeline & NDA Constraints (Phase 5 Shredder) Compliance

### NDA Safety Constraints (Stored & Transmitted vs Destroyed)
The codebase strictly complies with the *transmission* constraints:
- **Transmitted fields**: SHA hashes, timestamps, additions, deletions, files changed, category names, cluster metadata, orgHash, emailHash.
- **Destroyed/Omitted fields**: Commit subject lines, file paths, repository names, organization names, and author emails are never included in the JSON payload transmitted by `internal/api/sync.go`.

### Phase 5 Shredder Implementation Issues
The specification mandates a **"strict in-memory constraint"** for commit text:
1. **Heap Allocation of Immutable Strings**: During Phase 2 Classification (`internal/pipeline/phase2/intent.go` line 15), the raw byte slice `commit.Subject` is converted to a string: `strings.ToLower(string(commit.Subject))`. In Go, casting a slice to a string creates a copy allocated on the heap. Because Go strings are immutable, they **cannot be overwritten or zeroed in memory**. This violates the specification's strict in-memory requirement: *"All string variables holding commit subject lines must be explicitly zeroed and set to nil after Phase 2 scoring completes."*
2. **Phase 5 Zeroing is a No-Op**: In Phase 2 `Classify`, `crypto.ZeroBytes(commit.Subject)` is called, and `commit.Subject` is set to `nil`. Consequently, when Phase 5 `Shred` is executed, `commits[i].Subject` is already `nil`. The call to `crypto.ZeroBytes(commits[i].Subject)` has no effect.

---

## 4. Background Daemon vs Hooks Verification

- **No background daemon present**: Verified that no background service or persistent process is registered or built.
- **Hooks configuration**: The primary `post-merge` hook and secondary `post-rewrite` hook are successfully installed on `proofboard link` and uninstalled on `proofboard unlink`. They run asynchronously `2>/dev/null &` and correctly invoke `proofboard sync --incremental --from-hook`.
- **Divergence**: The CLI references a `daemon.log` path (in `runtime.go`) instead of the spec-mandated `sync.log` path, which is a leftover from when a daemon architecture was planned.

---

## 5. Incomplete / Missing Features per Sprint Specification

Below is a breakdown of missing features grouped by their designated Sprint in `SPEC.md`:

### Sprint 1
- **Phase 4 (Milestone Cluster Boundary Detection)**: Non-compliant. The spec requires PR merge commits to mark natural cluster boundaries and define separate milestone clusters. Instead, `internal/pipeline/phase4/milestones.go` bundles all commits into a single cluster.
- **Phase 7A (Outcome Summary Generation)**: Completely missing. The Go classification pipeline has no code or logic to generate the one-sentence metadata-based professional prefill.

### Sprint 2
- **Pre-Classification Trivial Commit Filter**: Completely missing. No check for single commits, doc-only changes, high noise, or revert-only ranges is executed. The CLI does not write aborted sync info (`trivial merge skipped — [timestamp] — [repoHash]`) to `~/.proofboard/sync.log`.
- **New Project Detection Prompt**: Completely missing. The three-option terminal prompt (`y` / `n` / `x`) when detecting an unlinked repository is unimplemented.
- **Permanent suppression storage**: The `suppressedWorkspaces` array in `state.json` and the logic to suppress workspaces are completely absent.
- **Proof-of-Ship Terminal Echo**: The required verbatim terminal output (`✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard`) is replaced with a generic sync log.

### Sprint 3
- **Logging to `sync.log` with Rotation**: Completely missing. The `logging` package in `internal/logging/` is a stub and is never imported or called in the CLI commands. Sync execution is silent on disk, and `proofboard logs` is broken.
- **Command implementation stubs**: `proofboard update` and `proofboard update-dictionary` are only stubs that print URLs and do not download files.
- **Monthly Career Summary Terminal Trigger**: Completely missing. The `monthlyCareerSummaryShown` tracking map is missing from the Go State struct, and the CLI does not check for or output the career summary notification.
- **Branch Filter Configuration**: The config subcommands `add-branch`, `remove-branch`, and `branches` are unimplemented.

### Phase 2 IDE Process Watcher (Optional)
- Confirmed that the `watcher start`, `watcher stop`, `watcher status` commands, and config tools (`add-ide`, `remove-ide`) are not implemented. This is compliant with the SPEC's recommendation to defer Phase 2 Watcher features until after CLI adoption metrics are gathered.
