Updated spec:

# Proofboard Career Agent — Product & UX Override (v1.8.16)

This section supersedes older user-facing terminology and the earlier “no persistent process” decision below. The open-source Go executable and NDA-safe eight-phase pipeline remain the implementation; the product presented to engineers is **Proofboard Career Agent**.

**Promise:** Proofboard builds your career while you focus on building software.

## Product Model

The engineer installs the Career Agent and writes code. The local agent quietly detects repositories, authenticates when needed, connects projects, synchronizes meaningful activity, updates career records, and generates engineering proof. Existing `proofboard auth`, `link`, and `sync` commands remain supported for advanced users, automation, debugging, scripting, and CI/CD, but are not required concepts in the primary experience.

## Installation

The recommended website action is **Install Proofboard Career Agent**. Installation must install the executable, add it to `PATH`, register the platform background service, and start it automatically without requiring a terminal. Releases therefore include a Linux amd64 `.deb`, macOS amd64 and arm64 `.pkg` installers, and a Windows amd64 setup `.exe` in addition to the four explicit static binaries. The website must choose the matching native installer; shell scripts and bare binaries are power-user surfaces. Power-user channels may include `brew install proofboard` and `npm install -g @proofboard/agent`.

The background registration mechanisms are a systemd user service or desktop autostart entry on Linux, a LaunchAgent on macOS, and an at-logon scheduled task on Windows.

## Authentication and Sessions

Synchronization checks credentials automatically. When no usable session exists, the agent creates a temporary device code and opens:

`https://proofboard.io/agent/cli-auth?code=<generated_code>`

The code is prefilled. After the user authorizes in the browser, the agent securely stores access and refresh tokens and resumes the interrupted project sync. A valid refresh token is used silently. If the complete session has expired, the user sees **Your Proofboard session has expired** with a **Reconnect** action.

## Repository Detection and Sync Project

The Career Agent watches supported IDE processes and workspace arguments. A newly opened, unsuppressed Git repository surfaces:

> New repository detected.
> Would you like Proofboard to track this project?

Actions:

- **Sync Project** — creates a project if necessary, connects the repository, performs its first sync, installs activity hooks, and begins continuous tracking.
- **Not Now** — dismisses for the current agent session.
- **Never Ask Again** — stores the local workspace suppression and does not prompt again.

Already tracked repositories do not prompt. If their HEAD or one-way local repository-metadata fingerprint differs from the last successful sync, the agent synchronizes them automatically. The metadata fingerprint covers provider/org/repository hashes, remote refs, and the default remote branch; raw names are never persisted or transmitted. Post-commit, post-merge, and post-rewrite hooks provide additional event-driven synchronization.

## Milestones and Status

Detected milestones use the title **Milestone detected** and expose **Review**, **Publish**, and **Ignore** actions. When the backend supplies a milestone bundle ID, Publish approves the bundle and Ignore declines it through the milestone-bundle API. Review opens that bundle in the dashboard. A locally detected milestone without a bundle ID routes Review/Publish to the dashboard until asynchronous bundle creation completes.

The status surface uses Career Agent terminology and includes:

- Active/stopped local state
- Last sync
- Number of repositories tracked
- Authentication status

## Privacy Messaging

User-facing surfaces should consistently state:

- Runs entirely on your machine.
- No proprietary source code leaves your computer.
- No employer access required.
- Designed to preserve NDA-safe engineering proof.
- Builds structured engineering proof without exposing confidential code.

The Phase 5 Shredder and all transmission allowlists below remain non-negotiable.

---

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
curl -fsSL https://releases.proofboard.io/install.sh | sh — detects OS and architecture, downloads correct binary, and installs it into the current account (~/.local/bin, or %LOCALAPPDATA%\Programs\Proofboard on Windows) so no administrator access is required. Set PROOFBOARD_SYSTEM_INSTALL=1 for a machine-wide install into /usr/local/bin.
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

Rewiew the endpoints and integrate them into the cli as needed, from authenticationto everythign else. if anything seems to be missing, open a PR for it here https://github.com/Proofboard-inc/proofboard-backend , you can use the git or gh cli. Document al the prs and issues you open somewhere.

