# Gap Analysis: Proofboard CLI vs SPEC.md v1.4

## Summary of Findings
An initial codebase exploration was performed to identify any gaps between the current Proofboard CLI Go implementation and the updated `SPEC.md` (version 1.4), `README.md`, and `GEMINI.md`. Several key functional gaps and API discrepancies were identified, detailed below.

---

## 1. Functional Gaps in CLI Implementation

| Gap | Description | SPEC.md Reference | File & Line |
|---|---|---|---|
| **Startup Update Check** | The CLI does not check for a newer version on startup. It is required to make a non-blocking check to `releases.proofboard.io/latest.json`. | Section 2.4 (Line 98) | `cmd/proofboard/main.go` & `internal/commands/root.go` |
| **Auto-Update Dictionary** | The dictionary auto-update check is unimplmented. `dictionary.Update` is a mock. The configuration option `AutoUpdateDictionary` exists but is never checked. | Section 10 (Line 710) | `internal/dictionary/updater.go:5`, `internal/commands/sync.go` |
| **Pending Sync Status** | The `proofboard status` command prints state fields but does not identify if there are pending unsynced local commits (by comparing local HEAD with `lastHeadSha`). | Section 3 (Line 114) | `internal/commands/status.go:36` |
| **Terminal Echo Logic** | The "Proof-of-Ship" terminal echo is only printed when a repository is linked for the first time in `sync.go`. It should print on every successful sync that transmits commits. | Section 5A.2 (Line 451) | `internal/commands/sync.go:390` |
| **Tier Naming Display** | The CLI prints `"Tier2"` and `"Tier2-skipped"` to the terminal on sync/status rather than the updated tier names `"SHA Proof"` and `"SHA Proof — handshake skipped"`. | Changes v1.4 #6 (Line 16) | `internal/commands/sync.go:368-370`, `internal/commands/status.go:36` |

---

## 2. API Endpoint Alignment Review

Comparing `SPEC.md` OpenAPI Specs (starting around line 929) with `internal/api/`:

1. **Missing OpenAPI Routes**:
   The CLI client requests `/cli/link` (`LinkPath`) and `/cli/sync` (`SyncPath`). However, these routes are completely missing from the OpenAPI schema defined in `SPEC.md`. The OpenAPI spec only defines `/api/v1/projects` and `/api/v1/vcs-sync/trigger`.
   * **Recommendation**: Ensure the backend OpenAPI spec documents `/cli/link` and `/cli/sync` (or `/api/v1/cli/link`, etc.) and ensure consistency in paths.

2. **Unused Client Methods**:
   The client implements `/api/v1/notifications` endpoint calls (`GetNotifications`, `GetUnreadNotificationCount`, `MarkNotificationRead`, `MarkAllNotificationsRead`) in `internal/api/notifications.go` and `internal/api/activity.go` but no commands in `internal/commands/` make use of them.

---

## 3. Discrepancies in External Components (Backend PR)

If `/cli/link` and `/cli/sync` are indeed implemented in the backend, they should be documented in the backend's OpenAPI schema. Since the parent environment is isolated in `CODE_ONLY` network mode, we cannot clone or push to the backend repository at `https://github.com/Proofboard-inc/proofboard-backend`. 
However, a PR would be required in `proofboard-backend` to:
- Formally document the `/cli/link` and `/cli/sync` endpoints in the OpenAPI spec.
- Align the HTTP responses and schemas with `SyncPayload` and `SyncReceipt`.
