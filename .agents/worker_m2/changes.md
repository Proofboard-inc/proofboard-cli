# Changes Made

This document details all modifications implemented for Milestone 2 (Pipeline Extensions).

## 1. Phase 4 Milestone Boundary Detection
- **`internal/git/log.go`**:
  - Implemented `MergeTimestamps` function which executes `git log --merges --format="%at"` in the repository and returns a sorted slice of Unix timestamps (`[]int64`).
- **`internal/pipeline/pipeline.go`**:
  - Added `MergeTimestamps []int64` field to the `RunInput` struct.
  - Passed `input.MergeTimestamps` to `phase4.Detect(scored, input.MergeTimestamps)`.
- **`internal/pipeline/phase4/milestones.go`**:
  - Refactored `Detect` to partition commits chronologically by looking up which boundary interval (`mergeTimestamps`) each commit's timestamp falls into.
  - Computed cluster metrics: `clusterLabel` (top category by total scores with alphabetical tie-breaker), `impactType` (most frequent impact type with alphabetical tie-breaker), `scale` ("large" >30, "medium" 10-30, "small" <10 commits), commit count, additions, deletions, duration in days, and up to 3 reference SHAs.
  - Populated `OutcomeSummary` for each cluster.

## 2. Phase 7A Outcome Summary Generation
- **`internal/model/cluster.go`**:
  - Added `OutcomeSummary string `json:"outcomeSummary"`` field to the `model.Cluster` struct.
- **`internal/pipeline/phase7a/summary.go`**:
  - Created a new package/file implementing the outcome summary generator. It constructs a one-sentence professional summary for each cluster utilizing permitted inputs: primary category, secondary category, impactType, scale, commitCount, and duration (converted to weeks or days). No prohibited text/metadata is leaked.

## 3. Pre-Classification Trivial Commit Filter
- **`internal/commands/sync.go`**:
  - Implemented trivial filters before classification/zeroing subjects:
    - Single commit range check (`len(raw) == 1`).
    - Documentation-only check (comparing filename patterns like `README`, `CHANGELOG`, `LICENSE` and extensions `.md`, `.txt`, `.rst`).
    - Revert-only check (matching prefix `"revert:"` case-insensitive).
  - Implemented boilerplate noise checks after classification (checking if average `aiNoiseScore` > 0.85).
  - If any trivial abort condition is met:
    - Writes `trivial merge skipped — [timestamp] — [repoHash]` to `~/.proofboard/sync.log`.
    - Returns `nil` (exits command silently with status 0, empty stdout/stderr, and zero network transmission).

## 4. Tests Added
- **`internal/commands/sync_test.go`**: Tests for `isDocFile` helper and `abortSync` logging logic.
- **`internal/pipeline/phase7a/summary_test.go`**: Tests for the format and logic of the outcome summary generator.
- **`internal/pipeline/phase4/milestones_test.go`**: Tests for chronological clustering, sorting, tie-breaking category ranking, impact type ranking, and cluster metric calculations.
- **`internal/git/log_test.go`**: Added git-based test `TestMergeTimestamps` checking correct timestamp extraction on a temporary Git repository.
