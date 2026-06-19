# Proofboard CLI Codebase Exploration & Gap Analysis

## Executive Summary
This analysis presents the findings of the codebase exploration for the Proofboard CLI at `/workspaces/proofboard-cli` against the updated `SPEC.md` (v1.4), `README.md`, and `GEMINI.md` (v1.2/v1.4 specification). The current codebase has successfully implemented all prior milestone features, including the strict in-memory execution pipeline (Phase 2-5 run *before* the Phase 6 handshake), the branch filter gate, the pre-classification trivial commit filter, update commands (both binary and dictionary), rotated sync logging, and simplified notification prompts. However, a major documentation and routing discrepancy exists regarding the CLI-specific endpoints (`/cli/link` and `/cli/sync`) which are completely omitted from the OpenAPI specification in `SPEC.md`.

---

## 1. Compliance Audit: CLI Implementation vs. Specification
A review of the Go CLI implementation shows that the codebase conforms tightly to the requirements in `SPEC.md` (v1.4), `README.md`, and `GEMINI.md`.

| Feature Area | SPEC.md v1.4 Requirement | CLI Implementation Status |
|---|---|---|
| **Daemon Removal** | 30-minute background daemon removed entirely. Replaced with `post-merge` and `post-pull` (via `post-rewrite`) hooks. | **Compliant.** No background daemon exists. The `link` command installs `post-merge` and `post-rewrite` hooks, while the `unlink` command removes them. |
| **Pipeline Execution Order** | Local pipeline processing and shredding (Phases 1-5) must occur in-memory *before* the Phase 6 remote network handshake. | **Compliant.** Refactored in milestone 5 remedy: `pipeline.Run` executes first, then `pbgit.LSRemoteHandshake` is run, and the payload is updated with the actual handshake status before transmission in Phase 8. |
| **Branch Filter Gate** | Sync triggers on hook must silent-exit if the branch is not on the watched list. watched branches manage via `config` subcommands. | **Compliant.** Commands `config add-branch`, `config remove-branch`, and `config branches` are implemented. `sync` checks `pbgit.IsProductionBranch` before starting. |
| **Pre-Classification Filter** | Skip sync and log `trivial merge skipped` if range has 1 commit, only doc files, high boilerplate (>0.85), or only reverts. | **Compliant.** Implemented in `sync.go` with check methods (`isDocFile`) and noise thresholds, writing compliant messages to `sync.log`. |
| **Self & Dictionary Updates** | Automatic/manual check of latest version, download to temp file, validate schema/checksums, and rename atomically. | **Compliant.** `update` and `update-dictionary` are fully implemented using temp files to ensure atomic replacement on the same volume. |
| **Notification Architecture** | Exactly three notification events: Project Detection Prompt (`y`/`n`/`x`), Proof-of-Ship Echo, and Monthly Career Summary quiet terminal line. | **Compliant.** The prompt is implemented synchronously in `sync.go` with permanent suppression mapping; the monthly summary line triggers on project open. |

---

## 2. API Endpoint Comparison
The OpenAPI specification starting at line 929 of `SPEC.md` was compared against the implemented client calls under `internal/api/`.

### Implemented vs. Documented Paths
1. **Notifications**:
   - `/api/v1/notifications` (GET, paginated) $\rightarrow$ Implemented via `GetNotifications` in `notifications.go`.
   - `/api/v1/notifications/unread-count` (GET) $\rightarrow$ Implemented via `GetUnreadNotificationCount` in `notifications.go`.
   - `/api/v1/notifications/{id}/read` (PATCH) $\rightarrow$ Implemented via `MarkNotificationRead` in `notifications.go`.
   - `/api/v1/notifications/mark-all-read` (PATCH) $\rightarrow$ Implemented via `MarkAllNotificationsRead` in `notifications.go`.
2. **Activity Log**:
   - `/api/v1/activity-log` (GET, paginated) $\rightarrow$ Implemented via `GetActivityLog` in `activity.go`.
3. **VCS / CLI Operations**:
   - **CLI Link**: The CLI calls `c.linkPath` which defaults to `/cli/link`.
   - **CLI Sync**: The CLI calls `c.syncPath` which defaults to `/cli/sync`.

### Discrepancies Found
- **Omission from OpenAPI Docs**: The endpoints `/cli/link` and `/cli/sync` are **not** present in the OpenAPI specification defined in `SPEC.md` (lines 929-7492). The OpenAPI documentation only describes paths prefixed with `/api/v1/` for the web app (e.g. `/api/v1/projects` for manual project creation or `/api/v1/projects/vcs/import` for cloud imports).
- **Prefix Inconsistency**: Unlike all documented endpoints which are versioned under `/api/v1/` (such as `/api/v1/notifications` or `/api/v1/activity-log`), the CLI-specific endpoints `/cli/link` and `/cli/sync` do not use the `/api/v1` prefix.

---

## 3. Discrepancies and Backend PR Recommendations
Because the CLI default config targets `/cli/link` and `/cli/sync` directly on the `APIBaseURL` (default: `https://api-dev.proofboard.io`), the backend API gateway or server must support routing these two endpoints.

### Recommended Action for `proofboard-backend`
A Pull Request should be created for `proofboard-backend` (https://github.com/Proofboard-inc/proofboard-backend) to address the following:
1. **Route Mapping / Alias**:
   Ensure the NestJS/backend routing maps incoming POST requests at `/cli/link` and `/cli/sync` to the appropriate internal controller actions.
   - If the backend controllers require the `/api/v1/` prefix (e.g., `/api/v1/cli/link` and `/api/v1/cli/sync`), the backend proxy/gateway must support rewriting or aliasing `/cli/link` and `/cli/sync` to their respective `/api/v1/cli/` versions to maintain backwards compatibility with existing CLI binaries.
2. **OpenAPI Specification Sync**:
   Add the following endpoint specifications to the backend OpenAPI JSON config (and thus sync them into `SPEC.md` in the future):
   - `POST /cli/link`:
     - **Summary**: Register a new local git repository.
     - **Request Body**: `LinkRequest` (`orgHash`, `repoHash`, `emailHash`).
     - **Response**: `200 OK` with `LinkResponse` (`displayOrg`, `tier`).
   - `POST /cli/sync`:
     - **Summary**: Submit anonymized git metadata payload from CLI client.
     - **Request Body**: `SyncPayload` schema containing shas, timestamps, categories, and anti-fraud signals.
     - **Response**: `201 Created` with `SyncReceipt` (`id`, `tier`, `status`).

---

## 4. Verification and Local Testing
- All local tests have been verified with `go test ./...` and pass cleanly.
- Code style and correctness were verified via `go vet ./...` with no errors.
- Since we are operating in `CODE_ONLY` network mode, no external connections to `github.com` or `proofboard-backend` were initiated to verify the remote repository. The recommended PR is based on logical discrepancy detection between the Go code's routing defaults and the API Docs.
