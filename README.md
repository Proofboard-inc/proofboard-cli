# Proofboard Career Agent

Proofboard builds your career while you focus on building software.

The Career Agent runs locally, detects Git workspaces opened in supported IDEs, and continuously turns meaningful engineering activity into structured proof. Authentication, project setup, and synchronization happen automatically; the `proofboard` executable remains available for advanced users, automation, and debugging.

The current release is v1.9.3. The native agent is implemented in Go 1.21+.

## Install

The recommended installation path is the **Install Proofboard Career Agent** download on [proofboard.io](https://proofboard.io). The platform installer adds the executable to `PATH`, registers the background agent at sign-in, and starts it immediately. Release automation produces double-click installers for this website flow:

- `Proofboard-Career-Agent-linux-amd64.deb`
- `Proofboard-Career-Agent-darwin-amd64.pkg`
- `Proofboard-Career-Agent-darwin-arm64.pkg`
- `Proofboard-Career-Agent-windows-amd64-setup.exe`

The four explicit static binaries remain attached to every release for power users and automation.

Power users can also install with a package manager:

```bash
brew install proofboard
npm install -g proofboard-cli
```

The release scripts support Linux, macOS, and Windows. Running `proofboard install` directly installs the native binary and registers the appropriate systemd user service, macOS LaunchAgent, or Windows scheduled task.

The npm package is a thin launcher for the signed native release. It does not
reimplement agent commands in JavaScript:

```bash
npx proofboard-cli install
npx proofboard-cli sync
```

## How It Works

Once installed, the Career Agent:

- detects Git repositories as they open in an IDE;
- offers **Sync Project**, **Not Now**, and **Never Ask Again** for new repositories;
- opens a prefilled browser authorization page only when authentication is required;
- silently refreshes valid sessions;
- creates and connects projects when **Sync Project** is selected;
- synchronizes commits, merges, rewrites, remote-ref/default-branch metadata changes, and newly detected repository activity;
- surfaces milestones with Review, Publish, and Ignore actions.

Linked repositories with new activity sync in the background without another prompt. Git hooks complement IDE detection so tracking continues after commits and merges.

## Career Agent Status

```bash
proofboard status
```

The status view reports whether the local Career Agent is active, the last successful sync, the number of tracked repositories, and authentication state.

Desktop/dashboard integrations can consume the same status without parsing terminal copy:

```bash
proofboard status --json
```

The underlying commands remain available for advanced workflows:

```bash
proofboard agent status
proofboard auth
proofboard auth logout
proofboard link
proofboard sync
proofboard sync --incremental
proofboard unlink
proofboard logs --lines 100
proofboard logs clear
proofboard update
proofboard update-dictionary
proofboard config ides
proofboard completion
proofboard install
proofboard uninstall
```

## Local Pipeline

Every synchronization runs the same eight phases:

1. Local Git ingest
2. Classification
3. Scoring
4. Milestone detection
5. Shredder
6. Handshake
7. Payload assembly
8. Transmission

Commit messages, file paths, repository and organization names, author emails,
file contents, and diffs must not survive Phase 5.

## Private by Design

- Runs entirely on your machine.
- No proprietary source code leaves your computer.
- No employer access is required.
- Designed to preserve NDA-safe engineering proof.
- Builds structured engineering proof without exposing confidential code.

Before any network transmission, the local pipeline destroys commit messages and file paths and reduces activity to anonymized metadata.

The Career Agent may transmit only:

- commit SHA hashes and timestamps;
- additions, deletions, and file counts;
- category labels and milestone cluster metadata;
- `orgHash`, `repoHash`, and `emailHash`;
- handshake status, anti-fraud counters, and agent/dictionary versions.

It never transmits commit messages, file paths, repository or organization names, author emails, file contents, or diffs. See [SHREDDER.md](SHREDDER.md) for the audit guide.

Website and dashboard copy is specified in [docs/career-agent-website-copy.md](docs/career-agent-website-copy.md).

## Local Files and Security

All API calls use HTTPS, synchronized payloads require a CLI JWT, and hashes
use SHA256. Local state is stored under `~/.proofboard`:

- `credentials.json` — access and refresh credentials, mode `0600`
- `state.json` — linked repository and synchronization state
- `sync.log` — structured local logs

Device private keys and authentication tokens are never printed or
transmitted. Only a registered device public key leaves the machine.

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

## Release Engineering

Every release contains four statically linked binaries:

- Linux amd64: `proofboard-linux-amd64`
- macOS amd64: `proofboard-darwin-amd64`
- macOS arm64: `proofboard-darwin-arm64`
- Windows amd64: `proofboard-windows-amd64.exe`

The complete release artifact set is:

- all four static binaries, each with a detached `.sig`;
- `checksums.txt` and `latest.json`;
- `Proofboard-Career-Agent-linux-amd64.deb`;
- both macOS `.pkg` installers;
- `Proofboard-Career-Agent-windows-amd64-setup.exe`;
- the `proofboard-cli` npm package tarball;
- `install.sh`, `install.ps1`, and `install.cmd`.

The install scripts resolve the current release from `proofboard.io`, fall back
to the directly published GitHub release, verify signatures or checksums,
install without administrator access by default, connect the Career Agent, and
run workspace detection.

Release automation must build the full GOOS/GOARCH matrix and verify every
attachment. A local build alone is not a complete release.
