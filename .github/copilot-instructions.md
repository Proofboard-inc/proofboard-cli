# AGENTS.md

Sync changes here across the following locations:
- (project root)/AGENTS.md
- (project root)/GEMINI.md
- (project root)/CLAUDE.md
- (project root)/.kiro/steering/project-rules.md
- (project root)/.cursorrules
- (project root)/.windsurfrules
- (project root)/.github/copilot-instructions.md

Read the product and architecture documentation in `(project root)/README.md`
before changing behavior. The normative specification is
`(project root)/SPEC.md`.

## Privacy Constraints

Preserve the NDA-safe, local-first architecture. After pipeline Phase 5, never
store:

* Commit messages
* File contents
* Diffs
* Repository names
* Organization names
* Author emails

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

Use SHA256 for hashes, HTTPS for API calls, JWT authentication for payloads,
and `0600` permissions for credentials. Never log or print private keys,
tokens, raw emails, repository names, or shredded proprietary data.

## Product Behavior

Keep authentication, repository setup, workspace detection, and synchronization
automatic. Installation must register and start the background agent. Preserve
the three workspace choices and suppression semantics documented in README.
Keep project detection and synchronization repository-agnostic: provider
tooling, repository visibility, and public-provider signals must never become
user-facing requirements or block local analysis.

Use `Proofboard Career Agent` as the user-facing name. Do not present `CLI`,
`auth`, `link`, or `sync` as required product concepts.

## Coding Rules

* Go 1.21+
* No global mutable state
* Context everywhere
* Unit tests required
* Cobra for CLI
* Viper for config
* Structured logging
* No panic in command handlers
* Explicit error wrapping

## Testing Integrity

Never use mocks, fake servers, simulated API responses, or mock syncs.
Authentication, linking, synchronization, and other network-dependent
acceptance tests must run against the real configured development services.
Never claim end-to-end success unless the real dev backend and frontend flow
succeeds.

Tests must isolate their filesystem state and must not alter real credentials,
repository state, shell profiles, logs, installed binaries, or background
services unless the test is explicitly exercising that real behavior and
restores or deliberately replaces it.

## Release Policy

Every release must satisfy the complete release matrix documented in README.
Never publish a partial release or only the local platform binary. Build all
four static targets, sign each binary, attach every native installer, metadata
file, install script, and npm tarball, and verify the attached assets before
claiming success.

## Backend Repository

https://github.com/Proofboard-inc/proofboard-backend

Local Clone Path: `/tmp/proofboard-backend`

### Backend Change Policy

Never commit or push changes directly to the backend repository's `main`
branch. All backend changes must use a dedicated branch and pull request. Do
not merge a backend pull request unless the user explicitly authorizes it.