```json
{
  "openapi": "3.0.0",
  "paths": {
    "/api/v1/notifications": {
      "get": {
        "description": "Returns notifications with rich meta. Each notification contains actionUrl for navigation and meta with type-specific data.",
        "operationId": "NotificationsController_getMyNotifications",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "isRead",
            "required": false,
            "in": "query",
            "schema": {
              "type": "boolean"
            }
          },
          {
            "name": "type",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "proposal_viewed",
                "proposal_accepted",
                "proposal_declined",
                "project_verified",
                "project_rejected",
                "review_received",
                "message_received",
                "payment_received",
                "github_connected",
                "vcs_activity",
                "proof_of_ship_detected",
                "milestone_bundle_approved",
                "milestone_bundle_ready",
                "proofboard_viewed",
                "vcs_sync_completed",
                "proof_asset_declined",
                "proof_asset_confirmed",
                "payment_verified",
                "reputation_milestone",
                "client_verification_completed",
                "dealboard_deployment_activated",
                "dealboard_analytics_expired",
                "dealboard_access_locked",
                "dealboard_high_intent",
                "payment_completed",
                "plan_upgraded",
                "plan_downgraded",
                "subscription_payment_failed"
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my notifications (paginated)",
        "tags": [
          "Notifications"
        ]
      }
    },
    "/api/v1/notifications/unread-count": {
      "get": {
        "operationId": "NotificationsController_getUnreadCount",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get unread notification count",
        "tags": [
          "Notifications"
        ]
      }
    },
    "/api/v1/notifications/{id}/read": {
      "patch": {
        "operationId": "NotificationsController_markRead",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "description": "Notification ID",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Mark a single notification as read",
        "tags": [
          "Notifications"
        ]
      }
    },
    "/api/v1/notifications/mark-all-read": {
      "patch": {
        "operationId": "NotificationsController_markAllRead",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Mark all notifications as read",
        "tags": [
          "Notifications"
        ]
      }
    },
    "/api/v1/activity-log": {
      "get": {
        "operationId": "ActivityLogController_getMyActivityLog",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "type",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "vcs_connected",
                "vcs_disconnected",
                "vcs_sync_completed",
                "vcs_sync_deferred",
                "project_created",
                "project_updated",
                "project_github_verified",
                "project_client_verified",
                "project_rejected",
                "project_flagged",
                "project_deleted",
                "milestone_bundle_created",
                "milestone_bundle_approved",
                "milestone_bundle_declined",
                "client_verification_sent",
                "client_verification_completed",
                "client_verification_declined",
                "reputation_increased",
                "reputation_decreased",
                "reputation_frozen",
                "reputation_changed",
                "proofboard_created",
                "proofboard_viewed",
                "profile_updated",
                "certification_added",
                "certification_verified",
                "certification_expired",
                "user_registered",
                "plan_upgraded",
                "vcs_synced"
              ]
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my activity log (paginated)",
        "tags": [
          "Activity Log"
        ]
      }
    },
    "/api/v1/users/me": {
      "get": {
        "operationId": "UsersController_getMe",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get current user profile",
        "tags": [
          "Users"
        ]
      },
      "patch": {
        "operationId": "UsersController_updateProfile",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateUserDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update my profile (name, title, bio, location, socialLinks, serviceType, username)",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/users/check-username": {
      "get": {
        "operationId": "UsersController_checkUsername",
        "parameters": [
          {
            "name": "username",
            "required": true,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Check if a username is available (used as-you-type on profile page)",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/users/{username}/public": {
      "get": {
        "operationId": "UsersController_getPublicProfile",
        "parameters": [
          {
            "name": "username",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get public profile by username (for proofboard.app/username)",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/users/me/avatar": {
      "patch": {
        "operationId": "UsersController_uploadAvatar",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload avatar",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/users/me/cover": {
      "patch": {
        "operationId": "UsersController_uploadCover",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload cover image",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/users/me/notification-preferences": {
      "patch": {
        "operationId": "UsersController_updateNotificationPreferences",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateNotificationPreferencesDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update notification preferences",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/users/me/linked-accounts": {
      "get": {
        "description": "Returns all active linked accounts (GitHub, GitLab, Bitbucket) added after onboarding. Tokens are never returned.",
        "operationId": "LinkedAccountsController_getLinkedAccounts",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "List all connected secondary VCS accounts",
        "tags": [
          "Linked Accounts"
        ]
      }
    },
    "/api/v1/users/me/linked-accounts/{provider}/connect": {
      "post": {
        "description": "Returns a redirect URL. The frontend opens this in a popup/redirect to start the OAuth flow. Call GET /users/me/linked-accounts after the callback completes to confirm the link.",
        "operationId": "LinkedAccountsController_initiateConnect",
        "parameters": [
          {
            "name": "provider",
            "required": true,
            "in": "path",
            "schema": {
              "enum": [
                "github",
                "gitlab",
                "bitbucket"
              ],
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Initiate OAuth to link a secondary VCS account",
        "tags": [
          "Linked Accounts"
        ]
      }
    },
    "/api/v1/users/me/linked-accounts/{linkedAccountId}": {
      "delete": {
        "description": "Soft-disconnects the account (revokes token, marks as disconnected). Projects imported from this account are unaffected.",
        "operationId": "LinkedAccountsController_unlinkAccount",
        "parameters": [
          {
            "name": "linkedAccountId",
            "required": true,
            "in": "path",
            "description": "ID of the linked account document",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Disconnect a secondary VCS account",
        "tags": [
          "Linked Accounts"
        ]
      }
    },
    "/api/v1/auth/{provider}/link/callback": {
      "get": {
        "operationId": "LinkedAccountsCallbackController_handleLinkCallback",
        "parameters": [
          {
            "name": "provider",
            "required": true,
            "in": "path",
            "schema": {
              "enum": [
                "github",
                "gitlab",
                "bitbucket"
              ],
              "type": "string"
            }
          },
          {
            "name": "code",
            "required": true,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "state",
            "required": true,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "OAuth callback for linking a secondary VCS account",
        "tags": [
          "Linked Accounts"
        ]
      }
    },
    "/api/v1/projects": {
      "post": {
        "operationId": "ProjectsController_create",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateProjectDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Create a manual proof asset",
        "tags": [
          "Projects"
        ]
      },
      "get": {
        "operationId": "ProjectsController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "status",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "draft",
                "in_progress",
                "completed"
              ]
            }
          },
          {
            "name": "verificationStatus",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "unverified",
                "pending_client",
                "pending_admin",
                "pending_automated_review",
                "verified",
                "rejected",
                "draft",
                "processing",
                "github_verified",
                "vouched",
                "flagged"
              ]
            }
          },
          {
            "name": "projectType",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "client_work",
                "open_source",
                "personal",
                "internal_tool"
              ]
            }
          },
          {
            "name": "proofSourceType",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "manual",
                "vcs",
                "work_log",
                "api_spec",
                "codebase_upload"
              ]
            }
          },
          {
            "name": "industry",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "ownerId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "createdAt",
                "name",
                "views",
                "completedAt",
                "internalTrustScore"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "List my proof assets (paginated, filterable)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/stats": {
      "get": {
        "operationId": "ProjectsController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my proof asset statistics",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/{id}": {
      "patch": {
        "operationId": "ProjectsController_update",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateProjectDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update a proof asset (name, story, impactSignal, paymentVerified, etc.)",
        "tags": [
          "Projects"
        ]
      },
      "delete": {
        "operationId": "ProjectsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Soft-delete a proof asset",
        "tags": [
          "Projects"
        ]
      },
      "get": {
        "operationId": "ProjectsController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get a single proof asset by ID",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/vcs/repos": {
      "get": {
        "operationId": "ProjectsController_listVcsRepos",
        "parameters": [
          {
            "name": "provider",
            "required": true,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "github",
                "gitlab",
                "bitbucket"
              ]
            }
          },
          {
            "name": "linkedAccountId",
            "required": false,
            "in": "query",
            "description": "ID of a linked VCS account to list repos from. Omit to use the primary connection (falls back to linked accounts if primary is not set).",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "List repos from a VCS provider (GitHub/GitLab/Bitbucket)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/vcs/import": {
      "post": {
        "description": "Status starts as PROCESSING. Advances to GITHUB_VERIFIED after AI scan, or REJECTED if boilerplate detected.",
        "operationId": "ProjectsController_importFromVcs",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/VcsImportDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Import a repo from GitHub/GitLab/Bitbucket — queues AI scan",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/work-log": {
      "post": {
        "operationId": "ProjectsController_createFromWorkLog",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/WorkLogProofDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Create proof asset from work log description (backend engineers)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/api-spec": {
      "post": {
        "operationId": "ProjectsController_createFromApiSpec",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ApiSpecProofDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Create proof asset from API spec (Postman/OpenAPI/endpoint list)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/codebase-upload": {
      "post": {
        "description": "For engineers who bundle their codebase (e.g. via the bundler tool). Goes to PENDING_ADMIN for review.",
        "operationId": "ProjectsController_createFromCodebaseUpload",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "multipart/form-data": {
              "schema": {
                "$ref": "#/components/schemas/CodebaseUploadMetaDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload a codebase file (txt/zip) for manual admin verification",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/{id}/cover": {
      "patch": {
        "operationId": "ProjectsController_uploadCover",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload cover image",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/{id}/images": {
      "patch": {
        "operationId": "ProjectsController_uploadImages",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload additional images (max 5)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/{id}/proof-files": {
      "patch": {
        "operationId": "ProjectsController_uploadProofFiles",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload proof files (triggers admin verification queue)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/milestone-bundles": {
      "get": {
        "operationId": "ProjectsController_getMilestoneBundles",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "status",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "pending",
                "approved",
                "declined"
              ]
            }
          },
          {
            "name": "projectId",
            "required": false,
            "in": "query",
            "description": "Filter bundles for a specific project ID",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my pending milestone bundles (from PR merges)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/milestone-bundles/{id}/approve": {
      "post": {
        "operationId": "ProjectsController_approveBundle",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ApproveBundleDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Approve a milestone bundle — creates a verified proof asset",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/milestone-bundles/{id}/decline": {
      "post": {
        "operationId": "ProjectsController_declineBundle",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Decline a milestone bundle — data stays in Technical Ledger",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/projects/milestone-bundles/{id}": {
      "patch": {
        "operationId": "ProjectsController_updateBundle",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateBundleDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Edit a milestone bundle — update title, summary, tools, category (owner only)",
        "tags": [
          "Projects"
        ]
      }
    },
    "/api/v1/admin/projects/stats": {
      "get": {
        "operationId": "AdminProjectsController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Project stats: total, verified, pending, by type",
        "tags": [
          "Admin — Projects"
        ]
      }
    },
    "/api/v1/admin/projects/pending-verification": {
      "get": {
        "operationId": "AdminProjectsController_getPending",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List projects pending admin verification (oldest first)",
        "tags": [
          "Admin — Projects"
        ]
      }
    },
    "/api/v1/admin/projects": {
      "get": {
        "operationId": "AdminProjectsController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "status",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "draft",
                "in_progress",
                "completed"
              ]
            }
          },
          {
            "name": "verificationStatus",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "unverified",
                "pending_client",
                "pending_admin",
                "pending_automated_review",
                "verified",
                "rejected",
                "draft",
                "processing",
                "github_verified",
                "vouched",
                "flagged"
              ]
            }
          },
          {
            "name": "projectType",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "client_work",
                "open_source",
                "personal",
                "internal_tool"
              ]
            }
          },
          {
            "name": "proofSourceType",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "manual",
                "vcs",
                "work_log",
                "api_spec",
                "codebase_upload"
              ]
            }
          },
          {
            "name": "industry",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "ownerId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "createdAt",
                "name",
                "views",
                "completedAt",
                "internalTrustScore"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          },
          {
            "name": "verifiedBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "hasReview",
            "required": false,
            "in": "query",
            "schema": {
              "type": "boolean"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List all projects (filter by ownerId, status, verificationStatus, type, industry, date range)",
        "tags": [
          "Admin — Projects"
        ]
      }
    },
    "/api/v1/admin/projects/{id}": {
      "get": {
        "operationId": "AdminProjectsController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Get a single project by ID",
        "tags": [
          "Admin — Projects"
        ]
      },
      "delete": {
        "operationId": "AdminProjectsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Soft-delete a project",
        "tags": [
          "Admin — Projects"
        ]
      }
    },
    "/api/v1/admin/projects/{id}/approve": {
      "patch": {
        "operationId": "AdminProjectsController_approve",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Approve project verification",
        "tags": [
          "Admin — Projects"
        ]
      },
      "post": {
        "operationId": "AdminController_approveProject",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ReviewProjectDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Approve a project verification (sets GITHUB_VERIFIED)",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/projects/{id}/reject": {
      "patch": {
        "operationId": "AdminProjectsController_reject",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/RejectProjectDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Reject project verification with reason",
        "tags": [
          "Admin — Projects"
        ]
      },
      "post": {
        "operationId": "AdminController_rejectProject",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ReviewProjectDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Reject a project verification",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/proofboards/me": {
      "get": {
        "operationId": "ProofboardsController_getMyBoard",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get the authenticated user's proofboard (owner view with full data)",
        "tags": [
          "Proofboard"
        ]
      },
      "patch": {
        "operationId": "ProofboardsController_updateMyBoard",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateProofboardDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update proofboard — bio, headline, skills, tools, visibility, theme",
        "tags": [
          "Proofboard"
        ]
      }
    },
    "/api/v1/proofboards/me/stats": {
      "get": {
        "operationId": "ProofboardsController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get proofboard stats (4 cards): total views, company domains, recruiter views, new this week",
        "tags": [
          "Proofboard"
        ]
      }
    },
    "/api/v1/proofboards/me/analytics": {
      "get": {
        "operationId": "ProofboardsController_getAnalytics",
        "parameters": [
          {
            "name": "days",
            "required": true,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Full proofboard analytics (views by day, top sources, top countries, corporate domains)",
        "tags": [
          "Proofboard"
        ]
      }
    },
    "/api/v1/proofboards/me/featured": {
      "patch": {
        "operationId": "ProofboardsController_updateFeatured",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateFeaturedProjectsDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Set featured projects on proofboard",
        "tags": [
          "Proofboard"
        ]
      }
    },
    "/api/v1/proofboards/u/{username}": {
      "get": {
        "description": "This is the public-facing URL: proofboard.io/u/<username> or proofboard.io/<username>",
        "operationId": "ProofboardsController_findByUsername",
        "parameters": [
          {
            "name": "username",
            "required": true,
            "in": "path",
            "description": "The user's unique username",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "View a proofboard by username (public — increments view count)",
        "tags": [
          "Proofboard"
        ]
      }
    },
    "/api/v1/proofboards/me/interactions/{projectId}": {
      "post": {
        "operationId": "ProofboardsController_recordInteraction",
        "parameters": [
          {
            "name": "projectId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Record a visitor interaction on a project card",
        "tags": [
          "Proofboard"
        ]
      }
    },
    "/api/v1/admin/proofboards/stats": {
      "get": {
        "operationId": "AdminProofboardsController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Proofboard stats: total, public, private, custom domains",
        "tags": [
          "Admin — Proofboards"
        ]
      }
    },
    "/api/v1/admin/proofboards": {
      "get": {
        "operationId": "AdminProofboardsController_findAll",
        "parameters": [
          {
            "name": "ownerId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "visibility",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "public",
                "private"
              ]
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "default": 1,
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "default": 20,
              "type": "number"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List all proofboards (filter by ownerId, visibility, isDefault, date range)",
        "tags": [
          "Admin — Proofboards"
        ]
      }
    },
    "/api/v1/admin/proofboards/{id}": {
      "get": {
        "operationId": "AdminProofboardsController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Get a proofboard by ID",
        "tags": [
          "Admin — Proofboards"
        ]
      },
      "delete": {
        "operationId": "AdminProofboardsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Soft-delete a proofboard",
        "tags": [
          "Admin — Proofboards"
        ]
      }
    },
    "/api/v1/auth/register": {
      "post": {
        "operationId": "AuthController_register",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/RegisterDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/login": {
      "post": {
        "operationId": "AuthController_login",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/LoginDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/me": {
      "get": {
        "operationId": "AuthController_getMe",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/verify-email": {
      "post": {
        "operationId": "AuthController_verifyEmail",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/VerifyEmailDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/resend-otp": {
      "post": {
        "operationId": "AuthController_resendOtp",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ResendOtpDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/onboarding/step-1": {
      "patch": {
        "operationId": "AuthController_onboardingStep1",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/OnboardingStep1Dto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/onboarding/step-2": {
      "patch": {
        "operationId": "AuthController_onboardingStep2",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/OnboardingStep2Dto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/onboarding/complete": {
      "patch": {
        "operationId": "AuthController_completeOnboarding",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/forgot-password": {
      "post": {
        "operationId": "AuthController_forgotPassword",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ForgotPasswordDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/reset-password": {
      "post": {
        "operationId": "AuthController_resetPassword",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ResetPasswordDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/google": {
      "get": {
        "operationId": "AuthController_googleAuth",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/google/callback": {
      "get": {
        "operationId": "AuthController_googleCallback",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/linkedin": {
      "get": {
        "operationId": "AuthController_linkedinAuth",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/linkedin/callback": {
      "get": {
        "operationId": "AuthController_linkedinCallback",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/github": {
      "get": {
        "operationId": "AuthController_githubAuth",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "GitHub OAuth — account login/signup. Pass ?state=connect:<jwt> to link to existing account.",
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/github/callback": {
      "get": {
        "operationId": "AuthController_githubCallback",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/github/app/install-url": {
      "get": {
        "description": "Generates a one-time URL to install the Proofboard GitHub App. Pass ?linkedAccountId=<id> to install the App on a specific linked GitHub account (e.g. a work org) instead of your primary account.",
        "operationId": "AuthController_getGithubAppInstallUrl",
        "parameters": [
          {
            "name": "linkedAccountId",
            "required": false,
            "in": "query",
            "description": "ID of a linked GitHub account to install the App on. Omit for primary account.",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get GitHub App installation URL",
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/github/app/callback": {
      "get": {
        "operationId": "AuthController_githubAppCallback",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "GitHub App installation callback — do not call directly",
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/gitlab": {
      "get": {
        "operationId": "AuthController_gitlabAuth",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "GitLab OAuth — login/signup + repo access. Pass ?state=connect:<jwt> to link.",
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/gitlab/callback": {
      "get": {
        "operationId": "AuthController_gitlabCallback",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/bitbucket": {
      "get": {
        "operationId": "AuthController_bitbucketAuth",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "Bitbucket OAuth — login/signup + repo access. Pass ?state=connect:<jwt> to link.",
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/auth/bitbucket/callback": {
      "get": {
        "operationId": "AuthController_bitbucketCallback",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "tags": [
          "Auth"
        ]
      }
    },
    "/api/v1/proposals": {
      "post": {
        "operationId": "ProposalsController_create",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateProposalDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Create a proposal (starts as draft)",
        "tags": [
          "Proposals"
        ]
      },
      "get": {
        "operationId": "ProposalsController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "status",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "draft",
                "sent",
                "viewed",
                "accepted",
                "declined",
                "expired"
              ]
            }
          },
          {
            "name": "clientId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "minAmount",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "maxAmount",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "createdAt",
                "status",
                "clientName"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my proposals (filter by status, client, amount range, date range)",
        "tags": [
          "Proposals"
        ]
      }
    },
    "/api/v1/proposals/share/{token}": {
      "get": {
        "operationId": "ProposalsController_findByToken",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "View proposal by share token (client-facing, marks as viewed)",
        "tags": [
          "Proposals"
        ]
      }
    },
    "/api/v1/proposals/share/{token}/accept": {
      "post": {
        "operationId": "ProposalsController_accept",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Client accepts a proposal",
        "tags": [
          "Proposals"
        ]
      }
    },
    "/api/v1/proposals/share/{token}/decline": {
      "post": {
        "operationId": "ProposalsController_decline",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/DeclineProposalDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Client declines a proposal",
        "tags": [
          "Proposals"
        ]
      }
    },
    "/api/v1/proposals/{id}": {
      "get": {
        "operationId": "ProposalsController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get a proposal by ID",
        "tags": [
          "Proposals"
        ]
      },
      "patch": {
        "operationId": "ProposalsController_update",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateProposalDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update a draft proposal",
        "tags": [
          "Proposals"
        ]
      },
      "delete": {
        "operationId": "ProposalsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Delete a proposal (soft)",
        "tags": [
          "Proposals"
        ]
      }
    },
    "/api/v1/proposals/{id}/send": {
      "patch": {
        "operationId": "ProposalsController_send",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Send a draft proposal to client",
        "tags": [
          "Proposals"
        ]
      }
    },
    "/api/v1/admin/proposals/stats": {
      "get": {
        "operationId": "AdminProposalsController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Proposal stats by status + total accepted revenue",
        "tags": [
          "Admin — Proposals"
        ]
      }
    },
    "/api/v1/admin/proposals": {
      "get": {
        "operationId": "AdminProposalsController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "status",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "draft",
                "sent",
                "viewed",
                "accepted",
                "declined",
                "expired"
              ]
            }
          },
          {
            "name": "clientId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "minAmount",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "maxAmount",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "createdAt",
                "status",
                "clientName"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          },
          {
            "name": "ownerId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List all proposals (filter by ownerId, status, client, amount range, date range)",
        "tags": [
          "Admin — Proposals"
        ]
      }
    },
    "/api/v1/admin/proposals/{id}": {
      "get": {
        "operationId": "AdminProposalsController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Get a proposal by ID",
        "tags": [
          "Admin — Proposals"
        ]
      },
      "delete": {
        "operationId": "AdminProposalsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Soft-delete a proposal",
        "tags": [
          "Admin — Proposals"
        ]
      }
    },
    "/api/v1/chat/sessions": {
      "post": {
        "operationId": "ChatController_startSession",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/StartSessionDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Start or get existing 1-to-1 session",
        "tags": [
          "Chat"
        ]
      },
      "get": {
        "operationId": "ChatController_getMySessions",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "List my chat sessions",
        "tags": [
          "Chat"
        ]
      }
    },
    "/api/v1/chat/unread-count": {
      "get": {
        "operationId": "ChatController_unreadCount",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Total unread messages count",
        "tags": [
          "Chat"
        ]
      }
    },
    "/api/v1/chat/sessions/{sessionId}/messages": {
      "get": {
        "operationId": "ChatController_getMessages",
        "parameters": [
          {
            "name": "sessionId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get messages (paginated, newest first)",
        "tags": [
          "Chat"
        ]
      },
      "post": {
        "operationId": "ChatController_sendMessage",
        "parameters": [
          {
            "name": "sessionId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/SendMessageDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Send text message via HTTP (also broadcasts via WebSocket to room)",
        "tags": [
          "Chat"
        ]
      }
    },
    "/api/v1/chat/sessions/{sessionId}/upload": {
      "post": {
        "operationId": "ChatController_uploadFile",
        "parameters": [
          {
            "name": "sessionId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Upload file attachment ≤ 10 MB (broadcasts via WebSocket after upload)",
        "tags": [
          "Chat"
        ]
      }
    },
    "/api/v1/chat/sessions/{sessionId}/read": {
      "patch": {
        "operationId": "ChatController_markRead",
        "parameters": [
          {
            "name": "sessionId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Mark all messages in session as read",
        "tags": [
          "Chat"
        ]
      }
    },
    "/api/v1/reputation/me": {
      "get": {
        "operationId": "ReputationController_getMyReputation",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my reputation score and breakdown",
        "tags": [
          "Reputation"
        ]
      }
    },
    "/api/v1/reputation/users/{userId}": {
      "get": {
        "operationId": "ReputationController_getUserReputation",
        "parameters": [
          {
            "name": "userId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get reputation score for a public user profile",
        "tags": [
          "Reputation"
        ]
      }
    },
    "/api/v1/admin/reputation/stats": {
      "get": {
        "operationId": "AdminReputationController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Platform reputation stats: by label, avg score, max score",
        "tags": [
          "Admin — Reputation"
        ]
      }
    },
    "/api/v1/admin/reputation": {
      "get": {
        "operationId": "AdminReputationController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "userId",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "minScore",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "maxScore",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "reputationLabel",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "new",
                "rising",
                "trusted",
                "expert"
              ]
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "totalScore",
                "calculatedAt",
                "averageRating"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List all reputation scores (filter by userId, label, score range, date range)",
        "tags": [
          "Admin — Reputation"
        ]
      }
    },
    "/api/v1/admin/reputation/users/{userId}": {
      "get": {
        "operationId": "AdminReputationController_findByUser",
        "parameters": [
          {
            "name": "userId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Get reputation score for a specific user",
        "tags": [
          "Admin — Reputation"
        ]
      }
    },
    "/api/v1/admin/reputation/users/{userId}/recalculate": {
      "post": {
        "operationId": "AdminReputationController_forceRecalculate",
        "parameters": [
          {
            "name": "userId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Force-recalculate reputation score for a user",
        "tags": [
          "Admin — Reputation"
        ]
      }
    },
    "/api/v1/admin/auth/login": {
      "post": {
        "operationId": "AdminController_login",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/AdminLoginDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "summary": "Admin login",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/dashboard": {
      "get": {
        "operationId": "AdminController_getDashboard",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Dashboard stats",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/projects/pending": {
      "get": {
        "operationId": "AdminController_getPendingProjects",
        "parameters": [
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {}
          },
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {}
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List projects pending admin verification",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/users": {
      "get": {
        "operationId": "AdminController_getUsers",
        "parameters": [
          {
            "name": "search",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {}
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List all users",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/create": {
      "post": {
        "operationId": "AdminController_createAdmin",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateAdminDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Create a new admin account (super admin only)",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/users/{userId}/reputation/freeze": {
      "post": {
        "description": "Sets reputationFrozen=true and locks score at 0 with FLAGGED label. Reputation stays locked even during recalculate() calls until unfrozen. Only use for confirmed fraud — this is permanent until manually reversed.",
        "operationId": "AdminController_freezeReputation",
        "parameters": [
          {
            "name": "userId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/FreezeReputationDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Freeze a user reputation score at 0 — SUPER_ADMIN only",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/admin/users/{userId}/reputation/unfreeze": {
      "post": {
        "operationId": "AdminController_unfreezeReputation",
        "parameters": [
          {
            "name": "userId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Unfreeze a user reputation and trigger recalculation — SUPER_ADMIN only",
        "tags": [
          "Admin"
        ]
      }
    },
    "/api/v1/waitlist/subscribe": {
      "post": {
        "operationId": "WaitlistController_subscribe",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/SubscribeWaitlistDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "summary": "Subscribe to the waitlist",
        "tags": [
          "Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist/stats": {
      "get": {
        "operationId": "AdminWaitlistController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Waitlist statistics including referral source breakdown",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist": {
      "get": {
        "operationId": "AdminWaitlistController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "example": 1,
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "example": 50,
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "description": "Search by email or name",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "isNotified",
            "required": false,
            "in": "query",
            "description": "Filter by notification status",
            "schema": {
              "type": "boolean"
            }
          },
          {
            "name": "referralSource",
            "required": false,
            "in": "query",
            "schema": {
              "example": "twitter",
              "type": "string"
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "example": "2024-01-01",
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "example": "2024-12-31",
              "type": "string"
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "default": "subscribedAt",
              "type": "string",
              "enum": [
                "subscribedAt",
                "email",
                "fullName"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "default": "desc",
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "List waitlist entries (filter by notified, referral source, date range, search)",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist/export": {
      "get": {
        "operationId": "AdminWaitlistController_export",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "example": 1,
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "example": 50,
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "description": "Search by email or name",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "isNotified",
            "required": false,
            "in": "query",
            "description": "Filter by notification status",
            "schema": {
              "type": "boolean"
            }
          },
          {
            "name": "referralSource",
            "required": false,
            "in": "query",
            "schema": {
              "example": "twitter",
              "type": "string"
            }
          },
          {
            "name": "dateFrom",
            "required": false,
            "in": "query",
            "schema": {
              "example": "2024-01-01",
              "type": "string"
            }
          },
          {
            "name": "dateTo",
            "required": false,
            "in": "query",
            "schema": {
              "example": "2024-12-31",
              "type": "string"
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "default": "subscribedAt",
              "type": "string",
              "enum": [
                "subscribedAt",
                "email",
                "fullName"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "default": "desc",
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Export all waitlist entries matching filters (for CSV download)",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist/{id}": {
      "get": {
        "operationId": "AdminWaitlistController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Get a single waitlist entry",
        "tags": [
          "Admin — Waitlist"
        ]
      },
      "delete": {
        "operationId": "AdminWaitlistController_deleteEntry",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Delete a single waitlist entry",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist/notify": {
      "patch": {
        "operationId": "AdminWaitlistController_markNotified",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/MarkNotifiedDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Mark specific entries as notified by IDs",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist/notify-all": {
      "patch": {
        "operationId": "AdminWaitlistController_markAllNotified",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Mark ALL pending (un-notified) entries as notified",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/admin/waitlist/bulk": {
      "delete": {
        "operationId": "AdminWaitlistController_bulkDelete",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/MarkNotifiedDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "Admin-JWT-auth": []
          }
        ],
        "summary": "Bulk delete waitlist entries by IDs",
        "tags": [
          "Admin — Waitlist"
        ]
      }
    },
    "/api/v1/users/me/dashboard": {
      "get": {
        "description": "Returns a single aggregated payload covering reputation, proof assets, commit trajectory, proofboard views, activity log, pending milestone bundles, and recent proofboards. Response is Redis-cached for 60 seconds.",
        "operationId": "DashboardController_getDashboard",
        "parameters": [],
        "responses": {
          "200": {
            "description": "Dashboard data returned successfully. `cached: true` when served from cache."
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get full dashboard data for the authenticated user",
        "tags": [
          "Users"
        ]
      }
    },
    "/api/v1/dealboards": {
      "post": {
        "operationId": "DealboardsController_create",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateDealboardDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Create a new dealboard (name + industry locked permanently)",
        "tags": [
          "Dealboards"
        ]
      },
      "get": {
        "operationId": "DealboardsController_findAll",
        "parameters": [
          {
            "name": "page",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "limit",
            "required": false,
            "in": "query",
            "schema": {
              "type": "number"
            }
          },
          {
            "name": "search",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "status",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "active",
                "deleted"
              ]
            }
          },
          {
            "name": "sortBy",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "createdAt",
                "updatedAt",
                "name"
              ]
            }
          },
          {
            "name": "sortOrder",
            "required": false,
            "in": "query",
            "schema": {
              "type": "string",
              "enum": [
                "asc",
                "desc"
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "List all dealboards for the authenticated user",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/stats": {
      "get": {
        "operationId": "DealboardsController_getStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get dealboard stats for dashboard: total boards, free boards, activations, total views",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/{id}": {
      "get": {
        "operationId": "DealboardsController_findOne",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get a single dealboard with deployments summary",
        "tags": [
          "Dealboards"
        ]
      },
      "delete": {
        "operationId": "DealboardsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "204": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Delete a dealboard",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/{id}/projects": {
      "patch": {
        "operationId": "DealboardsController_updateProjects",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateDealboardProjectsDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update proof assets attached to this dealboard (always editable)",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/{id}/description": {
      "patch": {
        "operationId": "DealboardsController_updateDescription",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateDealboardDescriptionDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update dealboard description (always editable)",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/{id}/deploy": {
      "post": {
        "description": "Returns deployment token ONCE — store it, it cannot be recovered.",
        "operationId": "DealboardsController_deploy",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "paymentReference",
            "required": true,
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/DeployDealboardDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Deploy dealboard to a recipient ($2.99 for BASIC, free for PRO)",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/{id}/deployments": {
      "get": {
        "operationId": "DealboardsController_getDeployments",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "List all deployments for a dealboard",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/deployments/{deploymentId}": {
      "get": {
        "operationId": "DealboardsController_getDeployment",
        "parameters": [
          {
            "name": "deploymentId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get single deployment analytics (owner view)",
        "tags": [
          "Dealboards"
        ]
      },
      "delete": {
        "operationId": "DealboardsController_deleteDeployment",
        "parameters": [
          {
            "name": "deploymentId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "204": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Delete a deployment",
        "tags": [
          "Dealboards"
        ]
      }
    },
    "/api/v1/dealboards/deploy/{token}": {
      "get": {
        "description": "Returns board + projects with watermark. Validates token and expiry.",
        "operationId": "DealboardPublicController_accessDeployment",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "Access a deployed dealboard via token (recipient view)",
        "tags": [
          "Dealboards — Public"
        ]
      }
    },
    "/api/v1/dealboards/deploy/{token}/visit": {
      "post": {
        "operationId": "DealboardPublicController_recordVisit",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/RecordDeploymentVisitDto"
              }
            }
          }
        },
        "responses": {
          "204": {
            "description": ""
          }
        },
        "summary": "Record a visit/session event on a deployed board",
        "tags": [
          "Dealboards — Public"
        ]
      }
    },
    "/api/v1/dealboards/deploy/{token}/asset-dwell": {
      "post": {
        "operationId": "DealboardPublicController_recordAssetDwell",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/RecordAssetDwellDto"
              }
            }
          }
        },
        "responses": {
          "204": {
            "description": ""
          }
        },
        "summary": "Record time spent on a specific proof asset",
        "tags": [
          "Dealboards — Public"
        ]
      }
    },
    "/api/v1/payments/initiate/deployment": {
      "post": {
        "description": "Returns authorization URL — redirect user to this URL to complete payment.",
        "operationId": "PaymentsController_initiateDeploymentPayment",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/InitiateDeploymentPaymentDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Initiate $2.99 payment to activate a deployment (BASIC users)",
        "tags": [
          "Payments"
        ]
      }
    },
    "/api/v1/payments/initiate/subscription": {
      "post": {
        "description": "Returns authorization URL — redirect user to complete subscription setup.",
        "operationId": "PaymentsController_initiateSubscription",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/InitiateSubscriptionDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Initiate Pro subscription ($9.99/mo or $99/yr)",
        "tags": [
          "Payments"
        ]
      }
    },
    "/api/v1/payments/subscription": {
      "delete": {
        "operationId": "PaymentsController_cancelSubscription",
        "parameters": [],
        "responses": {
          "204": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Cancel active Pro subscription",
        "tags": [
          "Payments"
        ]
      }
    },
    "/api/v1/payments/billing": {
      "get": {
        "operationId": "PaymentsController_getBillingHistory",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get billing history and active subscription",
        "tags": [
          "Payments"
        ]
      }
    },
    "/api/v1/payments/verify/{reference}": {
      "get": {
        "operationId": "PaymentsController_verifyPayment",
        "parameters": [
          {
            "name": "reference",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Verify a payment after callback redirect",
        "tags": [
          "Payments"
        ]
      }
    },
    "/api/v1/payments/webhook/paystack": {
      "post": {
        "operationId": "PaymentWebhookController_paystackWebhook",
        "parameters": [
          {
            "name": "x-paystack-signature",
            "required": true,
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "Paystack webhook endpoint — do not call manually",
        "tags": [
          "Payments — Webhooks"
        ]
      }
    },
    "/api/v1/vcs-sync/trigger": {
      "post": {
        "description": "Syncs repos where you are a contributor (not the owner). Owner repos sync automatically via webhooks.",
        "operationId": "VcsSyncController_triggerSync",
        "parameters": [],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Manually trigger VCS sync for all contributor-mode repos",
        "tags": [
          "VCS Sync"
        ]
      }
    },
    "/api/v1/certifications": {
      "post": {
        "operationId": "CertificationsController_create",
        "parameters": [],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateCertificationRequestDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Add a new certification to your profile",
        "tags": [
          "Certifications"
        ]
      },
      "get": {
        "operationId": "CertificationsController_findAll",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get all my certifications",
        "tags": [
          "Certifications"
        ]
      }
    },
    "/api/v1/certifications/{id}": {
      "patch": {
        "operationId": "CertificationsController_update",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateCertificationRequestDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Update a certification",
        "tags": [
          "Certifications"
        ]
      },
      "delete": {
        "operationId": "CertificationsController_remove",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Remove a certification from your profile",
        "tags": [
          "Certifications"
        ]
      }
    },
    "/api/v1/certifications/{id}/reverify": {
      "post": {
        "operationId": "CertificationsController_reverify",
        "parameters": [
          {
            "name": "id",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Re-run verification check on a certification URL",
        "tags": [
          "Certifications"
        ]
      }
    },
    "/api/v1/verification/projects/{projectId}/request": {
      "post": {
        "operationId": "VerificationController_requestVerification",
        "parameters": [
          {
            "name": "projectId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/RequestVerificationDto"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Send client verification Magic Link for a project",
        "tags": [
          "Verification"
        ]
      }
    },
    "/api/v1/verification/validate/{token}": {
      "get": {
        "operationId": "VerificationController_validateToken",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "Validate a verification token (public) — called before rendering client page",
        "tags": [
          "Verification"
        ]
      }
    },
    "/api/v1/verification/complete/{token}": {
      "post": {
        "operationId": "VerificationController_completeVerification",
        "parameters": [
          {
            "name": "token",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "user-agent",
            "required": true,
            "in": "header",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "accept-language",
            "required": true,
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CompleteVerificationDto"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": ""
          }
        },
        "summary": "Client clicks Approve — completes verification (public)",
        "tags": [
          "Verification"
        ]
      }
    },
    "/api/v1/verification/projects/{projectId}/history": {
      "get": {
        "operationId": "VerificationController_getProjectVerifications",
        "parameters": [
          {
            "name": "projectId",
            "required": true,
            "in": "path",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get verification request history for a project",
        "tags": [
          "Verification"
        ]
      }
    },
    "/api/v1/verification/my-stats": {
      "get": {
        "operationId": "VerificationController_getMyStats",
        "parameters": [],
        "responses": {
          "200": {
            "description": ""
          }
        },
        "security": [
          {
            "JWT-auth": []
          }
        ],
        "summary": "Get my verification request statistics",
        "tags": [
          "Verification"
        ]
      }
    }
  },
  "info": {
    "title": "Proofboard API",
    "description": "Backend API for Proofboard — the verified proof-of-work platform for developers.\n\n**Download Specification:** [openapi.json](https://proboardly-backend.onrender.com/openapi.json)\n\nThe API is organized around:\n- **Auth** — JWT login, Google/GitHub/GitLab/Bitbucket OAuth\n- **Projects (Proof Assets)** — VCS import, work logs, API specs, client verification\n- **Proofboards** — public portfolios and paid dedicated boards per project\n- **Proposals** — send + track scoped proposals with payment data\n- **Reputation** — trust score calculation and career ledger\n- **Notifications** — rich, clickable notifications with actionUrl + meta\n- **VCS Sync** — scheduled and manual sync for contributor-mode repos\n- **Certifications**, **Activity Log**, **Webhooks**, **Admin**\n\nAll successful JSON responses use `{ success: true, data: ... }` unless the route streams a file.",
    "version": "1.0.0",
    "contact": {
      "name": "Proofboard Engineering",
      "url": "https://proofboard.app",
      "email": "engineering@proofboard.app"
    },
    "license": {
      "name": "Proprietary",
      "url": ""
    }
  },
  "tags": [],
  "servers": [
    {
      "url": "https://proboardly-backend.onrender.com",
      "description": "Development Environment"
    },
    {
      "url": "https://api.proofboard.app",
      "description": "Production"
    },
    {
      "url": "https://api-dev.proofboard.app",
      "description": "Development"
    }
  ],
  "components": {
    "securitySchemes": {
      "JWT-auth": {
        "scheme": "bearer",
        "bearerFormat": "JWT",
        "type": "http",
        "in": "header",
        "description": "User JWT — obtained from POST /api/v1/auth/login or OAuth callback"
      },
      "Admin-JWT-auth": {
        "scheme": "bearer",
        "bearerFormat": "JWT",
        "type": "http",
        "in": "header",
        "description": "Admin JWT — obtained from POST /api/v1/admin/auth/login"
      }
    },
    "schemas": {
      "SocialLinksDto": {
        "type": "object",
        "properties": {
          "linkedin": {
            "type": "string"
          },
          "dribbble": {
            "type": "string"
          },
          "behance": {
            "type": "string"
          },
          "twitter": {
            "type": "string"
          },
          "website": {
            "type": "string"
          },
          "github": {
            "type": "string"
          }
        }
      },
      "NotificationPrefsDto": {
        "type": "object",
        "properties": {
          "emailNotifications": {
            "type": "boolean"
          },
          "onSiteNotifications": {
            "type": "boolean"
          },
          "verificationConfirmed": {
            "type": "boolean"
          },
          "verificationDeclined": {
            "type": "boolean"
          },
          "paymentVerified": {
            "type": "boolean"
          },
          "proofboardViewed": {
            "type": "boolean"
          },
          "githubImportComplete": {
            "type": "boolean"
          },
          "milestoneReady": {
            "type": "boolean"
          },
          "proposalViewed": {
            "type": "boolean"
          },
          "proposalAccepted": {
            "type": "boolean"
          },
          "proposalLinkOpened": {
            "type": "boolean"
          },
          "proposalDeclined": {
            "type": "boolean"
          }
        }
      },
      "UpdateUserDto": {
        "type": "object",
        "properties": {
          "firstName": {
            "type": "string"
          },
          "lastName": {
            "type": "string"
          },
          "username": {
            "type": "string",
            "description": "Public username (a-z0-9_-)",
            "example": "sasu_dev"
          },
          "title": {
            "type": "string"
          },
          "bio": {
            "type": "string"
          },
          "location": {
            "type": "string"
          },
          "serviceType": {
            "type": "string",
            "description": "Free-text role — e.g. \"Backend Engineer\", \"Solidity Developer\""
          },
          "sellingPlatform": {
            "type": "string",
            "enum": [
              "upwork",
              "fiverr",
              "toptal",
              "direct",
              "other"
            ]
          },
          "goals": {
            "type": "array",
            "items": {
              "type": "string",
              "enum": [
                "get_more_clients",
                "build_portfolio",
                "track_work",
                "improve_reputation",
                "other"
              ]
            }
          },
          "socialLinks": {
            "$ref": "#/components/schemas/SocialLinksDto"
          },
          "notificationPreferences": {
            "$ref": "#/components/schemas/NotificationPrefsDto"
          }
        }
      },
      "UpdateNotificationPreferencesDto": {
        "type": "object",
        "properties": {
          "emailNotifications": {
            "type": "boolean"
          },
          "onSiteNotifications": {
            "type": "boolean"
          },
          "verificationConfirmed": {
            "type": "boolean"
          },
          "verificationDeclined": {
            "type": "boolean"
          },
          "paymentVerified": {
            "type": "boolean"
          },
          "proofboardViewed": {
            "type": "boolean"
          },
          "githubImportComplete": {
            "type": "boolean"
          },
          "milestoneReady": {
            "type": "boolean"
          },
          "proposalViewed": {
            "type": "boolean"
          },
          "proposalAccepted": {
            "type": "boolean"
          },
          "proposalLinkOpened": {
            "type": "boolean"
          },
          "proposalDeclined": {
            "type": "boolean"
          }
        }
      },
      "ProjectStoryDto": {
        "type": "object",
        "properties": {
          "problemFacing": {
            "type": "string"
          },
          "howSolved": {
            "type": "string"
          },
          "outcome": {
            "type": "string"
          }
        }
      },
      "ImpactSignalDto": {
        "type": "object",
        "properties": {
          "category": {
            "type": "string",
            "example": "Response time"
          },
          "value": {
            "type": "number",
            "example": 40
          },
          "unit": {
            "type": "string",
            "example": "%"
          },
          "direction": {
            "type": "string",
            "enum": [
              "improved",
              "reduced"
            ]
          }
        },
        "required": [
          "category",
          "value",
          "unit",
          "direction"
        ]
      },
      "CreateProjectDto": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string"
          },
          "description": {
            "type": "string"
          },
          "industry": {
            "type": "string"
          },
          "projectType": {
            "type": "string",
            "enum": [
              "client_work",
              "open_source",
              "personal",
              "internal_tool"
            ]
          },
          "clientName": {
            "type": "string"
          },
          "clientLinkedinUrl": {
            "type": "string"
          },
          "clientWebsite": {
            "type": "string"
          },
          "completedAt": {
            "type": "string"
          },
          "duration": {
            "type": "string"
          },
          "story": {
            "$ref": "#/components/schemas/ProjectStoryDto"
          },
          "achievements": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "responsibilities": {
            "type": "string"
          },
          "tools": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "githubRepo": {
            "type": "string"
          },
          "caseStudyUrl": {
            "type": "string"
          },
          "websiteUrl": {
            "type": "string"
          },
          "impactSignal": {
            "$ref": "#/components/schemas/ImpactSignalDto"
          }
        },
        "required": [
          "name"
        ]
      },
      "PaymentVerifiedDto": {
        "type": "object",
        "properties": {
          "amount": {
            "type": "number",
            "example": 2500
          },
          "currency": {
            "type": "string",
            "enum": [
              "USD",
              "NGN",
              "GBP",
              "EUR"
            ]
          },
          "method": {
            "type": "string",
            "example": "Paystack"
          }
        },
        "required": [
          "amount",
          "currency"
        ]
      },
      "UpdateProjectDto": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string"
          },
          "description": {
            "type": "string"
          },
          "industry": {
            "type": "string"
          },
          "projectType": {
            "type": "string",
            "enum": [
              "client_work",
              "open_source",
              "personal",
              "internal_tool"
            ]
          },
          "clientName": {
            "type": "string"
          },
          "clientLinkedinUrl": {
            "type": "string"
          },
          "clientWebsite": {
            "type": "string"
          },
          "completedAt": {
            "type": "string"
          },
          "duration": {
            "type": "string"
          },
          "story": {
            "$ref": "#/components/schemas/ProjectStoryDto"
          },
          "achievements": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "responsibilities": {
            "type": "string"
          },
          "tools": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "caseStudyUrl": {
            "type": "string"
          },
          "websiteUrl": {
            "type": "string"
          },
          "impactSignal": {
            "$ref": "#/components/schemas/ImpactSignalDto"
          },
          "paymentVerified": {
            "$ref": "#/components/schemas/PaymentVerifiedDto"
          }
        }
      },
      "VcsImportDto": {
        "type": "object",
        "properties": {
          "provider": {
            "type": "string",
            "enum": [
              "github",
              "gitlab",
              "bitbucket"
            ]
          },
          "repoId": {
            "type": "string"
          },
          "repoName": {
            "type": "string",
            "description": "Full repo name in \"owner/repo\" format — use repoFullName from the listing response",
            "example": "DivineChuks/excellence-cbt"
          },
          "repoUrl": {
            "type": "string"
          },
          "projectStatus": {
            "type": "string",
            "enum": [
              "draft",
              "in_progress",
              "completed"
            ],
            "description": "Select the current state of this project.  Controls milestone bundle depth: draft/in_progress = up to 3 bundles, completed = up to 5 bundles.",
            "default": "in_progress"
          },
          "linkedAccountId": {
            "type": "string",
            "description": "ID of a linked VCS account to import from. Omit to use the primary connection (falls back to linked accounts if primary is not set). The selected account is recorded on the project for future vcs-sync operations."
          }
        },
        "required": [
          "provider",
          "repoId",
          "repoName",
          "repoUrl",
          "projectStatus"
        ]
      },
      "WorkLogProofDto": {
        "type": "object",
        "properties": {
          "description": {
            "type": "string",
            "example": "Built a real-time payments notification system"
          },
          "role": {
            "type": "string",
            "example": "Backend Engineer"
          },
          "stack": {
            "example": [
              "NestJS",
              "Redis",
              "MongoDB"
            ],
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "duration": {
            "type": "string",
            "example": "3 months"
          },
          "outcome": {
            "type": "string"
          },
          "clientContext": {
            "type": "string"
          }
        },
        "required": [
          "description",
          "role",
          "stack"
        ]
      },
      "ApiSpecProofDto": {
        "type": "object",
        "properties": {
          "specType": {
            "type": "string",
            "enum": [
              "openapi",
              "postman",
              "endpoint_list"
            ]
          },
          "specContent": {
            "type": "string",
            "description": "Raw JSON/YAML or plain-text endpoint list"
          },
          "context": {
            "type": "string"
          },
          "architectureNotes": {
            "type": "string"
          }
        },
        "required": [
          "specType",
          "specContent"
        ]
      },
      "CodebaseUploadMetaDto": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "example": "Payments Microservice"
          },
          "description": {
            "type": "string"
          },
          "tools": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "duration": {
            "type": "string"
          }
        },
        "required": [
          "name"
        ]
      },
      "ApproveBundleDto": {
        "type": "object",
        "properties": {
          "existingProjectId": {
            "type": "string",
            "description": "Link to an existing project instead of creating new"
          }
        }
      },
      "UpdateBundleDto": {
        "type": "object",
        "properties": {
          "title": {
            "type": "string",
            "description": "Updated bundle title"
          },
          "outcomeSummary": {
            "type": "string",
            "description": "Updated outcome summary (CV paragraph)"
          },
          "detectedTools": {
            "description": "Detected tools",
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "category": {
            "type": "string",
            "enum": [
              "pending",
              "approved",
              "declined"
            ]
          }
        }
      },
      "RejectProjectDto": {
        "type": "object",
        "properties": {
          "reason": {
            "type": "string"
          }
        }
      },
      "ProofboardThemeDto": {
        "type": "object",
        "properties": {
          "primaryColor": {
            "type": "string"
          },
          "fontFamily": {
            "type": "string"
          },
          "layout": {
            "type": "string",
            "enum": [
              "grid",
              "list",
              "masonry"
            ]
          }
        }
      },
      "UpdateProofboardDto": {
        "type": "object",
        "properties": {
          "bio": {
            "type": "string",
            "description": "Short professional bio"
          },
          "headline": {
            "type": "string",
            "description": "Headline / tagline shown on public board"
          },
          "industry": {
            "type": "string",
            "description": "Industry / domain e.g. \"Fintech\""
          },
          "visibility": {
            "type": "string",
            "enum": [
              "public",
              "private"
            ]
          },
          "skills": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "tools": {
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "theme": {
            "$ref": "#/components/schemas/ProofboardThemeDto"
          }
        }
      },
      "UpdateFeaturedProjectsDto": {
        "type": "object",
        "properties": {
          "featuredProjectIds": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        }
      },
      "RegisterDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "john@example.com"
          },
          "password": {
            "type": "string",
            "example": "SecurePass123!"
          },
          "firstName": {
            "type": "string",
            "example": "John"
          },
          "lastName": {
            "type": "string",
            "example": "Doe"
          }
        },
        "required": [
          "email",
          "password"
        ]
      },
      "LoginDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "john@example.com"
          },
          "password": {
            "type": "string",
            "example": "SecurePass123!"
          }
        },
        "required": [
          "email",
          "password"
        ]
      },
      "VerifyEmailDto": {
        "type": "object",
        "properties": {
          "code": {
            "type": "string",
            "example": "123456"
          },
          "email": {
            "type": "string",
            "example": "user@example.com"
          }
        },
        "required": [
          "code"
        ]
      },
      "ResendOtpDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "john@example.com"
          }
        },
        "required": [
          "email"
        ]
      },
      "OnboardingStep1Dto": {
        "type": "object",
        "properties": {
          "serviceType": {
            "type": "string",
            "example": "Software Engineering"
          },
          "sellingPlatform": {
            "type": "string",
            "enum": [
              "upwork",
              "fiverr",
              "toptal",
              "direct",
              "other"
            ]
          },
          "location": {
            "type": "string",
            "example": "Lagos, Nigeria"
          },
          "bio": {
            "type": "string",
            "example": "I help early stage founders ship MVPs"
          }
        },
        "required": [
          "serviceType",
          "sellingPlatform"
        ]
      },
      "OnboardingStep2Dto": {
        "type": "object",
        "properties": {
          "goals": {
            "type": "array",
            "items": {
              "type": "string",
              "enum": [
                "get_more_clients",
                "build_portfolio",
                "track_work",
                "improve_reputation",
                "other"
              ]
            }
          }
        },
        "required": [
          "goals"
        ]
      },
      "ForgotPasswordDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "john@example.com"
          }
        },
        "required": [
          "email"
        ]
      },
      "ResetPasswordDto": {
        "type": "object",
        "properties": {
          "token": {
            "type": "string",
            "example": "reset-token-here"
          },
          "password": {
            "type": "string",
            "example": "NewPassword123!"
          }
        },
        "required": [
          "token",
          "password"
        ]
      },
      "ProposalSectionDto": {
        "type": "object",
        "properties": {
          "title": {
            "type": "string"
          },
          "content": {
            "type": "string"
          },
          "order": {
            "type": "number"
          }
        },
        "required": [
          "title",
          "content"
        ]
      },
      "PricingBreakdownDto": {
        "type": "object",
        "properties": {
          "label": {
            "type": "string"
          },
          "amount": {
            "type": "number"
          }
        },
        "required": [
          "label",
          "amount"
        ]
      },
      "ProposalPricingDto": {
        "type": "object",
        "properties": {
          "type": {
            "type": "string",
            "enum": [
              "fixed",
              "hourly",
              "retainer"
            ]
          },
          "amount": {
            "type": "number"
          },
          "currency": {
            "type": "string",
            "default": "USD"
          },
          "breakdown": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/PricingBreakdownDto"
            }
          }
        },
        "required": [
          "type",
          "amount"
        ]
      },
      "CreateProposalDto": {
        "type": "object",
        "properties": {
          "clientEmail": {
            "type": "string"
          },
          "clientId": {
            "type": "string"
          },
          "clientName": {
            "type": "string"
          },
          "projectId": {
            "type": "string"
          },
          "title": {
            "type": "string"
          },
          "summary": {
            "type": "string"
          },
          "sections": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/ProposalSectionDto"
            }
          },
          "pricing": {
            "$ref": "#/components/schemas/ProposalPricingDto"
          },
          "validUntil": {
            "type": "string"
          }
        },
        "required": [
          "clientEmail",
          "title",
          "pricing"
        ]
      },
      "DeclineProposalDto": {
        "type": "object",
        "properties": {
          "reason": {
            "type": "string"
          }
        }
      },
      "UpdateProposalDto": {
        "type": "object",
        "properties": {
          "clientEmail": {
            "type": "string"
          },
          "clientId": {
            "type": "string"
          },
          "clientName": {
            "type": "string"
          },
          "projectId": {
            "type": "string"
          },
          "title": {
            "type": "string"
          },
          "summary": {
            "type": "string"
          },
          "sections": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/ProposalSectionDto"
            }
          },
          "pricing": {
            "$ref": "#/components/schemas/ProposalPricingDto"
          },
          "validUntil": {
            "type": "string"
          }
        },
        "required": [
          "clientEmail",
          "title",
          "pricing"
        ]
      },
      "StartSessionDto": {
        "type": "object",
        "properties": {
          "recipientId": {
            "type": "string",
            "description": "User ID to start a chat with"
          }
        },
        "required": [
          "recipientId"
        ]
      },
      "SendMessageDto": {
        "type": "object",
        "properties": {
          "content": {
            "type": "string"
          },
          "type": {
            "type": "string",
            "enum": [
              "text",
              "file",
              "image",
              "system"
            ]
          },
          "fileUrl": {
            "type": "string"
          },
          "fileName": {
            "type": "string"
          }
        },
        "required": [
          "content"
        ]
      },
      "AdminLoginDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "admin@proofboard.com"
          },
          "password": {
            "type": "string",
            "example": "SecureAdmin123!"
          }
        },
        "required": [
          "email",
          "password"
        ]
      },
      "ReviewProjectDto": {
        "type": "object",
        "properties": {
          "comment": {
            "type": "string",
            "example": "Could not verify the proof files provided"
          }
        }
      },
      "CreateAdminDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "admin@proboardly.com"
          },
          "password": {
            "type": "string",
            "example": "SecureAdmin123!"
          },
          "firstName": {
            "type": "string",
            "example": "Admin"
          },
          "lastName": {
            "type": "string",
            "example": "User"
          },
          "role": {
            "type": "string",
            "enum": [
              "super_admin",
              "moderator",
              "support"
            ]
          }
        },
        "required": [
          "email",
          "password"
        ]
      },
      "FreezeReputationDto": {
        "type": "object",
        "properties": {
          "reason": {
            "type": "string",
            "example": "Collusion ring detected — circular vouching with 3 accounts"
          }
        },
        "required": [
          "reason"
        ]
      },
      "SubscribeWaitlistDto": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "example": "user@example.com"
          },
          "fullName": {
            "type": "string",
            "example": "John Doe"
          },
          "phoneNumber": {
            "type": "string",
            "example": "+2348012345678"
          },
          "referralSource": {
            "type": "string",
            "example": "twitter",
            "description": "Where they heard about us"
          },
          "interests": {
            "example": [
              "design",
              "proposals"
            ],
            "type": "array",
            "items": {
              "type": "string"
            }
          },
          "metadata": {
            "type": "object",
            "description": "Extra metadata (UTM params etc)"
          }
        },
        "required": [
          "email"
        ]
      },
      "MarkNotifiedDto": {
        "type": "object",
        "properties": {
          "ids": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        "required": [
          "ids"
        ]
      },
      "CreateDealboardDto": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "example": "For FinTech Clients"
          },
          "industry": {
            "type": "string",
            "example": "FinTech"
          },
          "description": {
            "type": "string",
            "example": "Payment systems and banking dashboards."
          },
          "projectIds": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        "required": [
          "name",
          "industry"
        ]
      },
      "UpdateDealboardProjectsDto": {
        "type": "object",
        "properties": {
          "projectIds": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        },
        "required": [
          "projectIds"
        ]
      },
      "UpdateDealboardDescriptionDto": {
        "type": "object",
        "properties": {
          "description": {
            "type": "string"
          }
        }
      },
      "DeployDealboardDto": {
        "type": "object",
        "properties": {
          "recipientName": {
            "type": "string",
            "example": "Adaeze Johnson"
          },
          "recipientEmail": {
            "type": "string",
            "example": "adaeze@paystack.com"
          },
          "personalNote": {
            "type": "string",
            "example": "Hi Adaeze — I put this together specifically for Paystack's frontend role."
          }
        },
        "required": [
          "recipientName",
          "recipientEmail"
        ]
      },
      "RecordDeploymentVisitDto": {
        "type": "object",
        "properties": {
          "durationMs": {
            "type": "number"
          },
          "source": {
            "type": "string",
            "enum": [
              "direct",
              "cv-link",
              "email",
              "other"
            ]
          },
          "expandedProjects": {
            "type": "array",
            "items": {
              "type": "string"
            }
          }
        }
      },
      "RecordAssetDwellDto": {
        "type": "object",
        "properties": {
          "projectId": {
            "type": "string"
          },
          "durationMs": {
            "type": "number"
          }
        },
        "required": [
          "projectId",
          "durationMs"
        ]
      },
      "InitiateDeploymentPaymentDto": {
        "type": "object",
        "properties": {
          "dealboardId": {
            "type": "string"
          }
        },
        "required": [
          "dealboardId"
        ]
      },
      "InitiateSubscriptionDto": {
        "type": "object",
        "properties": {
          "type": {
            "type": "string",
            "enum": [
              "pro_monthly",
              "pro_yearly"
            ]
          }
        },
        "required": [
          "type"
        ]
      },
      "CreateCertificationRequestDto": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "example": "AWS Certified Solutions Architect"
          },
          "issuingOrganisation": {
            "type": "string",
            "example": "Amazon Web Services"
          },
          "credentialId": {
            "type": "string",
            "example": "ABC123"
          },
          "credentialUrl": {
            "type": "string",
            "example": "https://credly.com/badges/abc123"
          },
          "issueDate": {
            "type": "string",
            "example": "2024-01-15"
          },
          "expiryDate": {
            "type": "string",
            "example": "2027-01-15",
            "description": "Null for non-expiring certs"
          },
          "certificationType": {
            "type": "string",
            "enum": [
              "certification",
              "degree",
              "course",
              "award"
            ]
          },
          "badgeImageUrl": {
            "type": "string"
          },
          "isPublic": {
            "type": "boolean",
            "default": true
          }
        },
        "required": [
          "name",
          "issuingOrganisation",
          "issueDate"
        ]
      },
      "UpdateCertificationRequestDto": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "example": "AWS Certified Solutions Architect"
          },
          "issuingOrganisation": {
            "type": "string",
            "example": "Amazon Web Services"
          },
          "credentialId": {
            "type": "string",
            "example": "ABC123"
          },
          "credentialUrl": {
            "type": "string",
            "example": "https://credly.com/badges/abc123"
          },
          "issueDate": {
            "type": "string",
            "example": "2024-01-15"
          },
          "expiryDate": {
            "type": "string",
            "example": "2027-01-15",
            "description": "Null for non-expiring certs"
          },
          "certificationType": {
            "type": "string",
            "enum": [
              "certification",
              "degree",
              "course",
              "award"
            ]
          },
          "badgeImageUrl": {
            "type": "string"
          },
          "isPublic": {
            "type": "boolean",
            "default": true
          }
        },
        "required": [
          "name",
          "issuingOrganisation",
          "issueDate"
        ]
      },
      "RequestVerificationDto": {
        "type": "object",
        "properties": {
          "clientEmail": {
            "type": "string",
            "example": "client@company.com"
          },
          "clientName": {
            "type": "string",
            "example": "John Smith"
          }
        },
        "required": [
          "clientEmail"
        ]
      },
      "CompleteVerificationDto": {
        "type": "object",
        "properties": {
          "browserFingerprint": {
            "type": "string",
            "description": "Browser fingerprint hash"
          },
          "timeToClickMs": {
            "type": "number",
            "description": "Time in ms from page load to button click"
          },
          "h": {
            "type": "string",
            "description": "Honeypot field — must be empty for legit users"
          },
          "rating": {
            "type": "number",
            "example": 5
          },
          "review": {
            "type": "string",
            "example": "Excellent work, delivered on time."
          }
        }
      }
    }
  }
}
```


---


# Implement docs similar to the claud docs, create a docs folder with index.md and all other supporting pages linked, indexed and properly sectioned and updated with code changes.

https://code.claude.com/docs/en/terminal-guide


---

# 
