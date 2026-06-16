Updated spec:

PROOFBOARD   |   CLI Engineer Specification   |   v1.4   |   Go
PROOFBOARD
CLI Engineer Specification
Version 1.4  |  June 2026
Implementation language: Go  |  Open source: MIT  |  Audience: CLI Engineering team
What this document is: The complete technical specification for the Proofboard CLI — version 1.4.
 
Changes from v1.2:
(1) 30-minute background daemon removed entirely. Replaced with post-merge hook (primary) and post-pull hook (secondary). No persistent process.
(2) Branch filter gate added.
(3) Pre-classification trivial commit filter added.
(4) Notification architecture simplified — exactly three notifications: new project detection prompt, Proof-of-Ship terminal echo, Monthly Career Summary terminal line. Weekly Progress Report removed. Quarterly snapshot notifications removed. VAPID Web Push removed. WebSocket cards removed. Milestone highlights held in the application — no notification sent.
(5) Monthly Career Summary is the only proactive outreach. One email per month plus one quiet terminal line the first time the engineer opens a linked project after that month's summary generates.
(6) Tier naming updated — SHA Proof, Volume Proof, Public Proof.
(7) is_cli_active flag transition documented.
(8) emailHash identity bridge documented.
(9) IDE Process Watcher added as Phase 2 feature.


1.  Overview
The Proofboard CLI is a local-first Go binary that runs on the engineer's machine. It reads Git commit history from the local .git directory, classifies the work into categories, destroys all proprietary text before transmission, and sends only a clean cryptographic footprint to the Proofboard API.
The CLI is the product. OAuth is the fallback for engineers who cannot install it.

Property
Value
Language
Go
Licence
MIT — open source from Day 1
Binary type
Statically compiled — zero runtime dependencies
Minimum Go version
1.21
CLI entry point
proofboard
Core commands
auth  link  sync  stop  status  logs
Sync triggers
Post-merge hook (primary) + post-pull hook (secondary). No persistent background daemon.
API communication
HTTPS only — JWT-signed payloads
Data destroyed locally
Commit message text, file paths, raw repository name, author email (pre-hashing)
Data transmitted
SHA hashes, timestamps, numerical stats, algorithm-derived category labels, org hash, email hash


2.  Binary Distribution
The CLI ships as pre-compiled static binaries. No package manager dependency. No Node.js. Direct download or Homebrew tap.

2.1  Build Targets
Platform
Architecture
Binary Name
macOS
arm64 (Apple Silicon)
proofboard-darwin-arm64
macOS
x86_64 (Intel)
proofboard-darwin-amd64
Linux
x86_64
proofboard-linux-amd64
Windows
x86_64
proofboard-windows-amd64.exe


2.2  Distribution Mechanism
Channel
Spec
Direct download
Hosted at releases.proofboard.io/{version}/{binary-name}. OS-detected download link shown on CLI install screen in the web app.
Install script
curl -fsSL https://releases.proofboard.io/install.sh | sh — detects OS and architecture, downloads correct binary, places in /usr/local/bin.
Homebrew (macOS)
brew install proofboard/tap/proofboard — Phase 2.
apt/deb (Linux)
Phase 2. Not required at launch.
npx fallback
npx proofboard-cli auth — wraps the Go binary download. Available for engineers who prefer it. Not the primary path.


2.3  Code Signing
Platform
Requirement
macOS
Binary must be notarized via Apple Developer account. Unsigned binaries are blocked by Gatekeeper on macOS 10.15+. Notarization runs as a CI/CD pipeline step on every release.
Windows
Binary must be Authenticode-signed. Unsigned binaries trigger Windows SmartScreen on first run. Code signing certificate required before launch.
Linux
No OS-level signing requirement. SHA256 checksums published alongside each release for manual verification.


2.4  Update Mechanism
The CLI checks for a newer version on startup by calling GET https://releases.proofboard.io/latest.json. If a newer version exists, it prints a non-blocking notice: 'A new version of the Proofboard CLI is available. Run: proofboard update'. Auto-update without user confirmation is not implemented.

3.  Commands
Command
Behaviour
proofboard auth
Opens the engineer's default browser to https://app.proofboard.io/cli-auth. On completion, the web app writes a JWT to a callback URL. The CLI listens on local port 9876, stores the JWT in ~/.proofboard/credentials.json, and confirms authentication in the terminal.
proofboard link
Must be run inside a Git repository directory. Reads the remote URL, derives the org hash and repo hash, calls the API to register the repo. Prints the detected org name for confirmation.
proofboard sync
Runs the full eight-phase classification pipeline. Transmits the clean payload to the API.
proofboard sync --skip-handshake
Runs the pipeline without the Phase 6 handshake. For corporate VPN or SSH-proxy environments. Proof card marked 'handshake-skipped'.
proofboard stop
Removed in v1.2. The daemon no longer exists. Use proofboard unlink to remove hooks.
proofboard status
Prints the current sync state for all linked repositories: last sync time, tier achieved, pending sync.
proofboard logs
Prints the last 100 lines of the sync log at ~/.proofboard/sync.log.
proofboard update
Downloads and replaces the binary with the latest release for the current platform.
proofboard unlink
Removes the post-merge and post-rewrite hooks from .git/hooks/ and clears the repository entry from state.json.
proofboard watcher start
Phase 2 — IDE process watcher. Registers as a login item. See Section 5B.
proofboard watcher stop
Phase 2 — removes login item and terminates watcher process.
proofboard watcher status
Phase 2 — shows watcher state and configured IDEs.
proofboard config add-ide {name}
Phase 2 — adds a custom IDE process name to the watch list.


