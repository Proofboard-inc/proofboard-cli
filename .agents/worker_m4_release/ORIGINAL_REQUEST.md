## 2026-06-17T11:18:50Z

You are the Release Worker (archetype: `teamwork_preview_worker`).
Your working directory is `/workspaces/proofboard-cli/.agents/worker_m4_release/`.
Your parent conversation ID is `066f5421-8262-4d3c-a457-bf22bdc942ea`.
Your task is to build the static CLI binaries for the release and publish the release to GitHub.

### Task Checklist
1. **Sync rules files check**: Check that `GEMINI.md`, `AGENTS.md`, `CLAUD.md`, and `.kiro/steering/project-rules.md` are all synchronized. If there are any discrepancies in product or version settings, update them to keep them in sync.
2. **Build static binaries**:
   - Compile static binaries for the following platforms under directory `build/`:
     - Linux amd64 -> name it `proofboard-linux-amd64`
     - macOS amd64 -> name it `proofboard-darwin-amd64`
     - macOS arm64 -> name it `proofboard-darwin-arm64`
     - Windows amd64 -> name it `proofboard-windows-amd64.exe`
   - Ensure the binaries are statically compiled (using `CGO_ENABLED=0` and appropriate compiler flags).
3. **Verify compilation**:
   - Verify that the local platform's binary works by executing a simple check (e.g. `./build/proofboard-linux-amd64 status` or similar status command).
4. **Publish GitHub Release**:
   - First, run a check using `gh auth status` or similar command to verify if you are logged in and authorized to write to the repository.
   - Pushing the tag and releasing:
     - Determine the next tag version. The version constant is `"1.4.0"`.
     - Check if the tag `v1.4.0` already exists in remote/local. If the tag already exists, or if there is a conflict, we can delete the tag locally and remote and recreate it, or edit/recreate the release to publish the fresh compliant binaries.
     - Pushing tag: `git tag v1.4.0 && git push origin v1.4.0` (force push/delete/recreate tag if needed to overwrite).
     - Create release: `gh release create v1.4.0 build/proofboard-linux-amd64 build/proofboard-darwin-amd64 build/proofboard-darwin-arm64 build/proofboard-windows-amd64.exe --title "Proofboard CLI v1.4.0 Release" --notes "Compliance and SPEC v1.4 update release"`
     - If the release already exists, we can edit/update it or recreate it to publish the fresh compliant binaries.
5. **Add verification details**:
   - Run tests once more on the final code before packaging.
   - Document all actions and commands used in `/workspaces/proofboard-cli/.agents/worker_m4_release/changes.md` and your final `handoff.md`.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Once completed, write your handoff and send a message back to the parent.
