# Original User Request

## Initial Request — 2026-06-16T17:52:17Z

Ensure the Proofboard CLI project fully complies with all specifications in `SPEC.md`, `README.md`, and `GEMINI.md`, with all targets met and tested, and automatically publish a polished v1.2/v1.4 final release package to GitHub using the `gh` or `git` CLI.

Working directory: /workspaces/proofboard-cli
Integrity mode: development (with strict adherence to avoiding copyright/legal issues)

## Requirements

### R1. Specification Compliance
The Proofboard CLI must strictly match the architecture and constraints defined in the provided markdown specifications. All 8 phases must be fully implemented, ensuring proprietary text is destroyed before any network communication.

### R2. Test Suite Creation and Coverage
The agents must write their own automated verification scripts or Go tests to prove the specifications are met before proceeding to release.

### R3. GitHub Release Publication
Run an iterative "Ralph loop" to ensure all code targets are met. Once verified, build static binaries for Linux amd64, macOS amd64, macOS arm64, and Windows amd64, and use the `gh` CLI to publish a polished release to the repository.

### R4. Infrastructure Constraint (GitHub Access)
You will need network access to GitHub via the `gh` CLI. Ensure you are authorized or have the correct permissions/token to push tags and create releases in the provided repository.

## Acceptance Criteria

### Verification & Compliance
- [ ] A custom programmatic verification script or `go test` suite proves that the CLI strips proprietary data before Phase 6.
- [ ] No compilation or `go vet` errors remain in the project.

### Release
- [ ] A GitHub Release is created containing statically compiled binaries for `proofboard-linux-amd64`, `proofboard-darwin-amd64`, `proofboard-darwin-arm64`, and `proofboard-windows-amd64.exe`.

## Follow-up — 2026-06-16T19:09:21Z

<USER_REQUEST>
Ensure the Proofboard CLI project fully complies with all specifications in `SPEC.md` (which has been recently updated), `README.md`, and `GEMINI.md`, with all targets met and tested. Review any related API endpoints, open PRs if necessary, and automatically publish a final release package to GitHub using the `gh` or `git` CLI.

Working directory: /workspaces/proofboard-cli
Integrity mode: development

## Requirements

### R1. Specification Compliance (Updated Spec)
Review the recently updated `SPEC.md` and strictly match the latest architecture, endpoints, and constraints defined within it. All 8 phases must be fully implemented, ensuring proprietary text is destroyed before any network communication.

### R2. Endpoint Review & PR Management
Review the endpoints mentioned in the spec. If there are any discrepancies or needed fixes in external components or related repos, open the necessary PRs (only if needed).

### R3. Test Suite Creation and Coverage
The agents must write their own automated verification scripts or Go tests to prove the updated specifications are met before proceeding to release.

### R4. GitHub Release Publication
Run an iterative "Ralph loop" to ensure all code targets are met. Once verified, build static binaries for Linux amd64, macOS amd64, macOS arm64, and Windows amd64, and use the `gh` CLI to publish a polished release to the repository.

### R5. Infrastructure Constraint (GitHub Access)
You will need network access to GitHub via the `gh` CLI. Ensure you are authorized or have the correct permissions/token to push tags, create PRs, and publish releases.

## Acceptance Criteria

### Verification & Compliance
- [ ] A custom programmatic verification script or `go test` suite proves that the CLI conforms to the *updated* spec (e.g., stripping proprietary data before Phase 6).
- [ ] No compilation or `go vet` errors remain in the project.
- [ ] Endpoints are reviewed; PRs are opened *only* if necessary.

### Release
- [ ] A GitHub Release is created containing statically compiled binaries for `proofboard-linux-amd64`, `proofboard-darwin-amd64`, `proofboard-darwin-arm64`, and `proofboard-windows-amd64.exe`.
</USER_REQUEST>