4.  The Eight-Phase Classification Pipeline
The pipeline runs entirely on the developer's local machine. Phases 1 through 5 run before any network communication. Phase 6 is the org handshake. Phases 7 and 8 are payload assembly and server countersigning.

Phase 1 — Local Ingest
Architectural decision — read first, shred after. The pipeline reads commit text locally in Phase 2 before destroying it in Phase 5. This is not an NDA violation. The NDA concern is about what leaves the machine. The Shredder fires in Phase 5 before any network call is made. That is what protects the NDA position.


Strict in-memory constraint: commit text must never touch disk outside .git at any point during the pipeline. Not logs. Not temp files. Not crash reports. Not swap. Memory only. All string variables holding commit subject lines must be explicitly zeroed and set to nil after Phase 2 scoring completes.

git log --format="%H|%ae|%at|%s" --numstat --no-merges --author=$(git config user.email)


Returns per commit: full SHA hash, author email, Unix timestamp, commit subject line (first line only), and per-file numerical statistics. The author filter scopes results to the authenticated developer's commits only.

For incremental syncs:

git log {last_sha}..HEAD --format="%H|%ae|%at|%s" --numstat --no-merges --author=$(git config user.email)


Phase 2 — Classification Engine
Two-layer hybrid classification. Both layers run locally. No commit text is transmitted at any stage.

Layer 1 — Semantic intent scoring: reads the raw commit subject line in memory, produces a confidence-weighted intent signal (feature / bugfix / refactor / ship / review / docs) and a noise score (0.0–1.0). The raw commit subject line string is explicitly zeroed and dereferenced immediately after the intent score is produced. It does not persist beyond this step under any circumstances.

Layer 2 — Rule-based category scoring: uses the Layer 1 intent score (numerical) and file path pattern match from --numstat output (+2 points per match, higher weight than keyword). Keyword matching runs here (+1 point per match) but only while the string is still in scope from Layer 1. By the end of Layer 2, the commit subject line is gone. Only the numerical category scores survive.

Category Dictionary
Category
Keyword Signals (sample)
Path Signals (sample)
Authentication & security
auth, login, jwt, mfa, rbac, token, encrypt, session
auth/, middleware/, guards/, security/
Frontend & UI
component, render, hook, state, redux, tailwind, layout
components/, pages/, .tsx, .jsx, .vue
API & backend services
api, endpoint, route, controller, graphql, rpc, handler
api/, services/, controllers/, routes/
Database & data
migration, schema, query, prisma, mongoose, redis, index
migrations/, models/, db/, repositories/
Infrastructure & DevOps
deploy, docker, k8s, ci, lambda, terraform, pipeline
infra/, .github/, Dockerfile, .yml
Performance & optimisation
cache, lazy, memo, bundle, lighthouse, throttle, perf
next.config, vite.config, webpack/
Payments & billing
payment, stripe, billing, subscription, invoice, webhook
payments/, billing/, subscriptions/
Testing & quality
test, spec, mock, jest, cypress, coverage, fixture
__tests__/, .test., .spec., e2e/


The dictionary is distributed as a versioned JSON file alongside the binary. See Section 8. Every classification result is tagged with the dictionary version used.

Phase 3 — Scoring Matrix
After processing all commits, scores are summed per category. The top four categories by score become the project's Contribution Areas. The highest-scoring category becomes the primary domain label. A single commit contributes to multiple categories simultaneously. Minimum threshold: a category must have a score of 3 or higher to qualify.

Phase 4 — Milestone Cluster Detection
PR merge commits mark natural cluster boundaries. Each merge commit defines the end of a milestone cluster.

Field
Value
clusterLabel
Dominant category name from the pre-approved dictionary vocabulary — not raw commit text
impactType
feature / bugfix / refactor / ship
scale
large (>30 commits) / medium (10–30 commits) / small (<10 commits)
commitCount
Integer
additionTotal
Total lines added across all commits in the cluster
deletionTotal
Total lines deleted across all commits in the cluster
durationDays
Days between first and last commit in the cluster
referenceShaBucket
Array of up to 3 representative SHA hashes from the cluster


Phase 5 — The Shredder
The Shredder is non-negotiable. It is what makes Proofboard NDA-compliant. Every item listed below must be destroyed before any network call is made. The CLI is open source — engineers and their legal teams can verify this.


In practice, by Phase 5 the commit subject line strings have already been zeroed at the end of Phase 2. Phase 5 is a mandatory second pass — a defensive guarantee, not the primary destruction step.

