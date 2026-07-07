# Progress

Last visited: 2026-07-07T08:26:00Z

- [x] Verify binary existence in `dist/` and `build/`.
- [x] Run `file` and `ldd` checks on `proofboard-linux-amd64` to verify it is statically linked.
- [x] Run `go version -m` on all 4 binaries and verify `CGO_ENABLED=0` and `-ldflags="-s -w"`.
- [ ] Write findings to `handoff.md`.
