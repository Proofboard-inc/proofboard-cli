# Proofboard CLI — Trust & Signing Handoff

Portable brief for the Proofboard Career Agent (Go CLI). Written so it can be
handed to another engineer or another agent harness with no other context.

Repository: `Proofboard-inc/proofboard-cli` (private)
Language: Go 1.21+ · CLI framework: cobra · Config: viper
Latest published release at time of writing: **v1.8.16**

---

## 1. Current environment (verified by direct probe, 2026-07-25)

Do not take these from documentation — they were tested raw with `curl`.

| Host | Status |
|---|---|
| `https://api-dev.proofboard.io` | **Live.** Serves the CLI API. Same backend as the Render URL below. |
| `https://proboardly-backend.onrender.com` | **Live.** Documented as "Development Environment"; identical responses to `api-dev.proofboard.io`. |
| `https://api-dev.proofboard.app` | **Does not resolve** (DNS NXDOMAIN) despite appearing in the OpenAPI `servers` list. |
| `https://api.proofboard.app` | **Does not resolve** (DNS NXDOMAIN) despite appearing in the OpenAPI `servers` list. |
| `https://api.proofboard.io` | Resolves, but 404s on `/api/v1/cli/auth/device-code`. Not the CLI backend. |
| `https://proofboard-frontend.vercel.app/cli-auth` | **Live.** The real device-authorization page. |
| `https://proofboard.io/agent/cli-auth` | **404.** Route not deployed. |
| `https://app.proofboard.io` | **404.** |
| `https://releases.proofboard.io/latest.json` | **404.** Public download host is not serving anything yet. |

CLI defaults as of v1.8.16 (`internal/config/config.go`):

```
api.base_url    = https://api-dev.proofboard.io
app.base_url    = https://proofboard-frontend.vercel.app
agent.auth_url  = https://proofboard-frontend.vercel.app/cli-auth
```

---

## 2. Open blocker: device-key registration returns 500

**Symptom** reported from a Windows machine on v1.8.16:

```
proofboard sync
Connect your Proofboard Career Agent.
Your device code is: ZM5P-FLU3
Opening browser to connect your Career Agent...
https://proofboard-frontend.vercel.app/cli-auth?code=ZM5P-FLU3
Waiting for authentication...
authenticate: connect Career Agent: ensure device key: API returned 500 Internal Server Error: {"statusCode":500}
```

**Probe result** — the endpoint fails *before* authentication is evaluated:

```
POST https://api-dev.proofboard.io/api/v1/cli/auth/device-key
  (no Authorization header)      -> 500 {"statusCode":500,"message":"Internal server error"}
  (Authorization: Bearer garbage)-> 500 {"statusCode":500,"message":"Internal server error"}

POST https://proboardly-backend.onrender.com/api/v1/cli/auth/device-key
  (no Authorization header)      -> 500
```

A route guarded by a CLI token returns **401** when the token is absent. This
one returns **500** unconditionally, on both hostnames, for every request shape
tried. **This is a server-side defect, not a CLI payload problem.**

The CLI request body matches `RegisterDeviceKeyDto` from the published OpenAPI
document exactly:

```jsonc
// RegisterDeviceKeyDto: { publicKey: string (required), deviceName?: string }
{ "publicKey": "<base64 ed25519 public key>" }
```

`deviceName` is optional and is currently not sent — worth adding as a label,
but it is not the cause of the 500.

**Root cause, proven live (2026-07-25):**

1. Validation runs fine — the CLI's payload is well-formed:
   ```
   empty body            -> 400 "publicKey must be a string"  (DTO validation working)
   correct payload shape -> 500 "Internal server error"        (passed validation, then crashed)
   ```
2. The 500 is identical with no Authorization header, a garbage Bearer token,
   or a real token from a completed device-code flow. Auth state does not
   change the outcome.
3. The sibling route in the exact same controller correctly guards itself:
   ```
   DELETE /api/v1/cli/auth/device-key/{id}  (no token) -> 401 Unauthorized
   POST   /api/v1/cli/auth/device-key       (no token) -> 500 Internal server error
   ```
4. The live OpenAPI spec confirms it: every authenticated route declares
   `"security": [{"JWT-auth": []}]` except this one, which has no `security`
   block at all — compare `POST /api/v1/cli/auth/device-key` against
   `DELETE /api/v1/cli/auth/device-key/{deviceKeyId}` or `GET /api/v1/users/me`
   in `https://api-dev.proofboard.io/docs-json`.