Item
Destruction Method
Commit subject line strings
Overwrite with zero bytes, then dereference. Runs even if already zeroed in Phase 2. No exceptions.
File path strings from --numstat
Drop and dereference. File paths may reveal internal project structure — treated as sensitive as commit messages.
Author email
Hash with SHA256(email.toLowerCase().trim()) — store hash only, dereference raw string.
Repository name and org name
Hash with SHA256(provider + ':' + orgName + '/' + repoName) — store hash only, dereference raw strings.


What survives the Shredder: SHA hashes, Unix timestamps, numerical statistics (additions, deletions, files changed), category labels from the pre-approved dictionary vocabulary, cluster metadata (impactType, scale, commitCount, duration, line totals), orgHash, emailHash, cliVersion, dictionaryVersion.

Phase 6 — Org Handshake
git ls-remote --exit-code origin HEAD


Forces a live network handshake with the remote repository host. If the handshake succeeds, proceed to Phase 7. If it fails within 10 seconds, see Section 6 for the full fallback specification.

Phase 7 — Clean Payload Assembly
All fields must be clean (post-Shredder). Payload is signed with the developer's Proofboard JWT before transmission.

Field
Type / Notes
shas
[]string — array of full SHA hex strings
timestamps
[]int64 — Unix timestamps
additions
[]int — per-commit additions
deletions
[]int — per-commit deletions
filesChanged
[]int — per-commit file counts
categories
[]string — from pre-approved dictionary vocabulary only
impactScores
map[string]int — feature/bugfix/refactor/ship counts
milestoneClusters
[]Cluster — see Phase 4 output fields
orgHash
string — SHA256 hash
emailHash
string — SHA256 hash
handshakeStatus
string — 'success' or 'skipped'
capturedAt
string — ISO 8601 timestamp
cliVersion
string — CLI binary version
dictionaryVersion
string — dictionary file version tag


Phase 7A — Outcome Summary Generation
The outcome summary is generated from category metadata only. Not from commit text. Not from file paths. No proprietary text from commit messages enters the summary generation step at any point in the chain.


The outcome summary is the one-sentence business description prefilled for the engineer in the Review and Confirm screen. The engineer reviews, edits, and approves it before it is stored.

Permitted Inputs for Summary Generation
Permitted Input
Example Value
Primary category label
Payments and Billing
Secondary category label
Authentication and Security
impactType
feature
scale
large
commitCount
67
durationDays
98
additionTotal
12847
deletionTotal
4320


Prohibited Inputs
Prohibited Input
Why
Commit subject line text
Destroyed in Phase 2 and Phase 5. Never transmitted. Cannot be used.
File path strings
Destroyed in Phase 5. Never transmitted. Cannot be used.
Repository name or org name
Stored as one-way hashes only. Plain text never transmitted.
Inferred technology names beyond dictionary vocabulary
The dictionary vocabulary defines the ceiling of specificity. Inferring Kubernetes from Infrastructure & DevOps is not permitted.
Client names, project names, or business metrics
The engineer adds specifics they are comfortable sharing. The prefill is intentionally generic.


Generation Prompt Constraint
Do not include company names, client names, internal project names, product names, or specific quantitative metrics in the generated summary. Write in generic professional language that describes the type, scale, and impact of the work without identifying the specific organisation, project, or client. The engineer will add specific context they are comfortable sharing. Your output is a safe starting point, not a specific claim.


Example
Source
Text
Prefill (from category metadata)
Built and delivered a large-scale payments feature with authentication integration over 14 weeks across 67 commits.
Engineer's optional edit
Rebuilt Acme Corp's payment retry logic, reducing transaction failures from 3% to 0.4% within two sprints. (Engineer added specifics — their choice.)


Phase 8 — Server Countersigning
The Proofboard API receives the JWT-signed payload, verifies the signature, validates the payload schema, runs anomaly detection (see Section 9), and countersigns with Proofboard's private key (ECDSA-SHA256). The signed receipt is stored. The developer's project is updated to SHA Proof tier. The proof card is regenerated.

5.  Sync Trigger Architecture
V1.2 change: The 30-minute polling daemon is removed entirely. The post-merge hook is the primary trigger. The post-pull hook is the secondary trigger covering web-UI merges. Together they provide complete coverage without a persistent process.


5.1  Trigger Overview
Trigger
Behaviour
post-merge hook (primary)
Fires immediately after any successful local git merge. Written to .git/hooks/post-merge on proofboard link.
post-pull hook (secondary)
Fires after git pull completes, including fast-forward merges. Detects new commits by comparing current HEAD against the last recorded HEAD SHA. Written to .git/hooks/post-rewrite.
proofboard sync (manual)
Engineer-initiated full sync. Processes the entire commit history. Bypasses all filters.


5.2  Post-Merge Hook
#!/bin/sh
proofboard sync --incremental --from-hook 2>/dev/null &


Written to .git/hooks/post-merge on proofboard link. The --from-hook flag signals the CLI to run the branch filter gate and pre-classification filter first. The & runs the process in the background so the engineer's terminal is not blocked.

