# Binary Verification Analysis — Proofboard CLI v1.8.0

This report presents the analysis and verification of the pre-compiled binaries located in `/workspaces/proofboard-cli/dist/` for the Proofboard CLI v1.8.0 release.

---

## 1. Summary of Findings

1. **Binary Existence & Structure**:
   All four requested binaries exist in the `/workspaces/proofboard-cli/dist/` directory.
2. **Architecture Correctness**:
   Each binary targets the correct operating system and CPU architecture requested (Linux amd64, macOS amd64, macOS arm64, Windows amd64).
3. **Discrepancy with Release Requirements (Non-Static/Non-Stripped)**:
   The binaries currently inside `dist/` were compiled *without* the release flags defined in `build_release.sh` (i.e., `CGO_ENABLED=0` and `-ldflags="-s -w"`).
   - As a result, the Linux binary is **dynamically linked** to `libc.so` and is **not stripped** (retaining full debug symbols). Its size is 14.93 MB, whereas a static, stripped build is 10.31 MB.
   - This violates the project's release requirement: **"Static binaries only"**.
4. **Outdated GitHub Release Assets**:
   The current assets uploaded to the GitHub release `v1.8.0` match the binaries from an older build (from July 6, which lacks the latest HEAD commit `cd0baadb` adding idempotency to `link` and auto-install on `update`).

---

## 2. Table of Dist Binaries (As Found on Disk)

These binaries were compiled after the latest commit (`cd0baadb`), but without release flags:

| Filename | Target Platform | Size (Bytes) | SHA256 Checksum | Executable Type / Format | Linkage |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `proofboard-darwin-amd64` | macOS amd64 | 15,156,016 | `edd52d6ef94e4d51c95e45c724c383cd5cc4261b83323df94bf3aee86c4ccebc` | Mach-O 64-bit x86_64 | Dynamic (macOS Default) |
| `proofboard-darwin-arm64` | macOS arm64 | 14,224,050 | `777475c79e0ffde0a110e6205031dd0e07a854eaa3131198ff2335edbe81d4ed` | Mach-O 64-bit arm64 | Dynamic (macOS Default) |
| `proofboard-linux-amd64` | Linux amd64 | 14,934,984 | `088fdb13752950f36e53d2d3c941602d0d5891da7a4c55613bcb6fc75248e14f` | ELF 64-bit LSB x86-64 | **Dynamic** (interpreter `/lib64/ld-linux-x86-64.so.2`) |
| `proofboard-windows-amd64.exe` | Windows amd64 | 15,264,768 | `d6693b1decbdc3fc200196713a4045c79fffa4a73eacb52cf0958216c2ef06db` | PE32+ (console) x86-64 | Static |

---

## 3. Table of Correct Binaries (If Re-compiled with Release Flags)

We performed a test compilation of the latest HEAD commit (`cd0baadb`) using `build_release.sh` specifications (`CGO_ENABLED=0` and `-ldflags="-s -w"`):

| Filename | Target Platform | Size (Bytes) | SHA256 Checksum | Linkage & Symbols Verification |
| :--- | :--- | :--- | :--- | :--- |
| `proofboard-darwin-amd64` | macOS amd64 | 10,532,400 | `2e22258922ac24c8230567d2e564ac0e12bf8f60e3f79bf91da135c9e5a5c12d` | Mach-O 64-bit x86_64, stripped |
| `proofboard-darwin-arm64` | macOS arm64 | 9,792,850 | `2c60d8e423055da25714c68f5d4fdbe40dabe5505b90126d5acc792c7bef335d` | Mach-O 64-bit arm64, stripped |
| `proofboard-linux-amd64` | Linux amd64 | 10,313,890 | `7023c09ab8cb3252c71b15ca5e2e716c5f93247381567f47f7965753aca6e5d3` | ELF 64-bit LSB x86-64, **statically linked**, stripped |
| `proofboard-windows-amd64.exe` | Windows amd64 | 10,722,304 | `07db9a4d113b9d4c0e45f9b22c63ad6af77514ad60a42b9a570bc579002bb084` | PE32+ (console) x86-64, stripped |

---

## 4. Verification Details & Diagnostics

1. **Linux Binary Dynamic Linkage Audit**:
   Running `ldd dist/proofboard-linux-amd64` reveals dynamic links:
   ```
   linux-vdso.so.1 (0x00007ffd3c7e0000)
   libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007ccd1152a000)
   /lib64/ld-linux-x86-64.so.2 (0x00007ccd11745000)
   ```
   This occurs when CGO is not disabled during build (`CGO_ENABLED=1` is Go's default on Linux development hosts).

2. **Re-compilation Validation**:
   When compiling with `CGO_ENABLED=0 go build -ldflags="-s -w"`, the Go compiler builds static binaries and strips them. This reduces the sizes by ~30% and ensures they run on target platforms without shared library dependencies.

3. **Comparison with Currently Published GitHub Release `v1.8.0` Assets**:
   The current assets on the remote release have the following checksums (matching older `build/` files from July 6):
   - `proofboard-darwin-amd64`: `3fdba6144f627fdda5a7b06ff58267e6c3b7182a009d111ec8f25fe2ad42bd0c`
   - `proofboard-darwin-arm64`: `635d2215c068810b23284265e50ed02294e17aa392996c7611b34592d480ff83`
   - `proofboard-linux-amd64`: `fe9cecb778beb8d52a5b6eb9c639eb80cbc4094829f3f106120e76d0a31b2e1d`
   - `proofboard-windows-amd64.exe`: `79a0a9d65051dfaf61aaa954084d9e4f468c2c258336fe72d3c46e2fde546051`

---

## 5. Actionable Recommendation

To resolve both issues (rebuilding static binaries with the latest code modifications and publishing them correctly):
1. Re-run the release build script to compile the latest code statically:
   ```bash
   ./build_release.sh
   ```
2. Re-upload the newly generated static binaries from `dist/` to the GitHub release `v1.8.0` (which requires deleting the old release and tag first, then creating them new).
