# Project: Proofboard CLI Compliance & Release v1.4

## Architecture
Proofboard CLI is a local-first Git activity classification tool written in Go.
Data flows chronologically through the 8-Phase classification pipeline:
- **Phase 1 (Ingest)**: Executes `git log` to extract commits by developer email.
- **Phase 2 (Classification)**: Computes category and intent scores on commit subjects.
- **Phase 3 (Scoring)**: Computes Contribution Areas.
- **Phase 4 (Milestones)**: Groups commits into clusters terminated by PR merge boundaries.
- **Phase 5 (Shredder)**: Safely zeroes sensitive commit strings, drops file paths, hashes emails and repositories.
- **Phase 6 (Handshake)**: Live validation of remote repo access.
- **Phase 7 (Assemble)**: Prepares NDA-safe JSON payload.
- **Phase 7A (Outcome Summary)**: Generates safe pre-fill professional description.
- **Phase 8 (Transmission)**: Sends JWT-signed metadata to Proofboard API.

## Code Layout
- `cmd/proofboard/main.go`: Entrypoint
- `internal/api/`: API endpoints & client
- `internal/auth/`: OAuth flow & token handling
- `internal/commands/`: Cobra CLI command actions
- `internal/config/`: Configuration manager
- `internal/crypto/`: Zeroing, hashing, and encryption
- `internal/dictionary/`: Keyword & path matching dictionary
- `internal/git/`: Git repository & log processing
- `internal/hooks/`: Hook files generation
- `internal/logging/`: Log management
- `internal/model/`: Shared data models
- `internal/pipeline/`: 8-phase pipeline implementation

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Exploration, API Review, and PR Check | Explore codebase, review SPEC.md endpoints, check if backend changes/PR are needed, run tests | None | DONE |
| 2 | Compliance Validation and Fixing Gaps | Check and patch any compliance issues/gaps between the CLI codebase and SPEC.md v1.4 | M1 | DONE |
| 3 | Verification | Run full verification suite using Worker, Reviewers, and Forensic Auditor | M2 | DONE |
| 4 | Release Publication | Statically compile binaries for macOS, Linux, and Windows; publish tag/release via `gh` CLI | M3 | DONE |

## Interface Contracts
### `pipeline` ↔ `phase2`
- `Classify(commits []RawCommit, dict Dictionary) []CommitSignal`
- Zeroes raw commit subjects in place. Wipes local temporary lowercase subject byte slices before returning.

### `pipeline` ↔ `phase4`
- `Detect(result ScoredResult, repo git.Repo) []Cluster`
- Parses git history to detect merge commits and groups commits chronologically.

### `pipeline` ↔ `phase7a`
- `GenerateSummary(primary string, secondary string, commitCount int, durationDays int, additions int, deletions int) string`
- Returns a generic professional text summary.

### `commands` ↔ `logging`
- `ConfigureLogger(logPath string, maxBytes int64) error`
- Rotates log files dynamically.