5.3  Post-Pull Hook (Secondary Trigger)
#!/bin/sh
proofboard sync --incremental --from-hook 2>/dev/null &


Written to .git/hooks/post-rewrite on proofboard link. On every git pull completion, the CLI checks whether HEAD has moved since the last recorded sync SHA. If it has, and the current branch passes the branch filter gate, an incremental sync fires.

Why this matters: In team environments, most merges happen via the GitHub web UI. The engineer creates a PR, someone clicks Merge on GitHub, no local git merge command runs. Without the post-pull hook, the engineer's next git pull would fast-forward silently with no Proofboard capture. The post-pull hook catches this the moment the engineer pulls the updated code.


5.4  Branch Filter Gate
Property
Spec
Default watched branches
main, master, develop
Add a branch
proofboard config add-branch release
Remove a branch
proofboard config remove-branch develop
View current list
proofboard config branches
Check command
git rev-parse --abbrev-ref HEAD — returns current branch name for comparison
Behaviour on no match
Silent exit. Zero output. Zero network call.


⚠  Feature branch commits are noise. Only merges that land on the production branch represent completed, shippable work.


5.5  Pre-Classification Trivial Commit Filter
Abort condition
Definition
Single commit
Total new commits in the range is exactly 1.
Documentation only
Every file changed is a documentation file: .md, .txt, README, CHANGELOG, LICENSE, .rst. No source code changed.
High boilerplate noise
The Phase 2 Layer 1 AI noise score across all commits exceeds 0.85.
Revert-only range
All commits in the range are reverts of previous commits. Net effect is zero.


On abort: the CLI writes a single line to ~/.proofboard/sync.log: 'trivial merge skipped — [timestamp] — [repoHash]'. No notification fires. No payload transmits.

5.6  Incremental Sync
Both hooks call proofboard sync --incremental. The CLI reads the current HEAD SHA and compares it against the last-synced HEAD SHA stored in ~/.proofboard/state.json under lastSyncedHead[repoHash]. The pipeline runs only for the commit range between the two SHAs.

5.7  Last-Successful-Handshake Timestamp
Every successful Phase 6 handshake writes a timestamp to ~/.proofboard/state.json under lastHandshake[repoHash]. A SHA Proof can be generated from stored historical sync data even after repo access is revoked, provided a successful handshake occurred at least once during the claimed employment window.

5.8  Logging
All sync activity is written to ~/.proofboard/sync.log. Maximum file size 5MB, rotating. Log entries include: timestamp, repo hash (not name), trigger source, phase reached, outcome, error message if applicable. No commit text, file names, or repo names are written to the log.

5.9  Removing Hooks
proofboard unlink removes the post-merge and post-rewrite hooks from .git/hooks/ for the specified repository and clears the repository entry from state.json. Running proofboard link again re-registers all hooks.

5A.  Notification Architecture
Proofboard sends exactly three notifications. All are terminal-based or email. No push notifications. No WebSocket cards. No weekly report notifications. No quarterly snapshot notifications.
 
Milestone highlights are generated automatically from contribution data and held inside the application for the engineer to review at their own pace — no notification is ever sent about pending milestone highlights.


5A.1  New Project Detection Prompt
When the CLI detects a Git repository that has not been linked to Proofboard, it surfaces a prompt in the engineer's terminal. This is the only unsolicited terminal output the CLI produces outside of a sync event.

Proofboard — Git repository detected
 
Project: fintech-payments-api
Detected:
✓ Payment Processing
✓ API Development
✓ Authentication Systems
 
Add this project to your proofboard?
 
  y   Sync this project
  n   Not this project
  x   Never ask for this workspace


Option
Behaviour
y — Sync this project
Registers the repo via proofboard link and runs the eight-phase pipeline in the background. The Proof-of-Ship echo fires when complete.
n — Not this project
Dismisses for this session. Prompt surfaces again next time this workspace is opened.
x — Never ask for this workspace
Permanently suppresses for this directory. Written to ~/.proofboard/suppressed-workspaces.json. Never surfaces for this directory again.


⚠  The third option is required. Engineers have personal repos, tutorial projects, and throwaway experiments they do not want in their professional ledger. Without permanent suppression per workspace, the prompt becomes noise and engineers stop reading it.


5A.2  Proof-of-Ship Terminal Echo
When a sync completes and a qualifying milestone payload is transmitted, the CLI prints one line to the engineer's terminal.

✔  Proofboard: Milestone captured. Review at proofboard.io/dashboard


Nothing else fires at this moment. No email. No push notification. No WebSocket card. The engineer is already in their terminal — one line is enough.

Milestone highlights are generated automatically from the contribution data and held in the application. The engineer logs into the dashboard, reviews the pending highlights, approves or removes them. The rest are stored in the vault. No notification is sent about pending highlights.


5A.3  Monthly Career Summary
One email per month. No weekly report. No quarterly snapshot notification.

