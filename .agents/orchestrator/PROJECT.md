# Project: Proofboard CLI Compliance & Release v1.2/v1.4

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
| 1 | In-Memory Safety & Shredder | Eliminate heap-allocated string copies of commit subjects; ensure Phase 2 & 5 wipe memory securely | None | DONE |
| 2 | Pipeline Extensions | Implement Phase 4 PR merge boundaries, Phase 7A Outcome Summary, and Pre-Classification Trivial Commit Filter | M1 | DONE |
| 3 | CLI Subcommands & Prompts | Implement missing config branch commands, monthly career summary terminal output, and new project detection workspace prompt with suppression list | M2 | DONE |
| 4 | Updates & Logging | Implement structured logging to sync.log with rotation, proofboard update and update-dictionary functional logic | M3 | DONE |
| 5 | Cross-Platform Builds & Release | Perform final test runs, compile statically for Linux/macOS/Windows, auto-publish release package via GitHub CLI | M4 | DONE |

## Interface Contracts
### `pipeline` ↔ `phase2`
- `Classify(commits []RawCommit, dict Dictionary) []CommitSignal`
- Zeroes raw commit subjects in place. Wipes local temporary lowercase subject byte slices before returning.

### `pipeline` ↔ `phase4`
- `Detect(result ScoredResult, repo git.Repo) []Cluster`
- Parses git history to detect merge commits and groups commits chronologically.

### `pipeline` ↔ `phase7a` (New)
- `GenerateSummary(primary string, secondary string, commitCount int, durationDays int, additions int, deletions int) string`
- Returns a generic professional text summary.

### `commands` ↔ `logging`
- `ConfigureLogger(logPath string, maxBytes int64) error`
- Rotates log files dynamically.
