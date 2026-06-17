# Proofboard CLI

Proofboard CLI is a local-first developer verification system written in Go.

The CLI reads local Git history, classifies work locally, destroys proprietary text before any network call, and sends only anonymized metadata to the Proofboard API.

## Commands

```bash
proofboard auth
proofboard link
proofboard sync
proofboard sync --incremental
proofboard sync --skip-handshake
proofboard unlink
proofboard status
proofboard logs --lines 100
proofboard update
proofboard update-dictionary
proofboard config set auto-update-dictionary false
```

`proofboard stop` is not part of v1.4. The v1.4 architecture has no persistent daemon.

## What The CLI Transmits

- Commit SHA hashes
- Unix timestamps
- Additions, deletions, and files changed
- Category labels from the checked-in dictionary
- Milestone cluster metadata
- `orgHash`, `repoHash`, and `emailHash`
- Handshake status and anti-fraud counters
- CLI and dictionary versions

## What The CLI Destroys

- Commit messages
- File paths
- File contents and diffs
- Raw repository names
- Raw organization names
- Raw author emails

See [SHREDDER.md](SHREDDER.md) for the audit guide and code pointers.

## Corporate Proxy Notes

```bash
HTTPS_PROXY=https://proxy.company.com:8080 proofboard sync
```

For SSH proxying, configure `~/.ssh/config`:

```sshconfig
Host github.com
  ProxyCommand nc -X connect -x proxy.company.com:8080 %h %p
```

## Development

```bash
go test ./...
go vet ./...
```