The email delivers the Monthly Career Summary — category breakdown for the month, trajectory shift, AI-generated narrative paragraph summarising the engineer's professional focus that month.

In addition, the next time the engineer opens any linked project in their terminal after the summary is generated, the CLI surfaces one quiet line:

Proofboard: Your March career summary is ready. proofboard.io/career-summary


Property
Spec
Cadence
Once per month — last Friday of each month
Channels
Email + one-time terminal line on next project open
Terminal trigger
First linked project the engineer opens after summary generation that month
Repeat behaviour
Fires once and clears. Does not repeat on subsequent project opens that month.
Link
proofboard.io/career-summary


The quarterly snapshot exists as a dashboard feature. The engineer can view all historical snapshots from the dashboard at any time. No notification is sent when a quarterly snapshot generates — the engineer finds it when they go looking for it.


5B.  IDE Process Watcher  (Phase 2)
Phase 2 feature. Build this when is_cli_active conversion data shows engineers complete onboarding but do not install the CLI. If the bottleneck is motivation this does not help. If the bottleneck is friction — engineers who would sync but do not remember to run the command — this solves it precisely. Validate CLI adoption rates before building.


The IDE process watcher is an extension to the Go binary that detects when an engineer opens a project in their IDE and surfaces a sync prompt at the exact right moment. It uses OS-level process watching, not filesystem polling. The binary is completely idle between IDE launches.

5B.1  Process Watching vs Filesystem Watching
The binary watches the system's running processes, not the filesystem. Filesystem watching is constant, resource-consuming, and triggers security concerns on corporate machines. Process watching is idle until an IDE launches. It listens for one specific event type and does nothing else. This is the same pattern used by Raycast, Fig, and other developer tools that respond to application events.

5B.2  OS-Level Implementation
OS
Mechanism
macOS
NSWorkspace notification API — listens for application launch events. Zero polling.
Linux
/proc filesystem monitoring or D-Bus signals — detects IDE process creation events.
Windows
WMI event subscriptions — watches for IDE executable creation events.


5B.3  Supported IDEs
IDE
Process Name(s)
VS Code
code, code-insiders
Cursor
cursor
WebStorm
webstorm
IntelliJ
idea
Vim / Neovim
vim, nvim
Zed
zed
Sublime Text
sublime_text
Custom
Engineer-configurable via: proofboard config add-ide {process-name}


Launch support for the watcher: VS Code and Cursor only. Add WebStorm and IntelliJ in Phase 2 based on demand. Adding a new IDE is a configuration entry, not a binary rebuild.


5B.4  Workspace Directory Detection
When an IDE launch event fires, the binary reads the active workspace directory from the IDE process arguments. VS Code and Cursor write the active workspace path to a predictable location in their process arguments. Once the workspace directory is identified, the binary checks for a .git folder. If found and the repo is not already linked and not in the suppression list, the detection prompt surfaces.

5B.5  The Detection Prompt
Surfaces as a native OS notification or directly in the IDE integrated terminal if one is open.

OS
Notification Mechanism
macOS
Native notification via UserNotifications framework. Appears in Notification Centre.
Linux
libnotify via notify-send. Standard desktop notification.
Windows
Windows Toast Notifications via the Windows Runtime API.


Proofboard — Git repository detected
 
Project: fintech-payments-api
Detected:
✓ Payment Processing
✓ API Development
✓ Authentication Systems
 
Add this project to your proofboard?
 
[ Sync this project ]   [ Not this project ]   [ Never ask for this workspace ]


Option
Behaviour
Sync this project
Registers the repo and runs the eight-phase pipeline in the background. Proof-of-Ship echo fires when complete.
Not this project
Dismisses for this session. Prompt surfaces again next time this workspace is opened in an IDE.
Never ask for this workspace
Permanently suppresses for this directory. Written to ~/.proofboard/suppressed-workspaces.json. Never surfaces again.


⚠  The permanent suppression option is required. Without it the prompt becomes noise and engineers stop engaging with it.


5B.6  The Flow
Step
What Happens
1
Engineer installs Proofboard binary (one curl command or direct download)
2
Binary registers as a lightweight background process on login
3
Engineer launches VS Code, Cursor, or any configured IDE
4
Binary detects IDE process launch via OS-level event (NSWorkspace / D-Bus / WMI)
5
Binary reads active workspace directory from IDE process arguments
6
.git folder found — repo not previously linked and not in suppression list
7
Detection prompt surfaces via native OS notification or integrated terminal
8
Engineer selects Sync this project
9
CLI runs eight-phase pipeline in the background — engineer continues working
10
Proof-of-Ship echo fires when first proof asset is ready


5B.7  New CLI Commands
Command
Behaviour
proofboard watcher start
Registers the IDE process watcher as a login item.
proofboard watcher stop
Removes the login item and terminates the watcher process.
proofboard watcher status
Shows whether the watcher is running and which IDEs are configured.
proofboard config add-ide {name}
Adds a custom IDE process name to the watch list.
proofboard config remove-ide {name}
Removes an IDE process name from the watch list.


