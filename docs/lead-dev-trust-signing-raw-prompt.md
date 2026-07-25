# Raw input — Proofboard CLI Trust & Signing (verbatim)

This file is a portable copy of the lead developer's brief and the OpenAPI
spec, saved as-is so it can be handed to a different harness/agent if needed.
For the synthesized, verified version of this work (including live endpoint
probes and current implementation status), see
`docs/cli-trust-and-signing-handoff.md` in this same directory.

---

## Original Windows terminal output that triggered this investigation

```
PS C:\Users\DELL\Downloads\sigmadev-repos\igamer-app-js> proofboard sync
Connect your Proofboard Career Agent.

Your device code is: ZM5P-FLU3

Opening browser to connect your Career Agent...
If it does not open automatically, navigate to:
https://proofboard-frontend.vercel.app/cli-auth?code=ZM5P-FLU3

Waiting for authentication...
authenticate: connect Career Agent: ensure device key: API returned 500 Internal Server Error: {"statusCode":500}
PS C:\Users\DELL\Downloads\sigmadev-repos\igamer-app-js> proofboard log
unknown command "log" for "proofboard"

Did you mean this?
        logs

PS C:\Users\DELL\Downloads\sigmadev-repos\igamer-app-js> proofboard logs
2026-07-04T09:58:59Z — 4c501cb9f09356ef2b4fe5fcf5f316f1181de9925b33540b7af86df849198a18 — manual — start — success
2026-07-22T11:12:25Z — shell-hooks — maintenance — shell hook check — success — workspace detection hook installed
2026-07-22T11:13:58Z — shell-hooks — maintenance — shell hook check — skipped — workspace detection hook already present
2026-07-22T11:14:02Z — shell-hooks — maintenance — shell hook check — skipped — workspace detection hook already present
2026-07-22T11:14:12Z — shell-hooks — maintenance — shell hook check — skipped — workspace detection hook already present
2026-07-22T11:14:23Z — shell-hooks — maintenance — shell hook check — skipped — workspace detection hook already present
2026-07-22T11:14:25Z — HTTP GET — [REDACTED] — REQUEST (none) — RESPONSE (401 Unauthorized) {"statusCode":401,"timestamp":"2026-07-22T11:14:25.556Z","path":"/api/v1/notifications?isRead=false&limit=20&page=1","message":"Unauthorized"}
2026-07-22T11:14:27Z — HTTP POST — [REDACTED] — REQUEST {"deviceCode":"E9B7-74D3"} — RESPONSE (400 Bad Request) {"statusCode":400,"timestamp":"2026-07-22T11:14:27.335Z","path":"/api/v1/cli/auth/device-code","message":["property deviceCode should not exist"]}
```

---

## Lead developer's message (verbatim)

> Good day bro, quick heads up we're building fast so some of the
> security/auth stuff keeps shifting a bit as we go, apologies for that...
> since we're building for developers though, we gotta close every loophole
> we can so it actually feels solid and professional, not sloppy. Appreciate
> you rolling with the changes 🙏
>
> test the dev endpoints raw yourself, directly, don't make assumptions.
> https://api-dev.proofboard.io/docs

---

## Lead developer's brief: "Proofboard CLI — New Trust & Signing Flow"

**Audience:** CLI engineer (Go) only. This document describes what the CLI
needs to build and send. It does not describe backend implementation — you
don't need any of that to do this work, just the contract described below.

**Why this exists:** today, anyone holding a valid CLI login token can bypass
the CLI entirely and `curl` a fake sync payload straight to the API. This
round of work closes that gap by having each CLI installation prove —
cryptographically — that every payload it sends really came from that
installation, plus adds two extra trust signals the CLI is well-positioned to
provide that Proofboard currently has no way to get.

**What this does NOT do:** none of this proves the underlying commit data is
real. A user who owns their laptop can still type fake numbers into a payload
before signing it — signing only proves *"this payload came from the CLI
installation this user authenticated with,"* not *"this work happened."*
That's a known, permanent limit of any client-side trust mechanism — keep it
in mind, it's not something this work is trying to solve.

### 1. What stays exactly the same

- **The `proofboard auth` device-code flow is unchanged.** The server mints
  both the device code and the human-facing user code; the CLI just displays
  the code and polls. No changes needed here.
- **The `emailHash` field is unchanged.** The CLI already sends
  `emailHash = SHA256(email)` (plain hash), and the backend already verifies
  it correctly. Nothing to touch.
- **The overall sync flow shape is unchanged:** `proofboard auth` →
  `proofboard link` → git hook fires on merge → CLI reads own commits →
  categorizes/scores/clusters → shreds sensitive data → POSTs JSON payload.
  All of that stays as-is. This document only adds new fields to the payload
  and one new one-time step at auth.

### 2. What's new — three additions

