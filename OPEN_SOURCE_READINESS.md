# Open-source readiness — what to remove before making this repo public

Generated from a direct audit of the tracked files in this repository (not a guess — every
path below was confirmed with `git ls-files` / `git log` before being listed). Nothing has
been deleted. This is the punch list for you to review and execute.

Current repo size: `.git` alone is **103MB**, working tree is **230MB**. Most of that is
compiled binaries and text dumps that should never have been committed.

## 1. Delete outright — internal AI-agent working notes (highest priority)

This is the most important one. `.agents/` is **284 tracked files** of AI-agent-session
scratch state — orchestrator/worker/auditor/explorer/challenger working directories, each
with `BRIEFING.md`, `ORIGINAL_REQUEST.md`, `handoff.md`, `progress.md`, `audit_report.md`.
This is internal build-process exhaust, not project history anyone outside your team should
see.

```
git rm -r .agents/
```

Root-level files with the same character (agent handoff/assessment notes from working
sessions, not documentation):

```
git rm handoff.md progress.md report.md error-report.md backend-pr-proposal.md
git rm auth_output.txt link_output.txt   # both 0 bytes, dead scratch files
git rm temp_changes.txt                   # 223KB scratch dump
```

`docs/lead-dev-trust-signing-raw-prompt.md` is explicitly labeled "Raw input... saved as-is"
— a verbatim prompt dump. Delete or fold whatever's still useful into `docs/cli-trust-and-signing-handoff.md`, then delete the raw one:

```
git rm docs/lead-dev-trust-signing-raw-prompt.md
```

`docs/cli-trust-and-signing-handoff.md` itself reads as an internal engineering handoff note
too ("Portable brief... written so it can be handed to another engineer or agent harness").
Worth a skim before deciding whether it belongs in public `docs/` or should move to an
internal wiki instead.

## 2. Delete outright — committed binaries (the other big one)

Compiled release binaries should never be in source control — they belong in GitHub Releases
(your `.github/workflows/release.yml` already builds and attaches them there). Right now
you have **two full copies** committed:

```
git rm -r dist_original/                  # 39MB — proofboard-{darwin-amd64,darwin-arm64,linux-amd64,windows-amd64.exe}
git rm proofboard-darwin-amd64 proofboard-darwin-arm64 proofboard-linux-amd64 proofboard-windows-amd64.exe
git rm proofboard.sig
```

Add to `.gitignore` (it currently only ignores the bare `proofboard`/`proofboard.exe` names,
not the per-platform ones):

```
dist_original/
proofboard-darwin-*
proofboard-linux-*
proofboard-windows-*
*.sig
```

## 3. Review, probably remove — internal spec PDF

`Proofboard_Spec_v1_8.pdf` (301KB, tracked) is a product/business spec document. `SPEC.md`
is already the documented "normative specification" per your own `CLAUDE.md` — the PDF is
either redundant with it or an older/different-audience version. If it contains anything
not meant for public reading (pricing, roadmap, internal strategy), pull it before going
public:

```
git rm Proofboard_Spec_v1_8.pdf
```

## 4. Dead code found while doing the notification cleanup you asked for

These are now genuinely unreachable, not "maybe unused" — confirmed by grepping every call
site:

- `internal/notifications/system.go` — `Dispatch()` and `NotifyDesktop()` were already removed
  in this session (zero remaining callers once the project-detected OS popup was cut over to
  terminal-only). Already done, mentioned here for the record.
- `github.com/gen2brain/beeep` in `go.mod` — was only used by `NotifyDesktop`, now unused.
  Run `go mod tidy` after confirming nothing else references it.
- `internal/notifications/workspace.go` — `ActivateWorkspaceAction`'s `"review"`/`"publish"`/
  `"ignore"` cases and the `runMilestoneAction` OS-notification-click path are only reachable
  from an OS popup that no longer exists (milestone notifications are terminal-only now, per
  today's change). They're harmless (only reachable via the hidden `notify-activate` command,
  used for manual testing), but worth a deliberate decision: keep for future notification
  work, or strip along with the rest of the interactive-popup machinery.

## 5. Judgment calls — not obviously wrong, but worth deciding on purpose

- **`.kiro/steering/project-rules.md`, `AGENTS.md`, `GEMINI.md`, `.cursorrules`,
  `.windsurfrules`, `.github/copilot-instructions.md`, `CLAUDE.md`** — all identical content
  (per your own documented sync policy), all agent-instruction files. Increasingly normal to
  ship in public repos now, but decide deliberately whether you want your internal engineering
  conventions, coding rules, and release policy visible to the world. Nothing sensitive found
  in a skim, just flagging the decision.
- **`docs/career-agent-website-copy.md`** — marketing copy handoff, not code documentation.
  Probably fine to keep, but it's an odd fit for a CLI repo's `docs/` folder; consider whether
  it belongs in the frontend/marketing repo instead.
- **`Makefile`, `build_release.sh`** — legitimate, keep.

## 6. Before you actually flip the repo to public

- **Git history still has the binaries even after `git rm`.** A `git rm` only removes the
  file going forward — the 100MB+ of binary blobs stays in every historical commit, so
  `.git` stays bloated and (more importantly) anyone can `git log` / `git show` their way to
  every deleted file, including everything in `.agents/`. If any of that content is sensitive,
  a plain `git rm` is not enough — you'd need a history rewrite (`git filter-repo` or BFG)
  before making the repo public, and that's a destructive, coordinate-with-everyone-first
  operation. **I have not done this and would not do it without you explicitly asking for it
  and confirming no one else has local clones that would break.**
- **Rotate/verify secrets.** Skim `.env.example`, any committed config, and the `packaging/`
  scripts for anything that looks like a real key, token, or internal URL rather than a
  placeholder — a quick `git log -p -- '*.env*'` and a search for `sk_`, `_SECRET`, `_KEY`
  patterns across history is worth doing given the history-rewrite point above.
- **Double check `LICENSE`** matches what you actually intend for a public repo (currently
  present, worth a final read).

## Summary — commands to run (after you've reviewed each section above)

```bash
git rm -r .agents/
git rm handoff.md progress.md report.md error-report.md backend-pr-proposal.md
git rm auth_output.txt link_output.txt temp_changes.txt
git rm docs/lead-dev-trust-signing-raw-prompt.md
git rm -r dist_original/
git rm proofboard-darwin-amd64 proofboard-darwin-arm64 proofboard-linux-amd64 proofboard-windows-amd64.exe
git rm proofboard.sig
git rm Proofboard_Spec_v1_8.pdf   # only if it shouldn't be public
go mod tidy                        # after confirming beeep has no remaining references
```

Then decide on section 5's judgment calls, and read section 6 before actually flipping
visibility — the git-history point matters more than any of the file deletions above.