**Conclusion:** `CliAuthController.registerDeviceKey` is missing its
`@UseGuards(JwtAuthGuard)` (or equivalent). No guard runs, so nothing
populates the authenticated user on the request; the handler almost certainly
dereferences something like `req.user.id` to associate the key with a user,
throws on `undefined`, and Nest's global exception filter turns that into a
generic 500 — identically, regardless of what auth header is sent. This is a
one-line backend fix, not a CLI-side issue.

**For the backend engineer:** add the missing guard decorator to
`registerDeviceKey` in `CliAuthController` and confirm it now returns 401
without a token, matching every sibling route.

### CLI mitigation already applied

Both `proofboard auth` and `proofboard sync` used to abort when device-key
registration failed. They now degrade instead:

- Registration failure is recorded in `~/.proofboard/sync.log` as a warning.
- `auth` completes; credentials obtained from the device-code flow are kept.
- `sync` transmits the payload **unsigned** (`deviceKeyId` and
  `deviceSignature` omitted — both are optional in `CliPayloadDto`).
- Registration is retried on the next `auth` or `sync`.

This matches the rollout guidance in section 4 below: signature optional first,
required later.

---

## 3. Device-code flow (working, unchanged)

The server mints both the secret device code and the human-facing user code.

```
POST /api/v1/cli/auth/device-code        body: { deviceName?: string }
  -> { verificationUrl, deviceCode, userCode, expiresIn }
GET  /api/v1/cli/auth/poll/{deviceCode}  -> { status: approved|pending|expired|denied, token, refreshToken, username }
PUT  /api/v1/cli/auth/device-code/{userCode}/approve   (called by the web app, web JWT)
POST /api/v1/cli/auth/refresh            body: { refreshToken }
```

Live sample response:

```json
{
  "verificationUrl": "https://proofboard-frontend.vercel.app/cli-auth?code=7TZ9-3VZZ",
  "deviceCode": "ncVTGtcOXSpwhN2DVvyq0luP-rV3g_nTcT--x-XDV2E",
  "userCode": "7TZ9-3VZZ",
  "expiresIn": 600
}
```

### Trap worth knowing

The CLI used to pre-flight the authorization page with a `GET` and refuse to
open it if the body contained "this page could not be found". Single page
applications ship that string inside every page they serve, so the check
rejected a page that answered 200 and rendered correctly, and connecting failed
with "authorization page is unavailable". Detection now reads the document
`<title>` only, and it never blocks: an unreachable page is reported and the
address is handed over anyway.

---

## 4. Trust & signing requirements (from the lead developer)

**Why:** today anyone holding a valid CLI login token can bypass the CLI and
`curl` a fake sync payload straight to the API. Signing closes that gap.

**Explicit non-goal:** none of this proves the underlying commit data is real.
A user who owns their laptop can still type fake numbers into a payload before
signing it. Signing proves *"this payload came from the CLI installation this
user authenticated with"*, not *"this work happened"*. That is a permanent
limit of any client-side trust mechanism.

**Unchanged:** the device-code flow, the `emailHash` field (plain
`SHA256(email)`), and the overall `auth -> link -> hook -> ingest -> classify ->
score -> cluster -> shred -> POST` shape.

### 4.1 Device keypair

- **Ed25519**, generated once per installation, on the first successful
  `proofboard auth`.
- Storage priority: OS keychain (macOS Keychain, Windows Credential Manager,
  Linux Secret Service); fallback to `~/.proofboard/device.key` at `0600`.
- Never log, print, or transmit the private key.
- Register the **public** key via `POST /api/v1/cli/auth/device-key`, store the
  returned `deviceKeyId` locally.
- Re-auth on the same machine **reuses** the key. Rotate only on
  `proofboard auth --rotate-key`, or if the local key is missing/corrupt.

### 4.2 Signing every sync payload

1. **Canonicalize first.** Key order must be deterministic. Go's map iteration
   is randomized — marshal from a struct with fixed field order, or sort keys.
   The server recomputes the signature independently; byte-for-byte agreement is
   required or every signature fails even when nothing is wrong.
2. Sign the canonical bytes with the device private key.
3. Add `deviceKeyId` and `deviceSignature` to the payload.

The signature must cover the **entire** payload including `signedCommitRatio`
and `previousHead` — build the complete payload, then sign.

### 4.3 Signed-commit ratio

