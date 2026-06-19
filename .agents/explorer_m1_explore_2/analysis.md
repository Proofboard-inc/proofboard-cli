# Codebase Exploration and Gap Analysis Report

## Executive Summary
This report documents the gap analysis performed on the Proofboard CLI Go codebase at `/workspaces/proofboard-cli` against the updated specification (`SPEC.md` version 1.4), product description (`README.md`), and compliance instructions (`GEMINI.md`). It details discrepancies identified in the API client implementation versus the documented OpenAPI endpoints in `SPEC.md` and evaluates the status of related external repositories.

---

## 1. Document & Version Discrepancies
* **CLI Target Version vs Specification Version**:
  * **Specification**: `SPEC.md` defines version 1.4 (`PROOFBOARD CLI SPEC — v1.4` at line 923).
  * **Cobra CLI & Readme**: `README.md` (line 23) and `GEMINI.md`/`AGENTS.md` (Product section) reference `Proofboard CLI v1.2`.
  * **CLI Source Version**: In `internal/version/version.go`, the CLI version is hardcoded to `1.2.0`.
  * **Impact**: While the CLI implements most of the v1.4 features, it is labeled as v1.2.0. The version string must be updated to align with the v1.4 release.

---

## 2. API Endpoints Analysis & Comparison
A direct comparison was performed between the OpenAPI specification in `SPEC.md` (starting at line 929) and the API client files under `internal/api/`.

### Implemented vs Spec Path Comparison

| Endpoint Path | Method | Implemented in CLI | Documented in `SPEC.md` OpenAPI | Notes / Discrepancies |
|---|---|---|---|---|
| `/cli/link` | `POST` | Yes (`internal/api/link.go`) | **No** | Missing from the OpenAPI definition in `SPEC.md`. |
| `/cli/sync` | `POST` | Yes (`internal/api/sync.go`) | **No** | Missing from the OpenAPI definition in `SPEC.md`. |
| `/api/v1/notifications` | `GET` | Yes (`internal/api/notifications.go`) | Yes | Unused by any command or logic in the CLI. |
| `/api/v1/notifications/unread-count` | `GET` | Yes (`internal/api/notifications.go`) | Yes | Unused by any command or logic in the CLI. |
| `/api/v1/notifications/{id}/read` | `PATCH` | Yes (`internal/api/notifications.go`) | Yes | Unused by any command or logic in the CLI. |
| `/api/v1/notifications/mark-all-read` | `PATCH` | Yes (`internal/api/notifications.go`) | Yes | Unused by any command or logic in the CLI. |
| `/api/v1/activity-log` | `GET` | Yes (`internal/api/activity.go`) | Yes | Unused by any command or logic in the CLI. |
| `/api/v1/auth/me` | `GET` | No | Yes | Auth command uses `appBaseURL/cli-auth?port=9876` instead. |

### Key Findings & Discrepancies:
1. **Critical Path Document Mismatch**: 
   The endpoints `/cli/link` and `/cli/sync` are the core API routes called by the CLI to link repositories and sync anonymized metadata payloads. However, they are completely missing from the OpenAPI specification paths list in `SPEC.md`. Furthermore, `LinkRequest`, `LinkResponse`, `SyncPayload`, and `SyncReceipt` schemas are missing from the OpenAPI components section in `SPEC.md`.
2. **Unused Code**:
   The entire notifications API wrapper (`internal/api/notifications.go`) and activity log API wrapper (`internal/api/activity.go`) are fully implemented and unit-tested but are never referenced or used by any CLI command handlers.
3. **Auth Client Wrapper Mock**:
   `internal/api/auth.go` contains a dummy `Authenticate` method that just returns `ctx.Err()`. The actual authentication is handled out-of-band via browser-OAuth and callback server loop (`internal/auth/auth.go` and `internal/auth/callback_server.go`).

---

## 3. CLI Functional Gaps with SPEC.md v1.4
The CLI implementation was checked against SPEC.md v1.4 functional requirements:

1. **Automatic Startup Updates Checks (Missing)**:
   * **Requirement (Section 2.4)**: On CLI startup, the CLI must check for a newer version by calling `GET https://releases.proofboard.io/latest.json` and print a non-blocking notice.
   * **Requirement (Section 8)**: On CLI startup, it must check for a newer dictionary version by calling `GET releases.proofboard.io/dictionary/latest.json`.
   * **Implementation**: Startup update checks are completely missing. Checks and updates are only performed manually when running `proofboard update` or `proofboard update-dictionary`.
2. **Unused Dictionary Auto-Update Configuration flag**:
   * **Requirement (Section 8)**: `proofboard config set auto-update-dictionary false` should disable automatic dictionary updates on startup.
   * **Implementation**: The Cobra command is fully implemented to set/get the `AutoUpdateDictionary` flag in `state.json`, but the setting is never read or checked anywhere else in the codebase.
3. **Proof-of-Ship Notification Echo Discrepancy**:
   * **Requirement (Section 5A.2)**: When a sync completes and a qualifying milestone payload is transmitted, the CLI prints:
     `✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard`
   * **Implementation**: In `internal/commands/sync.go` (line 390), this message is only printed when `linkedThisTime` is true (i.e. only when linking the project during the sync command execution's interactive prompt). Successful subsequent `proofboard sync` calls do not print this message, which violates the spec.

---

## 4. External Repository & PR Check (proofboard-backend)
* **Check Results**:
  * Using the GitHub CLI (`gh repo view Proofboard-inc/proofboard-backend`), the query failed with `GraphQL: Could not resolve to a Repository with the name 'Proofboard-inc/proofboard-backend'.`
  * Using `gh repo list Proofboard-inc`, it returns `There are no repositories in @Proofboard-inc`.
  * **Conclusion**: The repository `Proofboard-inc/proofboard-backend` does not exist or is private and inaccessible under the current credentials, so no pull request can be opened there.
  * **Required Changes (If PR could be opened)**:
    * The backend OpenAPI documentation must be updated to include the `/cli/link` and `/cli/sync` paths.
    * The corresponding DTO schemas (`LinkRequest`, `LinkResponse`, `SyncPayload`, `SyncReceipt`) must be added to the OpenAPI specification component schemas.

---

## Recommendations
1. **Update Version Label**: Increment the version const in `internal/version/version.go` to `"1.4.0"` and update version strings in `README.md` and `GEMINI.md` to keep documentation consistent.
2. **Implement Startup Version/Dictionary Checks**: Add non-blocking asynchronous or fast synchronous checks to `NewRootCommand` or `Execute` to query the latest binary and dictionary versions from the CDN endpoints, respecting the `AutoUpdateDictionary` configuration from `state.json`.
3. **Fix Proof-of-Ship Terminal Echo**: Modify `internal/commands/sync.go` to always print the `✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard` line on successful sync payload transmission, regardless of whether the repo was linked during that run or previously.
4. **Clean up or Integrate Unused Endpoints**: Either remove `internal/api/notifications.go` and `internal/api/activity.go` to reduce code bloat, or expose them via new CLI subcommands (e.g. `proofboard notifications` or `proofboard activity`).
