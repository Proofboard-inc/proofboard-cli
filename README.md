# Proofboard Career Agent

Proofboard builds your career while you focus on building software.

The Career Agent runs locally, detects Git workspaces opened in supported IDEs, and continuously turns meaningful engineering activity into structured proof. Authentication, project setup, and synchronization happen automatically; the `proofboard` executable remains available for advanced users, automation, and debugging.

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
npm install -g @proofboard/agent
```

The release scripts support Linux, macOS, and Windows. Running `proofboard install` directly installs the native binary and registers the appropriate systemd user service, macOS LaunchAgent, or Windows scheduled task.

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
proofboard link
proofboard sync
proofboard sync --incremental
proofboard unlink
proofboard logs --lines 100
proofboard update
proofboard config ides
```

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
