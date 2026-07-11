# Plan - Proofboard CLI v1.8.0 Release Creation

This plan outlines the steps to verify the compiled binaries in `dist/` and publish the v1.8.0 final release of the Proofboard CLI to GitHub.

## Milestones

### Milestone 2: GitHub Release Creation
- **Objective**: Create the git tag `v1.8.0` (if not present) and publish a GitHub Release with the compiled binaries in `dist/`.
- **Exit Criteria**:
  - Git tag `v1.8.0` is pushed to the remote repository.
  - GitHub Release `v1.8.0` is created with the title "Proofboard CLI v1.8.0".
  - The release body mentions:
    - Removal of Phase 6 Handshake.
    - Addition of local fraud detection.
  - The four compiled binaries from `dist/` are uploaded as release assets:
    - `proofboard-linux-amd64`
    - `proofboard-darwin-amd64`
    - `proofboard-darwin-arm64`
    - `proofboard-windows-amd64.exe`
  - Verification auditor gives a CLEAN verdict.
