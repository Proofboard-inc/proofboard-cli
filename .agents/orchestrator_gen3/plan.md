# Plan for Proofboard CLI Compliance & Release v1.4

This plan details the steps required to verify that the Proofboard CLI project fully complies with all specifications in `SPEC.md`, `README.md`, and `GEMINI.md`.

## Milestone 1: Exploration, API Review, and PR Check
- **Objective**: Explore the Go codebase, review endpoints in SPEC.md, run the test suite via a worker, and verify if any PR is needed for `proofboard-backend`.
- **Exit Criteria**:
  - Codebase structure and existing state understood.
  - Endpoints reviewed and aligned.
  - Test suite passes initially.

## Milestone 2: Implementation of Compliance & Gaps
- **Objective**: Fix any compliance gaps between the code and `SPEC.md` v1.4.
- **Exit Criteria**:
  - All gaps corrected.
  - Unit tests added/updated for modified components.

## Milestone 3: Verification
- **Objective**: Run automated verification using worker, reviewer, challenger, and auditor subagents.
- **Exit Criteria**:
  - 100% tests pass.
  - Verification auditor gives a CLEAN verdict.

## Milestone 4: Release Publication
- **Objective**: Compile statically for Windows/macOS/Linux and publish to GitHub.
- **Exit Criteria**:
  - Release binaries generated and verified.
  - GitHub release created via `gh`.