1. A device keypair, generated once per installation, used to sign every sync
   payload.
2. A signed-commit ratio, computed from data git already gives you for free.
3. A `previousHead` field, so the backend can sanity-check for history
   rewriting between syncs.

### 3. New: Device key generation & payload signing

#### 3.1 Generate a keypair on `proofboard auth`

The first time a CLI installation completes `proofboard auth` successfully
(i.e. right after the device-code flow finishes and the CLI has its login
token), generate an **Ed25519** keypair locally. Ed25519 over RSA: smaller
signatures, faster to sign/verify, and it's what Proofboard's own release-key
signing already uses, so you're matching an existing convention rather than
introducing a new one.

**Storage — in priority order:**
1. OS keychain if available: macOS Keychain, Windows Credential Manager,
   Linux Secret Service (via whichever Go keychain library the CLI already
   uses for the login token, if it uses one — otherwise pick a well-maintained
   cross-platform one).
2. Fallback: a file in the CLI's config directory (e.g.
   `~/.proofboard/device.key`) with `0600` permissions, if no keychain is
   available on the platform.

**Never** log, print, or transmit the private key anywhere. Only the public
key ever leaves the machine.

#### 3.2 Register the public key with the server

Immediately after generating the keypair, send the **public key** to the
server's device-key registration endpoint (ask the backend engineer for the
exact route/payload shape when this lands — the CLI side just needs: send
public key, receive back a `deviceKeyId` string). Store that `deviceKeyId`
locally alongside the private key — you'll need to send it with every future
sync payload so the server knows which key to verify against (a user may have
more than one machine/installation).

#### 3.3 Re-auth on the same machine: reuse the key, don't rotate

If `proofboard auth` runs again on a machine that already has a valid local
device key, **reuse the existing keypair** — don't generate a new one. Only
rotate if:
- The user explicitly runs `proofboard auth --rotate-key` (a new flag to
  add), or
- The local key file/keychain entry is missing or corrupted.

Rotating on every login would be unnecessary churn and would force the server
to track more key history than it needs to.

#### 3.4 Sign every sync payload

On every `proofboard sync` (whatever your internal command/hook name is for
the POST to the sync endpoint):

1. **Canonicalize the payload before signing.** This is the part most likely
   to cause subtle bugs if skipped: JSON key order must be **deterministic**,
   not whatever your JSON marshaller happens to produce (Go's default map
   iteration order is randomized). Marshal into a struct with a fixed field
   order, or explicitly sort keys before signing, so the same logical payload
   always produces the exact same bytes to sign — otherwise the server's
   independently-recomputed signature check will fail even for a legitimate,
   unmodified payload.
2. **Sign the canonical bytes** with the device private key.
3. **Add two new fields to the payload**, alongside everything you already
   send:
   - `deviceKeyId` — the ID you got back when registering the public key.
   - `deviceSignature` — the signature itself (send as base64 or hex —
     confirm which with the backend engineer, but base64 is the more common
     convention).

The signature must cover the **entire** payload including the two new fields
described below (`signedCommitRatio`, `previousHead`) — sign after you've
built the complete payload, not before.

#### 3.5 What this closes

Once this ships, a payload without a valid signature — or signed with a key
the server doesn't recognize for that user/device — gets rejected, even with
a stolen or leaked login token. This is the main point of this whole round of
work; everything else below is smaller, additive trust signals.

### 4. New: Signed-commit ratio

Git already tells you which of a user's commits are cryptographically signed
(`%G?` in `git log` — or whatever your existing commit-walking code already
extracts per commit). This is one of the only trust signals available that
isn't purely self-reported: a user can't fake a GPG/SSH-signed commit without
an actual matching key pair being set up in their git config.

**What to do:**
1. While walking the user's own commits for a sync (the same pass you already
   do for categorization/clustering), also capture the signed-status per
   commit.
2. Compute: `signedCommitRatio = signedCommits / totalCommits` (a float
   between 0 and 1).
3. Add `signedCommitRatio` to the payload's anti-fraud/signals section
   (wherever your existing fraud-signal fields like noise-score or
   low-commit-count flags live — this is a sibling field to those).

This field is covered by the signature in section 3 — compute it before you
canonicalize-and-sign, not after.

### 5. New: `previousHead` (history-rewrite detection)

Nothing today stops a user from rewriting local git history between syncs to
inflate their timeline. This field gives the backend a way to sanity-check
for that, without ever needing to see actual commit content.

**What to do:**
1. After a successful sync of a given repo, store the HEAD commit SHA you
   just synced up to, **locally**, per-repo (e.g. in a small state file
   alongside wherever the CLI already tracks per-repo sync state —
   gitignored, never committed).
2. On the **next** sync of that same repo, include `previousHead` in the
   payload: the HEAD SHA you stored last time.