Git reports which commits are cryptographically signed (`%G?`). Capture it in
the same commit walk already used for categorization, then send
`antiFraudSignals.signedCommitRatio = signedCommits / totalCommits` (0..1).

### 4.4 `previousHead`

After a successful sync, store that repo's HEAD SHA locally. On the next sync of
that repo, send it as `previousHead` so the backend can detect history
rewriting. Omit the field on a repo's first-ever sync, and treat a missing or
unreadable local state file as a first-ever sync rather than guessing.

### 4.5 Implementation status in this repository

All three are **already implemented** as of v1.8.16:

| Requirement | Where |
|---|---|
| Ed25519 keypair, `0600` file, reuse-not-rotate, `--rotate-key` | `internal/auth/device_key.go`, `internal/commands/auth.go` |
| Public key registration + `deviceKeyId` storage | `internal/api/auth.go`, `internal/model/credentials.go` |
| Payload signing over the complete payload | `internal/commands/sync.go` |
| `signedCommitRatio` | `internal/pipeline/phase7/payload.go` |
| `previousHead` | `internal/commands/sync.go`, `internal/pipeline/phase7/payload.go` |

**Not yet done:** OS keychain storage. The private key currently always uses the
`~/.proofboard/device.key` fallback at `0600`. Moving to Keychain / Credential
Manager / Secret Service is the remaining piece of 4.1.

### 4.6 Questions still open with the backend engineer

1. **Why does `POST /api/v1/cli/auth/device-key` return 500 with no auth
   header?** Blocking; see section 2.
2. **Signature encoding** — base64 or hex? The CLI currently sends base64.
3. **Canonicalization rule** — the exact byte-level rule the server uses to
   re-verify. Needed in writing; assumptions here silently break every
   signature. The CLI currently signs `json.Marshal` of the Go struct with
   `deviceSignature` set to `""`, in declared field order.
4. **Rollout** — is `deviceSignature` optional-then-required, or required from
   day one? Required immediately breaks every CLI already installed.
5. Should `deviceName` be sent on registration, and what should it contain?

---

## 5. Payload contract

Everything previously sent is unchanged. Additions only:

```jsonc
{
  "...": "...(existing fields unchanged)...",
  "deviceKeyId": "string — from registration",
  "deviceSignature": "string — base64, over the full canonical payload",
  "antiFraudSignals": {
    "...": "...(existing)...",
    "signedCommitRatio": 0.0
  },
  "previousHead": "sha — omitted on a repo's first-ever sync"
}
```

Per the published OpenAPI `CliPayloadDto`, `deviceKeyId`, `deviceSignature` and
`previousHead` are **optional**; `signedCommitRatio` is optional within
`CliAntiFraudSignalsDto`. Required fields remain `shas`, `timestamps`,
`additions`, `deletions`, `filesChanged`, `categories`, `impactScores`,
`milestoneClusters`, `orgHash`, `emailHash`, `capturedAt`, `cliVersion`,
`dictionaryVersion`, `provider`, `antiFraudSignals`, `notifyPush`.

---

## 6. Hard constraints (non-negotiable)

Never store after Phase 5, and never transmit: commit messages, file contents,
diffs, file paths, repository names, organization names, author emails.

Only ever transmit: SHA hashes, timestamps, additions, deletions, files changed,
category labels, cluster metadata, `orgHash`, `emailHash`.

All hashes SHA256. All API calls HTTPS. All payloads JWT authenticated.
Credentials at `~/.proofboard/credentials.json` (`0600`), state at
`~/.proofboard/state.json`, logs at `~/.proofboard/sync.log`.

---

## 7. How to verify work in this repository

```sh
scripts/test_no_system_pollution.sh go test -count=1 ./...   # hermetic unit + integration
scripts/test_no_system_pollution.sh go test -race -count=1 ./...
go vet ./...
npm --prefix npm-package test
scripts/test_compiled_cli.sh /tmp/proofboard-compiled "$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/version/version.go)"
```

`scripts/test_no_system_pollution.sh` isolates `HOME` and fails if the run
touches `/usr/local/bin/proofboard`. Use it for anything that installs.

Releasing: bump the version in all 14 locations (`internal/version/version.go`,
`npm-package/{index.js,package.json}`, `packaging/windows/ProofboardCareerAgent.iss`,
`scripts/install.{sh,ps1}`, and the 8 synced rules/spec documents), update the
notes block in `.github/workflows/release.yml`, then tag `vX.Y.Z` and push. CI
builds all four targets, signs them, builds the native installers, and publishes
the release with the install scripts attached.
