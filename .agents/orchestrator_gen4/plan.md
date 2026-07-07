# Plan - Proofboard CLI v1.8.0 Release

This plan outlines the steps to verify the compiled binaries and publish the v1.8.0 final release of the Proofboard CLI to GitHub.

## Milestones

### Milestone 1: Pre-Release Verification
- **Objective**: Verify that the compiled binaries exist in the `build/` directory, check their version and target architectures, and ensure that local unit tests pass.
- **Exit Criteria**:
  - Binaries for linux-amd64, darwin-amd64, darwin-arm64, and windows-amd64.exe are present in `build/`.
  - Binary versions match `v1.8.0` (or `1.8.0`).
  - Unit tests run and pass without errors.
  - The local git repository status is clean and ready for tag.

### Milestone 2: GitHub Release Creation
- **Objective**: Create the git tag `v1.8.0` and publish a GitHub Release with the compiled binaries.
- **Exit Criteria**:
  - Git tag `v1.8.0` is pushed to the remote repository.
  - GitHub Release `v1.8.0` is created with the title "Proofboard CLI v1.8.0".
  - The release body mentions:
    - Removal of Phase 6 Handshake.
    - Addition of local fraud detection.
  - The four compiled binaries are uploaded as release assets.
  - Verification auditor gives a CLEAN verdict.
