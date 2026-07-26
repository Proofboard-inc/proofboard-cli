# proofboard-cli

Thin npm launcher for the signed native
[Proofboard Career Agent](https://github.com/Proofboard-inc/proofboard-cli)
release.

Every argument is forwarded unchanged to the real Go executable. The package
does not reimplement, simulate, or mock Proofboard commands.

```bash
npx proofboard-cli install
npx proofboard-cli auth
npx proofboard-cli link
npx proofboard-cli sync
```

The package carries the signed Linux amd64, macOS amd64, macOS arm64, and
Windows amd64 release binaries. It verifies the selected binary before
execution.

Proofboard processes Git history locally and transmits only anonymized
engineering metadata. It never transmits source code, diffs, commit messages,
file paths, repository names, organization names, or author emails.
