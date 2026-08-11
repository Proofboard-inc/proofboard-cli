# CLI accuracy, security, and dictionary-sync overhaul

Branch: `feat/cli-mvp-accuracy`

## Summary

This PR tightens the CLI's trust model, fixes two real correctness bugs in the classification/clustering pipeline, and replaces the dictionary update mechanism with one that actually works against the live backend. It also adds a few smaller UX pieces (spinner, monorepo detection, company/role autofill, unlink notification).

The corresponding backend endpoints (`GET /api/v1/cli/dictionary`, `POST /api/v1/cli/sync`, `DELETE /api/v1/cli/repos/:repoHash`) are already live on `proboardly-backend` — that side was pushed directly and needs no separate PR, so this description covers CLI changes only.

---

## 1. Device signing is now mandatory, not optional

Every sync payload must now carry a valid device signature. There used to be a `PROOFBOARD_DEVICE_SIGNING_MODE` config flag that could disable client-side signing entirely; it's gone. The backend already rejects any payload missing `deviceKeyId`/`deviceSignature` with a 401, so keeping a client-side toggle that led straight to that 401 was dead weight — worse, it was a genuine trust hole: an attacker could otherwise choose not to sign.

- `internal/config/config.go` — removed `DeviceSigningMode` from `Config` and its env var / validation.
- `internal/commands/sync.go` — the transmit step now unconditionally registers/loads the device key and signs the payload; the old `if mode == "required"` branch is gone.
- `internal/auth/device_key.go` — `Ensure()` no longer rewrites the OS keychain entry on every sync. It only calls `Save()` when the key is freshly registered or migrating from the legacy on-disk file, tracked via a `fromKeychain` flag. Previously every sync triggered a real Keychain write, which is what caused the repeated macOS "Keychain Access" permission prompts.
- Added a `PROOFBOARD_DISABLE_KEYCHAIN` env var / persisted `state.KeychainDisabled` flag (`internal/model/state.go`, `internal/commands/config.go`) for machines where interactive Keychain access isn't viable (CI, headless).

## 2. Canonical JSON no longer HTML-escapes payload bytes

`internal/crypto/canonical_json.go` used `json.Marshal`, which HTML-escapes `&`, `<`, `>` to `&` etc. by default. Node's `JSON.stringify` — what the backend re-serializes the payload with to verify the signature — never does this. Any commit category or field containing one of those characters (e.g. `"Authentication & Security"`) produced two different byte strings on each side, so the signature verified locally but failed server-side. Fixed by switching to `json.NewEncoder` with `SetEscapeHTML(false)`.

Verified against real `node -e 'JSON.stringify(...)'` output in `internal/crypto/canonical_json_test.go`.

## 3. Milestone clustering: capped and content-aware instead of an even split

`internal/pipeline/phase4/milestones.go` was rewritten. Two problems this fixes:

- **No cap.** A first sync on a repo with many small PRs was turning every merge commit into its own milestone — 50 milestones from 87 commits on one test repo. Added `maxClustersPerSync = 6` and a `consolidate()` pass that repeatedly merges the adjacent pair of clusters with the lowest merge cost until the count fits the cap. Same-category neighbors merge before cross-category ones (`crossCategoryPenalty`), so the algorithm prefers folding together two PRs that are really the same feature over merging unrelated work.
- **No merge-commit signal at all (rebase/squash workflows).** The old code did an arbitrary even split of the commit list. Replaced with `segmentLinear()`, which walks the chronological commit list and starts a new segment whenever the primary category changes or a gap larger than `linearSegmentGapHours` (72h) opens up — so "two weeks on auth, then a week on payments" becomes two segments instead of a fixed N-way split.

Also added `ReferenceShas` to each cluster (`model.Cluster`) — a first/middle/last sample (max 3) of that cluster's *own* commits, replacing the backend's previous index-arithmetic guess into the flat top-level `shas[]` array, which could point at commits from a different cluster entirely.

## 4. Classification: weighted signals instead of first-match, plus a feature-keyword layer

`internal/pipeline/phase2/intent.go`:

