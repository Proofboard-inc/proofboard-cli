# Analysis Report - Compiled Binaries Verification in build/

This report contains the findings from the verification of the compiled binaries located in the `build/` directory, including their existence, file size, permissions, architecture, static linking status, and execution on Linux.

## Summary of Findings

1. **Existence**: All four required binaries (`proofboard-linux-amd64`, `proofboard-darwin-amd64`, `proofboard-darwin-arm64`, and `proofboard-windows-amd64.exe`) exist in the `build/` directory.
2. **File Size**: The binaries in `build/` are larger (ranging from 13.6 MB to 14.6 MB) compared to the statically compiled and stripped binaries in `dist/` (ranging from 9.3 MB to 10.2 MB).
3. **Permissions**: All binaries have read/write/execute permissions (`-rwxrwxrwx`) in this environment.
4. **Architectures & Formats**:
   - `proofboard-linux-amd64`: ELF 64-bit LSB executable, x86-64.
   - `proofboard-darwin-amd64`: Mach-O 64-bit x86_64 executable.
   - `proofboard-darwin-arm64`: Mach-O 64-bit arm64 executable.
   - `proofboard-windows-amd64.exe`: PE32+ executable (console) x86-64.
5. **Static Linking Status**:
   - The binaries in `build/` are **dynamically linked** and **not stripped**. For example, `build/proofboard-linux-amd64` depends on `libc.so.6` and `ld-linux-x86-64.so.2`.
   - **This is a violation** of the release requirements specified in `AGENTS.md` and `SPEC.md`, which mandate **static binaries only** with zero runtime dependencies.
6. **Execution**: The Linux binary (`build/proofboard-linux-amd64`) executes successfully on the system, outputting `No linked repositories.` for the `status` command, and reporting its version as `1.8.0`.
7. **Version Discrepancy**:
   - The `build/` binaries report version `1.8.0` (matching the current modified source code in `internal/version/version.go`).
   - The `dist/` binaries report version `1.4.7` (matching the original/release tags and npm configurations).

---

## Detailed Binary Inspection

### Table 1: Binary Properties (`build/` Directory)

| Binary File | Size (Bytes) | Permissions | File Type / Architecture | Linking | Stripped | Go Version |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `proofboard-linux-amd64` | 14,931,752 | `-rwxrwxrwx` | ELF 64-bit LSB executable, x86-64 | **Dynamic** | No | 1.8.0 |
| `proofboard-darwin-amd64` | 15,156,000 | `-rwxrwxrwx` | Mach-O 64-bit x86_64 executable | **Dynamic** | No | 1.8.0 |
| `proofboard-darwin-arm64` | 14,224,050 | `-rwxrwxrwx` | Mach-O 64-bit arm64 executable | **Dynamic** | No | 1.8.0 |
| `proofboard-windows-amd64.exe` | 15,263,232 | `-rwxrwxrwx` | PE32+ executable (console) x86-64 | **Dynamic** | No | 1.8.0 |

### Table 2: Binary Properties (`dist/` Directory - Reference)

| Binary File | Size (Bytes) | Permissions | File Type / Architecture | Linking | Stripped | Go Version |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `proofboard-linux-amd64` | 10,317,986 | `-rwxrwxrwx` | ELF 64-bit LSB executable, x86-64 | **Static** | Yes | 1.4.7 |
| `proofboard-darwin-amd64` | 10,532,336 | `-rwxrwxrwx` | Mach-O 64-bit x86_64 executable | **Dynamic** (Mach-O) | Yes | 1.4.7 |
| `proofboard-darwin-arm64` | 9,792,802 | `-rwxrwxrwx` | Mach-O 64-bit arm64 executable | **Dynamic** (Mach-O) | Yes | 1.4.7 |
| `proofboard-windows-amd64.exe` | 10,722,304 | `-rwxrwxrwx` | PE32+ executable (console) x86-64 | Static / PE | Yes | 1.4.7 |

---

## Technical Verification Details

### 1. Linking Check (Linux Binary)

Running `ldd build/proofboard-linux-amd64` shows dynamic library dependencies:
```bash
$ ldd build/proofboard-linux-amd64
	linux-vdso.so.1 (0x00007ffda0dee000)
	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007b4e122dc000)
	/lib64/ld-linux-x86-64.so.2 (0x00007b4e124f7000)
```
This confirms that the binary in `build/` is compiled with dynamic linking enabled (e.g. standard compilation where CGO is enabled, and no `-extldflags "-static"` options were passed).

Conversely, `ldd dist/proofboard-linux-amd64` confirms it is statically linked:
```bash
$ ldd dist/proofboard-linux-amd64
	not a dynamic executable
```

### 2. Execution Check (Linux Binary)

The binary `build/proofboard-linux-amd64` executes successfully in the workspace environment:
```bash
$ ./build/proofboard-linux-amd64 status
No linked repositories.
```

```bash
$ ./build/proofboard-linux-amd64 --version
proofboard version 1.8.0
```

---

## Discrepancies and Issues Identified

1. **Violation of Static Linking Requirement**:
   The release binaries compiled in `build/` are **dynamically linked**, requiring `libc.so.6` at runtime. Under different distributions or minimal environments (like Alpine), they may fail to run due to missing dynamically linked libraries. The static binaries requirements are not met by files in `build/`.
   
2. **Lack of Stripping**:
   The binaries in `build/` are not stripped, which leaves debug symbols intact and increases file sizes by ~4-5 MB per binary.

3. **Compilation Pipeline Discrepancy**:
   The script `build_release.sh` compiles clean, statically linked, and stripped binaries under `dist/` by setting:
   `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/proofboard-linux-amd64 ./cmd/proofboard`
   However, the binaries in `build/` were compiled without these flags (likely standard `go build` with CGO enabled), resulting in dynamic linking and unstripped symbols.
