# Proofboard Career Agent — Website Copy

This copy is the handoff for the Proofboard website and dashboard repositories. User-facing pages must not present the command-line interface as the product.

## Primary message

**Proofboard builds your career while you focus on building software.**

The Proofboard Career Agent runs locally, recognizes engineering activity, and quietly turns it into structured career proof.

Primary call to action: **Install Proofboard Career Agent**

## Navigation and status labels

Use:

- Career Agent Documentation
- Career Agent Status
- Proofboard Career Agent Active
- Install Proofboard Career Agent

Do not use:

- Download CLI
- CLI Documentation
- CLI Status
- CLI Connected

## Installation panel

### Install Proofboard Career Agent

Install once, then keep building. The Career Agent starts automatically, detects Git projects opened in your IDE, and synchronizes meaningful engineering activity in the background.

No terminal required.

Select the installer from the visitor's platform:

- Linux x86-64: `Proofboard-Career-Agent-linux-amd64.deb`
- macOS Intel: `Proofboard-Career-Agent-darwin-amd64.pkg`
- macOS Apple Silicon: `Proofboard-Career-Agent-darwin-arm64.pkg`
- Windows x86-64: `Proofboard-Career-Agent-windows-amd64-setup.exe`

Each native installer places `proofboard` on `PATH`, registers the Career Agent at sign-in, and starts it for the current user. Do not direct the primary website button to a shell script or bare command-line binary.

Power-user alternatives:

```text
brew install proofboard
npm install -g @proofboard/agent
```

## Repository prompt

**New repository detected.**

Would you like Proofboard to track this project?

- Sync Project
- Not Now
- Never Ask Again

## Authentication prompt

**Your Proofboard session has expired.**

- Reconnect

## Milestone prompt

**Milestone detected.**

Example: Payment Infrastructure Completed

- Review
- Publish
- Ignore

## Career Agent status

```text
Proofboard Career Agent
Running locally
Last sync: 3 minutes ago
Tracking 12 repositories
Authentication: Connected
```

## Privacy block

- Runs entirely on your machine.
- No proprietary source code leaves your computer.
- No employer access required.
- Designed to preserve NDA-safe engineering proof.
- Builds structured engineering proof without exposing confidential code.

Supporting copy:

Proofboard classifies work locally and destroys proprietary commit text and file paths before any network request. Only anonymized engineering metadata is transmitted.
