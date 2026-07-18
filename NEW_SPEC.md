# NEW_SPEC

This document records the IDE-open detection refinement for Proofboard CLI.

## Desktop Detection Update

When an engineer opens a Git workspace in an IDE, Proofboard must detect the workspace even if the engineer never runs `proofboard link` or `proofboard sync` manually.

The watcher treats both of these cases as actionable:

* Unlinked repositories.
* Linked repositories whose local workspace is open but not yet synced.

The UX must reuse the existing three-option prompt model:

* `y` links or re-syncs the repo and continues.
* `n` dismisses for the current session.
* `x` suppresses the workspace permanently.

The IDE watcher should compare the active workspace against Proofboard state, ignore already-suppressed workspaces, and surface the prompt as soon as the IDE opens the project.

## Scope Note

This is a detection refinement, not a new notification channel. It stays within the existing terminal prompt and desktop notification surface already defined in the main spec.
