# Change Log — NDA Safety / In-Memory Constraint Violation (Milestone 1)

## Modifications

### `internal/pipeline/phase2/intent.go`
- Avoided casting `commit.Subject` to string during Phase 2 classification.
- Implemented `toLowerBytes(b []byte) []byte` to perform in-memory lowercase operations on a mutable, zeroable byte slice.
- Switched keyword matching from `strings.Contains` to `bytes.Contains(subjectLowerBytes, []byte(strings.ToLower(keyword)))`.
- Updated `NoiseScore` to trim and perform lowercase operations purely on byte slices (avoiding string casts) and checking matching against list of trivial byte slices.
- Explicitly zeroed out the original `commit.Subject` and `subjectLowerBytes` via `crypto.ZeroBytes(...)` at the end of each commit's iteration in `Classify`, setting references to `nil`.

### `internal/pipeline/phase2/intent_test.go`
- Added unit tests verifying classification logic, trivial keyword noise score matching, and validating that raw commit subjects and their backing arrays are fully zeroed and nil'd during Phase 2.

### `internal/pipeline/phase5/shredder_test.go`
- Added `TestShredWithNilSubject` to ensure the second defensive pass of shredding functions properly and doesn't panic when `Subject` is nil.

## Verification
- Built the codebase and verified that `go test ./...` and `go vet ./...` compile and pass.
