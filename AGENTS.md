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

Proofboard CLI v1.8.3

Implementation language: Go 1.21+

CLI entrypoint:

```bash
proofboard
```

## Mission

Proofboard is a local-first developer verification system.

The CLI reads local Git history, classifies work locally, destroys proprietary information before network transmission, and sends only anonymized metadata to the Proofboard API.

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

Required commands:

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

Linux amd64

macOS amd64

macOS arm64

Windows amd64

Static binaries only.

Always update the local binary (`./proofboard`) and the GitHub release artifacts when compiling new changes.

## Backend Repository
https://github.com/Proofboard-inc/proofboard-backend

Local Clone Path: `/tmp/proofboard-backend`
*(Note: If missing, reclone the repository to this path to perform backend changes.)*