5B.8  Why Not a VS Code Extension
Concern
Binary Process Watcher
IDE coverage
Works for every IDE from a single binary installation.
Vendor dependency
No Marketplace submission. No review policies. No vendor decisions.
IDE switches
Continues working when engineer moves from VS Code to Cursor — no reinstallation.
Corporate security
Less alarming than an extension with access to the full VS Code editor API surface.
NDA trust
Binary is open source and independently verifiable — same trust argument as the CLI pipeline.


5B.9  Interim Path Before Phase 2
Before building the IDE watcher, add the three-option workspace prompt to the existing post-merge hook output for repositories that have not been linked yet. If engineers engage with the in-terminal version, the watcher UX is validated before the Phase 2 engineering investment.

Proofboard — unlinked repository detected.
 
Project: fintech-payments-api
Detected:
✓ Payment Processing
✓ API Development
✓ Authentication Systems
 
Add this project to your proofboard?
 
  y   Sync this project
  n   Not this project
  x   Never ask for this workspace


6.  Skip-Handshake Fallback
The git ls-remote handshake (Phase 6) proves active repo access. On corporate machines with VPN-enforced SSH keys, internal proxy requirements, or network isolation, the handshake may fail even when the developer has legitimate access.

6.1  Failure Behaviour
Scenario
Behaviour
Handshake succeeds
Normal Tier 2 sync. handshakeStatus: 'success'.
Handshake times out (>10s) — no flag passed
CLI exits with error: 'Remote handshake timed out. If you are on a corporate network, retry with: proofboard sync --skip-handshake'. No data transmitted.
Handshake fails — --skip-handshake flag passed
Pipeline continues. handshakeStatus: 'skipped'. Proof card shows 'Handshake-skipped' label. Trust score adjustment applied.
No successful handshake on record for this repo
Even with --skip-handshake, if lastHandshake[repoHash] is null, CLI prints: 'No prior handshake recorded. A successful handshake must occur at least once during active employment.' Sync is blocked.


6.2  Trust Score Impact
When handshakeStatus is 'skipped', the cliVerified signal receives +10 instead of +20. The proof card shows 'SHA Proof — handshake skipped' rather than 'SHA Proof'.

6.3  Documentation Requirement
# HTTPS proxy
HTTPS_PROXY=https://proxy.company.com:8080 proofboard sync
 
# SSH proxy via ProxyCommand
# Add to ~/.ssh/config:
# Host github.com
#   ProxyCommand nc -X connect -x proxy.company.com:8080 %h %p


7.  Authentication & JWT
7.1  Auth Flow
Engineer runs proofboard auth. CLI opens https://app.proofboard.io/cli-auth?port=9876 in the default browser. Engineer authenticates via GitHub or GitLab OAuth. On success, the web app calls back to http://localhost:9876/callback?token={jwt}. CLI receives the JWT, stores it at ~/.proofboard/credentials.json with 0600 permissions, and prints: 'Authenticated as {username}. Run proofboard link inside a repository to get started.'

EmailHash Identity Bridge
During proofboard auth, the CLI reads the local Git configuration email (git config user.email), hashes it with SHA256(email.toLowerCase().trim()), and includes this hash in the authentication handshake. The server matches this hash against the emailHash from the engineer's OAuth signup record. This links the CLI session to the correct user account.

Critical: An engineer who signs up via the web app and later installs the CLI must end up on the same user record. The emailHash bridge ensures this. If the CLI email and the OAuth email differ, the server flags the mismatch and prompts the engineer to update their git config email or add the email at app.proofboard.io/settings/emails.


7.2  Credentials File
Field
Notes
token
JWT — signed by Proofboard API. Contains: sub (user ID), exp (expiry), scope (cli).
username
Stored for display purposes only.
refreshToken
Used to obtain a new JWT when the current one expires.


7.3  Payload Signing
Every payload transmitted in Phase 7 is signed with the stored JWT as a Bearer token in the Authorization header. An expired or invalid JWT causes the sync to fail with a human-readable error instructing the developer to re-run proofboard auth.

8.  Dictionary Update Mechanism
Property
Spec
Distribution
Hosted at releases.proofboard.io/dictionary/{version}/dictionary.json
Version check
On CLI startup, GET releases.proofboard.io/dictionary/latest.json returns the current version string. Compared against locally stored version in ~/.proofboard/dictionary-version.
Update behaviour
If newer version exists, downloads to temp file, validates JSON schema, then atomically replaces ~/.proofboard/dictionary.json.
High-security opt-out
proofboard config set auto-update-dictionary false. Manual update via proofboard update-dictionary.
Transparency
Every classification result transmitted includes dictionaryVersion. Same commit history + same dictionary version = same categories.
Fallback
If CDN is unreachable, uses locally cached dictionary. Sync is not blocked by a dictionary update failure.


9.  Anti-Fraud Signals from CLI
The CLI produces signals that feed into the server-side anomaly detection system. The CLI does not make fraud decisions — it captures and transmits the signals.