3. On the **first-ever** sync of a repo, omit `previousHead` (or send it as
   `null`/empty — confirm the exact convention with the backend engineer, but
   omitting the field entirely if you have no prior value is the simplest
   approach).
4. If your local state file is ever missing or unreadable (corrupted,
   deleted, user cleared it), treat the repo as if it's a first-ever sync —
   send no `previousHead` rather than crashing or guessing.

This field is also covered by the signature — same rule as above,
compute/read it before signing.

### 6. Payload shape — summary of every new field

Everything you already send stays the same. These are the only additions:

```json
{
  "...": "...(everything already sent, unchanged)...",
  "deviceKeyId": "string — from the one-time registration step in 3.2",
  "deviceSignature": "string (base64/hex — confirm encoding) — signature over the full canonical payload",
  "antiFraudSignals": {
    "...": "...(existing fields unchanged)...",
    "signedCommitRatio": 0.0
  },
  "previousHead": "string, sha — omit or null on a repo's first-ever sync"
}
```

### 7. Suggested build order

1. **Signed-commit ratio** (section 4) — no dependency on anything else,
   cheapest to ship, testable in isolation.
2. **`previousHead` tracking** (section 5) — needs the local per-repo state
   file, independent of signing.
3. **Device key generation + signing** (section 3) — the biggest piece.
   Needs the backend's registration endpoint to exist before the CLI can
   register keys, so coordinate timing with the backend engineer rather than
   shipping this ahead of the server side being ready. Consider making
   `deviceSignature` optional-but-sent during a rollout window (so old CLI
   versions don't suddenly get rejected) until the backend flips to requiring
   it.

### 8. Questions to confirm with the backend engineer before starting section 3

- Exact URL/method for the device-key registration endpoint, and its exact
  request/response shape.
- Signature encoding: base64 or hex.
- Exact canonicalization rule the server will use to independently re-verify
  your signature — you and the server must produce byte-for-byte identical
  canonical payloads, or every signature will fail verification even when
  nothing is wrong. Get this in writing, don't assume.
- Rollout plan: will `deviceSignature` be optional-then-required, or required
  from day one (which would break any CLI version already in the wild until
  users upgrade)?

---

## OpenAPI spec, as fetched from https://api-dev.proofboard.io/docs

The full JSON is large (~2500 lines). Rather than duplicate it a second time
in this repository, fetch it live when needed:

```sh
curl -sS https://api-dev.proofboard.io/docs-json   # or /openapi.json per the description field
```

Key facts extracted from it, relevant to sections above:

- `servers` block lists three hosts: `https://proboardly-backend.onrender.com`
  ("Development Environment"), `https://api.proofboard.app` ("Production"),
  `https://api-dev.proofboard.app` ("Development"). **The two `.app` hosts do
  not resolve via DNS** as of 2026-07-25 — verified with direct `curl`. Only
  the Render host (and its mirror `api-dev.proofboard.io`) actually serves
  traffic.
- `POST /api/v1/cli/auth/device-key` — operationId
  `CliAuthController_registerDeviceKey`, summary: "Register this machine's
  public key so future sync payloads can be signed and verified server-side
  (see cli-ingest.service.ts). Called by `proofboard auth` after the
  device-code flow completes. POST /api/v1/cli/auth/device-key — requires CLI
  token." Body: `RegisterDeviceKeyDto { publicKey: string (required),
  deviceName?: string }`.
- `POST /api/v1/cli/auth/device-code` — server generates both `deviceCode`
  (secret, for polling) and `userCode` (shown to human). Body:
  `CliDeviceCodeDto { deviceName?: string }`.
- `GET /api/v1/cli/auth/poll/{deviceCode}` — public, no auth, polled every 3s.
- `PUT /api/v1/cli/auth/device-code/{code}/approve` — called by the web app
  with a web JWT when the developer clicks Confirm; `:code` is the
  user-facing code, not the secret device code.
- `DELETE /api/v1/cli/auth/device-key/{deviceKeyId}` — revokes a device key
  from the dashboard, requires web JWT.
- `POST /api/v1/cli/sync` — `CliPayloadDto`. `deviceKeyId`, `deviceSignature`,
  and `previousHead` are all **optional** top-level fields in the current
  schema; `signedCommitRatio` is optional inside `CliAntiFraudSignalsDto`.
  Required fields: `shas`, `timestamps`, `additions`, `deletions`,
  `filesChanged`, `categories`, `impactScores`, `milestoneClusters`,
  `orgHash`, `emailHash`, `capturedAt`, `cliVersion`, `dictionaryVersion`,
  `provider`, `antiFraudSignals`, `notifyPush`.

If the full raw JSON is needed verbatim for another harness, re-fetch it —
pasting the entire ~2500-line document here would make this file unwieldy and
it will drift from the live spec anyway.
