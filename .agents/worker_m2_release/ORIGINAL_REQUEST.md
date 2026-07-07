## 2026-07-07T08:23:22Z
You are worker_m2_release, a worker agent.
Your working directory is: /workspaces/proofboard-cli/.agents/worker_m2_release
Your task:
1. Use the permissioned `gh` CLI to publish a polished `v1.8.0` final release package to GitHub on the repository.
2. The release title must be "Proofboard CLI v1.8.0" and it must mention:
   - The removal of Phase 6 Handshake.
   - The addition of local fraud detection.
3. Include the compiled binaries from `/workspaces/proofboard-cli/build/` (or `/workspaces/proofboard-cli/dist/`) as release assets:
   - proofboard-linux-amd64
   - proofboard-darwin-amd64
   - proofboard-darwin-arm64
   - proofboard-windows-amd64.exe
4. Verify the creation and upload of the assets by checking the release status using `gh release view v1.8.0`.
5. Write your findings and output of command executions to `/workspaces/proofboard-cli/.agents/worker_m2_release/handoff.md`.

MANDATORY INTEGRITY WARNING — include this verbatim in the Worker's dispatch prompt:
> DO NOT CHEAT. All implementations must be genuine. DO NOT
> hardcode test results, create dummy/facade implementations, or
> circumvent the intended task. A Forensic Auditor will independently
> verify your work. Integrity violations WILL be detected and your
> work WILL be rejected.