Signal
How CLI captures it
orgHashMismatch
If the org hash derived from the Git remote URL does not match the org hash stored during proofboard link, the sync payload includes orgHashMismatch: true. Applied penalty: -40 to Internal Trust Score.
identityMismatch
If the author email hash on any commit does not match the authenticated account's email hash, the payload flags the mismatched count. Applied penalty: -30.
singleCommitRepoCap
If total commit count is below 5 at sync time, the payload includes lowCommitCount: true. Server caps the Internal Trust Score.
boilerplateSignal
The AI noise score from Phase 2. Values above 0.8 indicate uniform or trivial commit patterns. Transmitted as aiNoiseScore.
handshakeStatus
Transmitted as 'success' or 'skipped'. Affects cliVerified bonus in the trust score.


10.  Weekly Progress Report — Removed
The Weekly Progress Report is discontinued. The only proactive outreach Proofboard sends is the Monthly Career Summary. Weekly reports generated notification noise without proportional value. The data they surfaced — commit activity, Reputation Index delta — is visible at any time on the engineer's dashboard. Engineers check it when they want it, not when Proofboard pushes it.


11.  Post-Employment Log Parser — Wasm Hybrid
For engineers who have already left a company and lost repository access, a web-based log upload path is available.

11.1  The Problem
Engineers who have lost repo access cannot run proofboard sync — the Phase 6 handshake will fail with no prior handshake on record. However, they may have saved a raw git log dump from their work machine before returning it.

11.2  Implementation Split
The Wasm module handles Phase 5 only: shredding, hashing, and tokenisation. It does NOT implement the full classification pipeline. Category mapping runs server-side on the anonymised token structures the Wasm module produces.


Component
Responsibility
Browser (Wasm)
Accept raw git log --numstat text file via drag-and-drop. Run Phase 5 Shredder: overwrite all commit subject line strings with zero bytes, hash author email, hash repo name, drop file path strings. Produce anonymised token structures. Transmit the clean token payload to the API.
Server
Receive the anonymised token payload. Run Phases 2–4 (classification, scoring, cluster detection). Run Phase 8 (countersigning). Issue a SHA Proof marked as 'Log-attested'.


11.3  Proof Card Label
Log-attested proofs display as 'SHA Proof — log attested' on the proof card. The trust score range is the same as standard Tier 2 (60–90) but the label distinction is permanent and cannot be upgraded to a live-handshake proof after the fact.

11.4  Accepted Input Format
git log --format="%H|%ae|%at|%s" --numstat --no-merges


No other format is accepted. If the file fails to parse against this schema, the upload is rejected with a clear error message showing the engineer the exact command to run.

⚠  The Wasm module is a separate deliverable for the web team — not in CLI sprint scope. CLI team's responsibility is limited to defining the Phase 5 Shredder logic so the web team can implement it in Wasm following the same specification.


12.  Tier 3 Scope — Decision Locked
Tier 3 (Human Attestation) is not CLI scope. This decision is locked. The CLI handles Tier 1 and Tier 2 only.


Tier 3 is a web-based employer verification portal. The flow is entirely server-side and web-based. The CLI is not involved. No employer portal. No HR notification system. No Tier 3 CLI commands.

13.  NDA and Compliance Position
13.1  What the CLI Never Stores or Transmits
Category
Detail
Commit message text
Any form, any length — never stored, never transmitted
File names or directory paths
Never stored, never transmitted
Code diffs or file content
Never stored, never transmitted
Repository names
Stored and transmitted as one-way SHA256 hashes only
Organisation names
Stored as one-way hashes. Display name sourced from public API only.
Author email addresses
Stored and transmitted as SHA256 hashes only
README, PR descriptions, issue text
Never processed, never stored, never transmitted


13.2  What the CLI Stores and Transmits
Category
Data
Cryptographic evidence
SHA hash strings (one-way). Unix timestamps.
Numerical statistics
Additions, deletions, files changed per commit.
Classification output
Algorithm-derived category labels from pre-approved vocabulary only.
Milestone cluster metadata
Category, impact type, scale, commit count, duration, line totals. No commit message text.
Attestation
orgHash, emailHash, handshakeStatus, cliVersion, dictionaryVersion.


13.3  The Legal Position
Proofboard operates a local-first architecture. No company data is transmitted to Proofboard's servers. The CLI runs a pattern-matching algorithm on the developer's own machine. The algorithm produces numerical scores and pre-defined category labels from a public vocabulary. Only these outputs — equivalent to a developer writing on their CV that they worked on authentication and API systems — are transmitted. The commit messages, file names, and code content that would constitute proprietary IP are destroyed on the developer's machine before any network communication occurs. SHA hashes transmitted are one-way cryptographic fingerprints that cannot be reversed to reveal any business information.


14.  Open Source Requirements
The CLI must be open source from Day 1. Engineers and their legal teams must be able to verify that the Shredder works as described. A closed-source CLI is just another terms of service.