- Category scoring now weights signals by how trustworthy they are: file-path match (`pathWeight = 3`, structural, can't be faked in a message) > subject line (`subjectKeywordWeight = 2`) > body text / extracted symbols (`bodyKeywordWeight = symbolWeight = 1`). Ties are broken in favor of the category with a structural path match, then alphabetically.
- Category iteration now goes through a sorted slice of dictionary keys instead of ranging a Go map directly — map iteration order is randomized per process, which was silently changing which category won a tied score from run to run.
- New: **feature keywords**. Separate from category (`Category` answers "what kind of work"; `FeatureKeyword` answers "which feature" — "dashboard", "checkout", "onboarding"). Matched against a flat vocabulary (`Dictionary.FeatureKeywords`) rather than the 25-category list — a commit's category alone was too coarse a signal for the AI to write a distinct outcome summary for each milestone, so several unrelated "API & Backend Services" bundles read almost identically. The cluster's dominant feature keyword (by commit count, not score sum) goes into the sync payload.
- New file `internal/pipeline/phase2/symbols.go` — extracts function/class/import-like identifiers from the commit body text via per-language regex sets, used as a lightweight extra classification signal (`symbolWeight`) when the subject line alone is uninformative ("updates", "fix").

## 5. Dictionary sync now actually works

`internal/dictionary/updater.go` was rewritten. The old flow was a two-step "check version, then download from a separate URL" pattern modeled on a release-CDN (`releases.proofboard.io`) that was never built — `update-dictionary` and the periodic startup check were both silently failing against every real environment.

`Update()` now does a single GET directly against the backend's `GET /api/v1/cli/dictionary` (already public, no auth required) and decodes the response body straight into `model.Dictionary`. Compare-version, validate, atomic temp-file-then-rename install are unchanged. Removed the `ReleaseBaseURL` parameter from `Update()`'s signature — the CLI binary's own GitHub-release update mechanism (`internal/commands/update.go`) still uses `ReleaseBaseURL`/`ReleaseClient`, this was only ever needed for the dictionary specifically.

`internal/dictionary/dictionary.json` — the embedded bootstrap copy — is now a fallback only for a machine that has never synced; the backend's dictionary response is the live source of truth going forward.

`CompareVersions` moved from `internal/commands` into `internal/dictionary` so both the command and the startup check share one implementation (`internal/dictionary/version_compare_test.go` replaces the old `internal/commands/version_compare_test.go`).

## 6. Local company/role autofill at link time

`internal/commands/link.go` — `promptForCompanyAndRole()`. When linking interactively, the CLI now offers the org name it already computed locally from the git remote (`pbgit.ParseRemote`) and the tech-stack labels from local detection (item 7) back to the user to confirm or edit, instead of leaving both blank. Never guesses from repo/commit *content* — only from data already computed locally for other purposes. Skipped entirely for `--non-interactive`/agent-triggered runs, and the backend only applies the values if the request creates a brand-new project (never overwrites an existing one's values).

## 7. Local tech-stack + monorepo manifest detection

New package `internal/detection/stack.go`. Pure Go, no third-party dependency (no go-enry/tree-sitter) — extension histograms via `git ls-files` for language counts, and manifest scans (`package.json`, `go.mod`, `requirements.txt`, `Cargo.toml`, `Gemfile`, `composer.json`, etc.) for framework labels plus structural flags (`HasCI`, `HasTests`, `HasDocker`, `HasIaC`). Only labels/counts/booleans leave the machine — never file contents.

Manifests are matched by basename anywhere in the tracked-file tree (capped at 20 scanned), not just at repo root, so a monorepo with `apps/frontend/package.json` and `apps/backend/go.mod` and nothing at the actual root gets detected correctly instead of reporting an empty stack. Feeds both the sync payload (`model.StackReport`, refreshed on every sync) and the link-time company/role prompt (item 6).

## 8. Unlink notifies the backend

`internal/commands/unlink.go` + new `internal/api/unlink.go` (`DELETE /api/v1/cli/repos/:repoHash`). Previously `unlink` only removed local state and git hooks — the backend never found out, so a project stayed marked CLI-active indefinitely after a local unlink. A 404 from the backend (already unlinked, or never registered) is treated as success, not a warning; any other failure prints a warning but still completes the local cleanup, since local state should never get stuck because of a network blip.

## 9. Terminal spinner

New package `internal/spinner`, plus `internal/commands/spinner_helper.go`'s `withSpinner()`. Dependency-free (manual `os.ModeCharDevice` check instead of `golang.org/x/term`), and inert whenever output isn't a real TTY — `--json`, piped output, and hook/agent-triggered background syncs never get spinner frames written into their output.

## 10. Notification dashboard URL

`internal/notifications/workspace.go` — the "Review this milestone" desktop notification action had `https://proofboard.io/dashboard` hardcoded, which 404s. Now reads from config, falling back to the environment's actual frontend URL.

---

## Testing

- `go build ./...`, `go vet ./...` clean.
- Full suite passes package-by-package: `internal/api`, `internal/auth`, `internal/config`, `internal/crypto`, `internal/detection`, `internal/dictionary`, `internal/git`, `internal/hooks`, `internal/logging`, `internal/notifications`, `internal/pipeline` (all phases), `internal/state`.
- `internal/commands` passes in full except two pre-existing, unmodified end-to-end tests (`TestAuthCommandEndToEnd`, `TestCareerAgentEndToEndAuthorizesConnectsAndSyncs`) that require real interactive macOS Keychain access and a reachable dev backend per this repo's testing policy (`AGENTS.md`) — not something a sandboxed run can satisfy, and neither test was touched by this branch.

## Files changed

**Modified (39):** `.gitignore`, `internal/api/{client,link}.go`, `internal/auth/device_key{,_test}.go`, `internal/commands/{command_behavior_test,compliance_test,config,link,milestone3_test,milestone4_test,root,sync,unlink,update,update_dictionary}.go`, `internal/config/config{,_test}.go`, `internal/crypto/canonical_json{,_test}.go`, `internal/dictionary/{dictionary.json,updater.go}`, `internal/git/log{,_test}.go`, `internal/model/{cluster,commit,dictionary,payload,state}.go`, `internal/notifications/workspace{,_test}.go`, `internal/pipeline/phase2/intent{,_test}.go`, `internal/pipeline/phase4/milestones{,_test}.go`, `internal/pipeline/phase5/shredder.go`, `internal/pipeline/phase7/payload{,_test}.go`, `internal/pipeline/pipeline{,_test}.go`

**Deleted (1):** `internal/commands/version_compare_test.go` (moved to `internal/dictionary/version_compare_test.go`)

**New (13):** `internal/api/{link_test,unlink}.go`, `internal/commands/{link_prompt_test,spinner_helper,startup_dictionary_throttle_test}.go`, `internal/detection/{stack,stack_test}.go`, `internal/dictionary/{updater_test,version_compare_test}.go`, `internal/model/stack.go`, `internal/pipeline/phase2/{symbols,symbols_test}.go`, `internal/spinner/spinner.go`

---

## 11. Accuracy pass: category split, multi-keyword matching, conventional-commit impact

Follow-up round on real dogfooding output where clusters were too coarse and outcome summaries read generically.

- **`internal/pipeline/phase2/intent.go`** — a commit now keeps EVERY feature keyword that scores > 0 (`model.CommitSignal.FeatureKeywords`), not just the single top-scorer. `phase4/milestones.go`'s `dominantFeatureKeyword` now counts across all of a commit's matched keywords and can name a cluster with two independently-strong keywords ("orders & delivery") instead of always collapsing to one.
- **Conventional Commits prefix now overrides category-inferred impact.** A `chore:`/`docs:`/`test:`/`build:`/`ci:`-prefixed commit was landing as impact `feature` purely because it happened to keyword-match a category whose dictionary default is `feature`. `impactFromConventionalPrefix()` reads the subject's declared type (author's explicit signal) and overrides the category default when present.
- **Backend dictionary** (`proboardly-backend/src/modules/cli/schemas/cli-dictionary.ts`) — "API & Backend Services" was a single catch-all matching `api|route|controller|handler|middleware|webhook|...`, absorbing 40-50+ commits per repo into one milestone with nothing more specific to say than its own category name. Split into five: **API & Backend Services** (routing/contracts only now), **Business Logic & Domain Services**, **Background Jobs & Queues**, **Integrations & Webhooks**, **Request Validation & Middleware**. Mirrored in `commit-category.helper.ts` and `category-signal.helper.ts` (the VCS_PUBLIC path's classifiers) and in this repo's embedded bootstrap dictionary. Career-signal archetype matching (`archetypes.config.ts`) summed the old category's user share for any requirement that referenced it, via a new `ARCHETYPE_CATEGORY_GROUPS` map, so existing archetypes (Backend Engineer, etc.) keep matching the same population instead of silently breaking once the split dilutes any one category's percentage.
- **Feature keyword vocabulary** expanded from ~110 to 300+ entries — was skewed toward e-commerce/JS-web; added fintech/banking, healthcare, legal, real estate, education, HR/recruiting, logistics/delivery, travel, gaming, IoT, and general backend/infra terms (rate limiter, circuit breaker, event sourcing, connection pooling, etc.), plus compound phrases ("delivery rider", "crypto wallet") so a single contained word still resolves the whole concept.
- **AI outcome-summary prompt** (`proboardly-backend/src/infrastructure/ai/ai.service.ts`) — two concrete failure modes fixed: (1) the model was blending two verbs from the offered pool into one sentence ("Hardened designed and delivered..."); the prompt now demands exactly one verb, copied verbatim, as the first word. (2) the prompt's own example text for the no-feature-keyword case ("a large-scale API & Backend Services effort") was getting copied near-verbatim into real output — replaced with a non-copyable instruction. The filler/vague-word ban list was expanded significantly (CV-reads-as-padding words: "leveraging", "cutting-edge", "efforts", "solutions", "best practices", etc.). `usedVerbs` (already tracked per-project to dedupe opening verbs) is now also passed INTO the prompt (`avoidVerbs`) so the model avoids a repeat itself, instead of relying solely on the post-hoc mechanical swap.

## 12. Redundant/broken notification paths removed

- **The background Career Agent no longer fires its own OS-level "Project detected" popup.** `internal/commands/agent.go`'s IDE-workspace poller (`inspectIDEWorkspaces`) used to spawn `proofboard notify`, which on macOS shows either a corner `terminal-notifier` banner or — when that optional tool isn't installed — an always-centered, modal AppleScript `display dialog`. With several IDE windows open on different repos, this stacked up multiple centered popups (screenshotted during testing). That responsibility now belongs solely to the shell startup hook (see below); `launchWorkspaceNotification` was removed from `detect.go`.
- **Fixed the shell hook so "Project detected" is actually visible.** The installed hook line ran `detect` backgrounded with BOTH stdout and stderr sent to `/dev/null` — so its terminal message was silently discarded on every single terminal open, AND `detect` records the prompt as "shown" the instant it runs, permanently burning the one-time prompt with nothing ever displayed. `detect` only does fast local git plumbing (no network), so there's no latency reason to background/silence it. The hook line is now synchronous with only stderr suppressed (`proofboard detect 2>/dev/null`), matching the existing `notices` line's pattern; `detect`'s own inspection is now wrapped in a 2s bound as a safety net. `ensureLineInFile`'s migration logic now recognizes the old backgrounded line (and the old PowerShell `Start-Process -WindowStyle Hidden` line) as legacy and rewrites it, and this migration now also runs opportunistically on every command (cheap local file check, no network) instead of only once at `proofboard install` time — so existing installs self-heal instead of staying stuck with the broken line forever.
- **Removed the CLI's local "monthly career summary" notification** (`triggerMonthlyCareerSummary`/`getReadyCareerSummaryMonth` in `runtime.go`, `MonthlyCareerSummary` event, `MonthlyCareerSummaryShown` state field). It fired on a purely local last-Friday-of-month clock guess, completely disconnected from whether the backend had actually generated anything, and hardcoded a dead link (`proofboard.io/career-summary`, not the configured app URL). The backend's actual `MonthlyCareerSummaryWorker` cron job and its real notification pipeline are unaffected — this only removes the CLI's own fabricated local popup.
- **Dictionary auto-update already keeps a local copy and already prints a terminal notice when it updates** (`internal/dictionary/updater.go`'s atomic rename install + `root.go`'s `"Dictionary updated successfully to version %s."` line, throttled to once per 6h) — confirmed this already matches the NDA-safe "always keep a local copy, notify in-terminal on change" requirement; no change needed there.

## Not covered by this PR (open follow-ups, tracked separately)

- `proofboard link` re-runs organisation detection and re-asks "Is this your employer/client?" even when the repo is already linked — should short-circuit with a status message instead.
- `proofboard auth` throws a network-timeout error even when already authenticated; it should allow switching accounts without requiring the device-code round trip to succeed first.
- `proofboard unlink` prints a warning framed as "hooks removed" even in cases that need clearer wording; also confirm the backend unlink call path end-to-end against a reachable environment (dev DNS was unreachable during the last test, so `DELETE /cli/repos/:repoHash` was never actually exercised).
- `proofboard update` returns a raw "GitHub latest release returned 404 Not Found" with no explanation of what it does or a next step.
- No colored/styled terminal output yet — plain text throughout.
