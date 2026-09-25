# Proofboard Shredder Audit Guide

Proofboard CLI is designed so proprietary repository text is used only in memory before Phase 5 and is never transmitted.

## Destroyed Before Network Calls

The shredder boundary is implemented in `internal/pipeline/phase5/shredder.go`.

Destroyed fields:

- Commit subject bytes are overwritten with zero bytes and set to nil.
- File path strings from `git log --numstat` are cleared and dropped.
- Raw author emails are cleared after SHA256 normalization.
- Raw repository names are not stored in state or payloads.
- Raw organization names are not stored in state or payloads.

The Phase 2 classifier also zeroes commit subject bytes immediately after deriving numerical/category signals in `internal/pipeline/phase2/intent.go`.

## Surviving Payload Data

Only these fields survive into `internal/model.SyncPayload`:

- Commit SHA hashes
- Unix timestamps
- Additions, deletions, files changed
- Category labels from the versioned dictionary
- Impact scores and milestone cluster metadata
- `orgHash`, `repoHash`, `emailHash`
- Handshake status, CLI version, dictionary version
- Anti-fraud counters and flags

## Tests

Privacy coverage lives in:

- `internal/pipeline/pipeline_test.go`
- `internal/pipeline/phase5/shredder_test.go`
- `internal/api/client_test.go`

Run:

```bash
go test ./...
```
