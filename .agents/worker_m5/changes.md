# Changes Report — Milestone 5 (Build & Release)

## 1. Committing Uncommitted Milestone 1-4 Work
Before compiling the official release, we checked the git working directory. The previous milestones' implementations and tests were completed but not committed. To ensure the git tag `v1.4.0` corresponds exactly to the code that was built, we staged and committed all project source code, unit tests, and configuration changes:
- Tracked and staged files under `cmd/` and `internal/`
- Documentation and specification files: `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `SPEC.md`, `.kiro/`
- Committed with message: `"feat: complete Milestones 1-4 implementations and tests"`
- Pushed the commit to remote branch `main`.

## 2. Static Binary Compilation
We compiled static binaries for four targeted platforms with size optimization flags `CGO_ENABLED=0` and `-ldflags="-s -w"`.
The compilation commands executed:
- macOS arm64:
  `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o build/proofboard-darwin-arm64 ./cmd/proofboard`
- macOS amd64:
  `CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/proofboard-darwin-amd64 ./cmd/proofboard`
- Linux amd64:
  `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/proofboard-linux-amd64 ./cmd/proofboard`
- Windows amd64:
  `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/proofboard-windows-amd64.exe ./cmd/proofboard`

All four binaries were successfully built and stored under `/workspaces/proofboard-cli/build/` with the correct filenames.

## 3. Tagging and Release Publication
- Git Repository Tagging: We created tag `v1.4.0` at the newly committed HEAD and pushed the tag to the remote repository.
- GitHub Release Creation: Using the `gh` CLI, we created a release for `v1.4.0` and uploaded the compiled binaries.
  - Release Title: `"Proofboard CLI v1.4.0"`
  - Release Notes: `"First official release of Proofboard CLI v1.4.0 with NDA safety constraints and local classification."`
  - Release URL: `https://github.com/Proofboard-inc/proofboard-cli/releases/tag/v1.4.0`