Requirement
Spec
Licence
MIT
Repository
github.com/proofboard/cli — public from the first commit
README
Must include: what the CLI does, what it transmits, what it destroys, how to audit Phase 5, and the proxy/VPN configuration guide
Phase 5 audit guide
A dedicated SHREDDER.md document explaining exactly which strings are destroyed and in which order, with code pointers to the relevant functions
Dictionary
The category dictionary JSON file must be in the repository and versioned with the code
CI/CD transparency
GitHub Actions workflow files must be public so engineers can verify the build pipeline for distributed binaries
Checksums
SHA256 checksums for all distributed binaries published alongside each release in releases.proofboard.io/{version}/checksums.txt


15.  Implementation Priorities
The CLI ships at launch. Not Phase 2. The launch cohort's first impression depends on engineers getting SHA Proof scores on Day 1. Volume Proof OAuth-only scores (25–55) will kill early retention if the CLI is not available at launch.


Sprint 1 — Core Pipeline
Phase 1: git log ingest with author filter. Phase 2: classification engine — both layers. Phase 3: scoring matrix and Contribution Areas. Phase 4: milestone cluster detection. Phase 5: The Shredder — all destruction in correct order. Phase 6: git ls-remote handshake + --skip-handshake fallback. Phase 7: clean payload assembly and JWT signing. Phase 7A: outcome summary generation. Phase 8: API transmission and receipt handling. proofboard auth command. proofboard link command. proofboard sync command.

Sprint 2 — Hooks & Distribution
Post-merge hook and post-pull hook. Branch filter gate. Pre-classification trivial commit filter. Incremental sync. Last-successful-handshake timestamp storage. New project detection prompt (three-option terminal output). Proof-of-Ship terminal echo. proofboard status, logs, unlink commands. Binary builds for all four targets — macOS arm64, macOS amd64, Linux amd64, Windows amd64. Code signing — macOS notarization, Windows Authenticode. Install script at releases.proofboard.io/install.sh. GitHub Actions pipeline for automated cross-platform builds on tag push.

Sprint 3 — Polish & Audit
proofboard update command. Dictionary update mechanism — CDN check, atomic replace, opt-out config. Monthly Career Summary terminal trigger — surfaces once per month on first linked project open after summary generation. SHREDDER.md documentation. Proxy configuration guide in README. SHA256 checksums published alongside releases. Anti-fraud signals: orgHashMismatch, identityMismatch, aiNoiseScore. Logging to ~/.proofboard/sync.log with rotation.

Phase 2 — IDE Process Watcher
Build only after is_cli_active conversion data validates the need. See Section 5B for the full specification. Do not build before Sprint 3 is complete and adoption metrics are available.

Appendix A:  CLI Command Reference
Command
Flag / Notes
proofboard auth
No flags. Opens browser for OAuth.
proofboard link
Must be run inside a Git repo. Registers the repo.
proofboard sync
--incremental (delta only) | --skip-handshake (corporate environments) | --verbose (prints pipeline steps)
proofboard status
No flags. Prints state for all linked repos.
proofboard logs
--lines N (default 100). Prints sync log.
proofboard update
No flags. Downloads latest binary.
proofboard update-dictionary
No flags. Downloads latest dictionary.
proofboard config
set auto-update-dictionary true|false | add-branch {name} | remove-branch {name} | branches | add-ide {name} | remove-ide {name}
proofboard unlink
Removes the linked repo from state. Does not delete proof assets from the server.
proofboard watcher start
Phase 2. Registers IDE process watcher as a login item.
proofboard watcher stop
Phase 2. Removes login item and terminates watcher process.
proofboard watcher status
Phase 2. Shows watcher state and configured IDEs.


Appendix B:  State File Schema
Stored at ~/.proofboard/state.json:

{
  "linkedRepos": {
    "{repoHash}": {
      "lastHeadSha": "string",
      "lastSyncAt": "ISO8601",
      "lastHandshake": "ISO8601",
      "tier": "Tier2 | Tier2-skipped | Tier1",
      "dictionaryVersion": "string"
    }
  },
  "suppressedWorkspaces": [
    "/path/to/workspace"
  ],
  "monthlyCareerSummaryShown": {
    "2026-03": true
  }
}


Appendix C:  Anti-Fraud Signal Reference
Signal
Source
Server Action
orgHashMismatch
CLI derives org hash from git remote URL. Mismatch with registered hash sets flag.
-40 to Internal Trust Score
identityMismatch
Author email hash on commits compared to authenticated account email hash.
-30 per mismatch event
singleCommitRepoCap
Commit count below 5 at sync time.
Score capped regardless of other signals
aiNoiseScore
Layer 1 classification noise score. High score (>0.8) indicates boilerplate patterns.
High score → -20 boilerplate penalty server-side
handshakeStatus
'success' or 'skipped'
Skipped: cliVerified bonus reduced from +20 to +10


PROOFBOARD CLI SPEC — v1.4
Build it. Open source it. Ship it at launch.


---

# API Docs
https://api-dev.proofboard.io/docs


---


# Implement docs similar to the claud docs, create a docs folder with index.md and all other supporting pages linked, indexed and properly sectioned and updated with code changes.

https://code.claude.com/docs/en/terminal-guide


---

# 
