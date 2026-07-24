# AGENTS.md

Sync changes here across the following locations:
- (project root)/AGENTS.md
- (project root)/GEMINI.md
- (project root)/CLAUDE.md
- (project root)/.kiro/steering/project-rules.md
- (project root)/.cursorrules
- (project root)/.windsurfrules
- (project root)/.github/copilot-instructions.md

Spec at - (project root)/SPEC.md

## Product

Proofboard Career Agent v1.8.14

Implementation language: Go 1.21+

Agent executable and advanced CLI entrypoint:

```bash
proofboard
```

## Mission

Proofboard is a local-first Career Agent that builds structured engineering proof while developers focus on building software.

The Career Agent reads local Git history, classifies work locally, destroys proprietary information before network transmission, and sends only anonymized metadata to the Proofboard API. Authentication, repository setup, and synchronization should be automatic; commands remain available for advanced users, automation, debugging, scripting, and CI/CD.

The NDA-safe architecture is non-negotiable.

## Hard Constraints

Never store:

* Commit messages
* File contents
* Diffs
* Repository names
* Organization names
* Author emails

after Phase 5.

Never transmit:

* Commit messages
* File paths
* Repository names
* Organization names
* Author emails

Only transmit:

* SHA hashes
* timestamps
* additions
* deletions
* files changed
* category labels
* cluster metadata
* orgHash
* emailHash

## Commands

Required advanced commands:

* auth (Supports headless fallback via CLI output)
* link
* unlink
* sync
* status
* logs
* update
* config
* install (Global system-wide installer)
* uninstall (Global uninstaller)
* completion (Interactive auto-completion setup)
* agent (Background Career Agent lifecycle and status)

## Desktop Detection

When an engineer opens a Git workspace in an IDE, the Career Agent must detect the workspace even if the engineer never runs `link` or `sync` manually. The watcher must treat both untracked repositories and tracked-but-unsynced repositories as actionable. Tracked repositories sync automatically without another prompt.

Use the same three-option surface as the terminal prompt:

* `Sync Project` connects or re-syncs the repo and continues.
* `Not Now` dismisses for the current session.
* `Never Ask Again` suppresses the workspace permanently.

The watcher must compare the active workspace against Proofboard state, ignore already-suppressed workspaces, and surface the prompt as soon as the IDE opens the project.

## Career Agent UX

Installation must register and start the background agent. When synchronization requires authentication, the agent opens `https://proofboard.io/agent/cli-auth?code=<generated_code>` with the temporary code prefilled, stores access and refresh tokens securely, and resumes synchronization. Valid refresh tokens are used silently.

The user-facing product name is `Proofboard Career Agent`. Avoid presenting `CLI`, `auth`, `link`, or `sync` as required user concepts. The primary promise is: `Proofboard builds your career while you focus on building software.`

## Pipeline

Phase 1
Local Git ingest

Phase 2
Classification

Phase 3
Scoring

Phase 4
Milestone detection

Phase 5
Shredder

Phase 6
Handshake

Phase 7
Payload assembly

Phase 8
Transmission

## Coding Rules

* No global mutable state
* Context everywhere
* Unit tests required
* Cobra for CLI
* Viper for config
* Structured logging
* No panic in command handlers
* Explicit error wrapping

## Security Rules

All hashes:

SHA256

All API calls:

HTTPS only

All payloads:

JWT authenticated

Credentials:

~/.proofboard/credentials.json
0600 permissions

State:

~/.proofboard/state.json

Logs:

~/.proofboard/sync.log

## Release Requirements

Linux amd64 (proofboard-linux-amd64)

macOS amd64 (proofboard-darwin-amd64)

macOS arm64 (proofboard-darwin-arm64)

Windows amd64 (proofboard-windows-amd64.exe)

Static binaries only.

CRITICAL DIRECTIVE: When cutting a new release, you MUST strictly build the full cross-compilation matrix (GOOS/GOARCH) for all 4 targets listed above and upload all 4 explicit binaries to the GitHub release. Do not merely upload the local environment's binary.

## Backend Repository
https://github.com/Proofboard-inc/proofboard-backend

Local Clone Path: `/tmp/proofboard-backend`
*(Note: If missing, reclone the repository to this path to perform backend changes.)*
