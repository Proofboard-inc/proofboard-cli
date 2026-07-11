## 2026-07-07T09:00:15Z
Your task is to recompile the release binaries statically and upload them to a new v1.8.0 GitHub release.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Steps to execute:
1. Recompile the release binaries by running:
   ./build_release.sh
2. Verify the built binaries:
   - Check their sizes and SHA256 checksums.
   - Run `file dist/*` to confirm that the Linux binary is statically linked and stripped.
   - Run `ldd dist/proofboard-linux-amd64` to verify it is not dynamically linked.
3. Clean up existing v1.8.0 release and tags:
   - Delete the existing release on GitHub:
     bash -c 'gh release delete v1.8.0 -y'
   - Delete the remote tag:
     git push origin :refs/tags/v1.8.0
   - Delete the local tag:
     git tag -d v1.8.0
4. Recreate the v1.8.0 tag and push it:
   - Create local tag:
     git tag v1.8.0
   - Push tag to remote:
     git push origin v1.8.0
5. Create the new release:
   - Run the creation command uploading the 4 files in `dist/` with the notes from `/workspaces/proofboard-cli/.agents/explorer_m2_gen6_3/release_notes.md`:
     bash -c 'gh release create v1.8.0 dist/* --title "Proofboard CLI v1.8.0" -F /workspaces/proofboard-cli/.agents/explorer_m2_gen6_3/release_notes.md'
6. Verify release:
   - View release to ensure all 4 assets are uploaded and have correct sizes:
     bash -c 'gh release view v1.8.0'
Write your execution report to /workspaces/proofboard-cli/.agents/worker_m2_gen6/handoff.md and notify the parent when done.
