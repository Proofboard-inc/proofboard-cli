package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/detection"
	"github.com/proofboard/proofboard/internal/dictionary"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/notifications"
	"github.com/proofboard/proofboard/internal/pipeline"
	"github.com/proofboard/proofboard/internal/pipeline/phase1"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/style"
	"github.com/spf13/cobra"
)

func newSyncCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var incremental bool
	var full bool
	var fromHook bool
	var fromAgent bool
	var verbose bool
	var resync bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run local ingest, anonymization, and sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			if fromAgent {
				if deferred, deferErr := deferExpiredAgentSession(ctx, runtime, out); deferErr != nil {
					return fmt.Errorf("check Career Agent session: %w", deferErr)
				} else if deferred {
					return nil
				}
			}
			credentials, err := loadOrAuthCredentials(ctx, out, runtime)
			if err != nil {
				return fmt.Errorf("authenticate: %w", err)
			}
			repo, err := pbgit.Discover(ctx, runtime.workingDir)
			if err != nil {
				if isNotGitRepoError(err) {
					return fmt.Errorf("not a git repository (or any parent up to the filesystem root) — run `proofboard sync` from inside a git repository")
				}
				return err
			}
			remoteURL, err := pbgit.OriginURL(ctx, repo)
			if err != nil {
				return err
			}
			identity, err := pbgit.ParseRemote(remoteURL)
			if err != nil {
				return err
			}
			triggerSource := "manual"
			if fromAgent {
				triggerSource = "agent"
			} else if fromHook {
				triggerSource = "hook"
			}
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "start", "success", "")

			current, err := runtime.state.Load(ctx)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "load state", "failure", err.Error())
				return err
			}
			repoState, linked := current.LinkedRepos[identity.RepoHash]
			if !linked {
				repoPath, err := filepath.Abs(repo.Path)
				if err != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "failure", err.Error())
					return err
				}
				if statestore.IsWorkspaceSuppressed(current, repoPath) {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "workspace suppressed")
					return nil
				}
				if fromHook {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "unlinked repo in hook")
					return nil
				}
				notifications.PrintEvent(out, notifications.NewProjectDetected(identity.Repo))
				fmt.Fprintln(out, "Preparing this project for automatic tracking...")
				linkCmd := newLinkCommand(ctx, out)
				if fromAgent {
					linkCmd.SetArgs([]string{"--non-interactive"})
				} else {
					linkCmd.SetArgs([]string{})
				}
				if err := linkCmd.ExecuteContext(ctx); err != nil {
					return err
				}
				current, err = runtime.state.Load(ctx)
				if err != nil {
					return err
				}
				repoState, linked = current.LinkedRepos[identity.RepoHash]
				if !linked {
					return fmt.Errorf("project connection did not complete")
				}
			}
			if repoState.EmailHashKey == "" {
				// Legacy state predates per-project email HMAC keys. Recover it
				// through the same documented handshake as `proofboard link`;
				// sending the stale project ID as the initial request can return
				// 404 after the backend's repository mapping has changed.
				linkCmd := newLinkCommand(ctx, out)
				linkCmd.SetArgs([]string{"--non-interactive"})
				if err := linkCmd.ExecuteContext(ctx); err != nil {
					return fmt.Errorf("refresh project security keys: %w", err)
				}
				current, err = runtime.state.Load(ctx)
				if err != nil {
					return fmt.Errorf("reload project security keys: %w", err)
				}
				var refreshed bool
				repoState, refreshed = current.LinkedRepos[identity.RepoHash]
				if !refreshed || repoState.EmailHashKey == "" {
					return fmt.Errorf("refresh project security keys: link response did not include emailHashKey")
				}
			}
			if resync {
				// Pure replay of the last transmitted payload: no git ingest,
				// no reclassification, no pipeline run, just re-sign the
				// cached content with `Regenerate: true` and resend it.
				if repoState.LastSyncPayload == nil {
					if _, printErr := fmt.Fprintln(out, `Nothing to resync yet — run "proofboard sync" first.`); printErr != nil {
						return printErr
					}
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "resync", "skipped", "no cached payload")
					return fmt.Errorf("resync: no cached sync payload for this repository yet")
				}
				candidate := *repoState.LastSyncPayload
				candidate.Regenerate = true
				receipt, _, transmitErr := transmitSyncPayload(ctx, out, runtime, identity, triggerSource, fromAgent, candidate)
				if errors.Is(transmitErr, errAgentReconnectRequired) {
					return nil
				}
				if transmitErr != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 8: Transmission", "failure", transmitErr.Error())
					var apiErr *api.Error
					if errors.As(transmitErr, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
						if apiErr.RetryAfter > 0 {
							if _, printErr := fmt.Fprintf(out, "Regenerate already requested recently — try again in %s.\n", formatRetryDuration(apiErr.RetryAfter)); printErr != nil {
								return printErr
							}
						} else if _, printErr := fmt.Fprintln(out, "Regenerate already requested recently — try again shortly."); printErr != nil {
							return printErr
						}
						return fmt.Errorf("resync throttled: %w", transmitErr)
					}
					return fmt.Errorf("transmit resync payload: %w", transmitErr)
				}
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 8: Transmission", "success", "")
				if err := reportSyncOutcome(out, receipt, candidate, false); err != nil {
					return err
				}
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "complete", "success", "")
				return nil
			}

			metadataHash, err := pbgit.MetadataFingerprint(ctx, repo)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "repository metadata", "failure", err.Error())
				return err
			}
			metadataChanged := repoState.MetadataHash == "" || repoState.MetadataHash != metadataHash
			if fromHook {
				branch, err := pbgit.CurrentBranch(ctx, repo)
				if err != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "check branch", "failure", err.Error())
					return err
				}
				allowedBranches := current.WatchedBranches
				if len(repoState.ProductionBranches) > 0 {
					allowedBranches = append(append([]string{}, allowedBranches...), repoState.ProductionBranches...)
				}
				if !pbgit.IsProductionBranch(branch, allowedBranches) {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "branch filter", "skipped", "not a production branch")
					return nil
				}
				changed, err := hooks.PostRewrite(ctx, repo, repoState.LastHeadSHA)
				if err != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "post-rewrite hook check", "failure", err.Error())
					return err
				}
				if !changed {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "post-rewrite hook check", "skipped", "no new commits detected")
					return nil
				}
			}
			lastSHA := ""
			if incremental && !full {
				lastSHA = repoState.LastHeadSHA
			}
			dict, err := dictionary.LoadDefault(ctx)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "load dictionary", "failure", err.Error())
				return err
			}
			if err := dictionary.Validate(dict); err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "validate dictionary", "failure", err.Error())
				return err
			}
			if verbose {
				fmt.Fprintln(out, "Phase 1: ingest")
			}
			raw, err := phase1.Ingest(ctx, repo, lastSHA)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 1: Ingest", "failure", err.Error())
				return err
			}
			if identity.Provider == "github" && len(raw) > 0 {
				verified, verifyErr := pbgit.VerifyProviderContribution(ctx, identity, credentials.Username)
				switch {
				case verifyErr != nil:
					_ = logging.WriteSyncLog(
						runtime.homeDir,
						identity.RepoHash,
						triggerSource,
						"public provider enrichment",
						"skipped",
						"provider signals unavailable; exact local identity attribution retained",
					)
				case verified.Private:
					_ = logging.WriteSyncLog(
						runtime.homeDir,
						identity.RepoHash,
						triggerSource,
						"public provider enrichment",
						"skipped",
						"private repository; local analysis only",
					)
				default:
					enriched, applied := pbgit.ApplyPublicGitHubSignals(raw, verified)
					if applied {
						raw = enriched
						_ = logging.WriteSyncLog(
							runtime.homeDir,
							identity.RepoHash,
							triggerSource,
							"public provider enrichment",
							"success",
							"public GitHub contribution signals applied",
						)
					} else {
						_ = logging.WriteSyncLog(
							runtime.homeDir,
							identity.RepoHash,
							triggerSource,
							"public provider enrichment",
							"skipped",
							"no matching provider signal; exact local identity attribution retained",
						)
					}
				}
			}
			if len(raw) == 0 && !metadataChanged {
				_, err := fmt.Fprintln(out, "No commits to sync.")
				if err == nil && triggerSource == "manual" {
					_, err = fmt.Fprintln(out, "If this looks wrong: check `git config user.email` matches an email on your Proofboard account, and that this branch is tracked (`proofboard config add-branch <name>`).")
				}
				if err == nil && triggerSource == "manual" && repoState.LastSyncPayload != nil {
					_, err = fmt.Fprintln(out, "To regenerate your milestone summaries without new commits, run: proofboard sync --resync")
				}
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 1: Ingest", "skipped", "no new commits to sync")
				return err
			}

			// Pre-Classification Trivial Commit Filter checks (before they are classified / subjects zeroed)
			shouldAbort := false

			if fromHook {
				// a. Single commit: Range has exactly 1 commit.
				if len(raw) == 1 {
					shouldAbort = true
				}

				// b. Documentation only: All files changed are docs (.md, .txt, README, CHANGELOG, LICENSE, .rst)
				if !shouldAbort && len(raw) > 0 {
					hasFiles := false
					isDocOnly := true
					for _, commit := range raw {
						for _, fp := range commit.FilePaths {
							hasFiles = true
							if !isDocFile(fp) {
								isDocOnly = false
								break
							}
						}
						if !isDocOnly {
							break
						}
					}
					if hasFiles && isDocOnly {
						shouldAbort = true
					}
				}

				// d. Revert-only range: All commits are reverts (e.g. subject starts with "revert:" case-insensitive).
				if !shouldAbort {
					isRevertOnly := true
					for _, commit := range raw {
						if !isRevertSubject(commit.Subject) {
							isRevertOnly = false
							break
						}
					}
					if isRevertOnly {
						shouldAbort = true
					}
				}
			}

			if shouldAbort {
				return abortSyncWithTrigger(runtime.homeDir, identity.RepoHash, triggerSource)
			}

			mergeTimestamps, err := pbgit.MergeTimestamps(ctx, repo)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "MergeTimestamps", "failure", err.Error())
				return err
			}
			previousHead := repoState.LastHeadSHA
			payloadEmailHash := credentials.EmailHash
			identityEmailHash := credentials.EmailHash
			if repoState.EmailHashKey != "" {
				gitEmail, emailErr := pbgit.UserEmail(ctx, repo)
				if emailErr != nil {
					return fmt.Errorf("read local git identity: %w", emailErr)
				}
				identityEmailHash = crypto.NormalizedSHA256(gitEmail)
				payloadEmailHash, emailErr = crypto.NormalizedHMACSHA256(repoState.EmailHashKey, gitEmail)
				gitEmail = ""
				if emailErr != nil {
					return fmt.Errorf("hash local git identity: %w", emailErr)
				}
			}

			if verbose {
				fmt.Fprintln(out, "Phases 2-5: classify, score, cluster, shred")
			}
			// Best-effort local stack detection, refreshed on every
			// sync; never block or fail a sync over a detection error.
			var stack *model.StackReport
			if report, detectErr := detection.DetectStack(repo.Path, dict); detectErr == nil {
				stack = &report
			}
			// Manifest-based detection only sees dependency names, so it can
			// easily miss an industry a project genuinely operates in (e.g. a
			// logistics integration called over raw HTTP, no matching npm
			// dependency). Merge in whatever commit-subject text also
			// resolves, before raw is handed to the pipeline below (whose
			// Phase 2 zeroes Subject as part of shredding): manifest matches
			// first (stronger signal), then any additional subject-based
			// matches not already present, capped at the same limit.
			if stack != nil {
				for _, hint := range detection.IndustryHintsFromCommits(raw, dict) {
					if len(stack.IndustryHints) >= detection.MaxIndustryHints {
						break
					}
					already := false
					for _, existing := range stack.IndustryHints {
						if existing == hint {
							already = true
							break
						}
					}
					if !already {
						stack.IndustryHints = append(stack.IndustryHints, hint)
					}
				}
			}
			// Was this sync's commits captured from the repo's actual
			// detected default branch, or a manually-`add-branch`'d one?
			// Never sent as a branch name, just this boolean, so the
			// backend can weight SHA-proof trust accordingly without the
			// CLI transmitting anything proprietary/identifying. Fails open
			// (true) only when the current branch itself can't be determined
			// at all; when it can, but the repo's real default can't be
			// detected (e.g. a self-hosted remote with no origin/HEAD
			// symref), falls back to whether the branch is at least among
			// the recognized watched-branches set rather than penalizing
			// every such setup by default.
			isDefaultBranch := true
			if currentBranch, branchErr := pbgit.CurrentBranch(ctx, repo); branchErr == nil && currentBranch != "" {
				// Local ref only. Sync runs unattended from post-commit and
				// post-merge hooks and from the background agent, so it must
				// never wait on the network: asking the remote here is what
				// left a stuck git process behind on every commit against a
				// remote that wanted credentials. When the local ref is
				// absent the watched-branch fallback below answers instead,
				// which costs a little accuracy and no hangs.
				if detected := localDefaultBranch(ctx, repo.Path); detected != "" {
					isDefaultBranch = currentBranch == detected
				} else {
					isDefaultBranch = pbgit.IsProductionBranch(currentBranch, current.WatchedBranches)
				}
			}
			payload, err := pipeline.New(dict).Run(ctx, pipeline.RunInput{
				Raw:               raw,
				OrgHash:           identity.OrgHash,
				RepoHash:          identity.RepoHash,
				EmailHash:         payloadEmailHash,
				IdentityEmailHash: identityEmailHash,
				Provider:          identity.Provider,
				ExpectedOrgHash:   repoState.OrgHash,
				MergeTimestamps:   mergeTimestamps,
				PreviousHead:      previousHead,
				Stack:             stack,
				IsDefaultBranch:   isDefaultBranch,
			})
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phases 2-5: Pipeline", "failure", err.Error())
				return err
			}
			if len(raw) == 0 && metadataChanged {
				payload.AntiFraudSignals.LowCommitCount = false
			}
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phases 2-5: Pipeline", "success", "")

			// c. High boilerplate noise: Average aiNoiseScore (AINoiseScore) across all commits in the range > 0.85
			if fromHook && payload.AntiFraudSignals.AINoiseScore > 0.85 {
				return abortSyncWithTrigger(runtime.homeDir, identity.RepoHash, triggerSource)
			}

			if verbose {
				fmt.Fprintln(out, "Phase 6: transmit")
			}
			// receipt/transmittedPayload are populated by transmitSyncPayload
			// on a successful call to runtime.api.Sync, read after it
			// returns to decide what the final print should say (see the
			// receipt.Status branch inside reportSyncOutcome) and to cache
			// the exact signed payload that left the machine for a future
			// `sync --resync`.
			receipt, transmittedPayload, err := transmitSyncPayload(ctx, out, runtime, identity, triggerSource, fromAgent, payload)
			if errors.Is(err, errAgentReconnectRequired) {
				return nil
			}
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 8: Transmission", "failure", err.Error())
				return fmt.Errorf("transmit sync payload: %w", err)
			}
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 8: Transmission", "success", "")
			head, err := pbgit.Head(ctx, repo)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "get HEAD", "failure", err.Error())
				return err
			}
			repoState.LastHeadSHA = head
			repoState.LastSyncAt = time.Now().UTC()
			repoState.DictionaryVersion = dict.Version
			repoState.MetadataHash = metadataHash
			// Cache the exact transmitted (signed) payload so a future
			// `sync --resync` can replay it verbatim without re-ingesting
			// git history.
			repoState.LastSyncPayload = &transmittedPayload
			current, err = runtime.state.Load(ctx)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "load state", "failure", err.Error())
				return err
			}
			current.LinkedRepos[identity.RepoHash] = repoState
			if err := runtime.state.Save(ctx, current); err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "save state", "failure", err.Error())
				return err
			}
			// The backend's response to this sync's payload, not the
			// earlier "nothing to send at all" short-circuit above (that
			// one never reaches the network). A server-reported dedup/
			// regenerate status means the CLI DID transmit a payload, but
			// the server determined it already has this exact content
			// (e.g. a metadata-only change whose content-hash matches what
			// it already stored), so the "✓ Synced" message below would be
			// misleading if printed unconditionally.
			if err := reportSyncOutcome(out, receipt, transmittedPayload, len(transmittedPayload.SHAs) == 0 && metadataChanged); err != nil {
				return err
			}
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "complete", "success", "")
			return nil
		},
	}
	cmd.Flags().BoolVar(&incremental, "incremental", true, "sync only commits since the last recorded HEAD")
	cmd.Flags().BoolVar(&full, "full", false, "reprocess the entire commit history from the beginning, ignoring the last recorded HEAD")
	cmd.Flags().BoolVar(&fromHook, "from-hook", false, "run silent hook gating before syncing")
	cmd.Flags().BoolVar(&fromAgent, "agent", false, "run as a non-interactive Career Agent sync")
	_ = cmd.Flags().MarkHidden("agent")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print pipeline steps")
	cmd.Flags().BoolVar(&resync, "resync", false, "regenerate milestone summaries for your last-synced commits without new commits")
	return cmd
}

// transmitSyncPayload signs payload with the device key and sends it via
// runtime.api.Sync, threading the same auth-retry/spinner machinery and
// no-linked-project org/repo-hash swap that every sync (including a
// `sync --resync` replay) relies on. Returns the receipt and the payload as
// actually signed and transmitted (including the final DeviceKeyID/
// DeviceSignature) so callers can cache exactly what left the machine.
func transmitSyncPayload(ctx context.Context, out io.Writer, runtime runtimeContext, identity model.RemoteIdentity, triggerSource string, fromAgent bool, payload model.SyncPayload) (model.SyncReceipt, model.SyncPayload, error) {
	var receipt model.SyncReceipt
	var transmitted model.SyncPayload
	transmit := func() error {
		freshCredentials, err := runtime.credentials.Load(ctx)
		if err != nil {
			return fmt.Errorf("reload credentials: %w", err)
		}
		if freshCredentials.Token == "" {
			return fmt.Errorf("missing authentication token")
		}
		signedPayload := payload
		// Device signing is mandatory: the backend unconditionally
		// rejects any sync payload missing deviceKeyId/deviceSignature
		// (cli-ingest.service.ts), so there is no optional path here.
		keyStore := pbauth.NewDeviceKeyStore(runtime.homeDir)
		deviceKey, keyErr := keyStore.Ensure(ctx, runtime.api, freshCredentials.Token, false)
		if keyErr != nil {
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource,
				"register device key", "warning", keyErr.Error())
			return fmt.Errorf("register device signing key: %w", keyErr)
		}
		freshCredentials.DeviceKeyID = deviceKey.DeviceKeyID
		if err := runtime.credentials.Save(ctx, freshCredentials); err != nil {
			return fmt.Errorf("persist device key id: %w", err)
		}
		signedPayload.DeviceKeyID = deviceKey.DeviceKeyID

		// The signature has to cover the payload exactly as sent, so it
		// is recomputed whenever the payload changes.
		sign := func(candidate model.SyncPayload) (model.SyncPayload, error) {
			candidate.DeviceSignature = ""
			signingBytes, err := crypto.CanonicalJSON(candidate)
			if err != nil {
				return candidate, fmt.Errorf("marshal sync payload for signing: %w", err)
			}
			signature, err := keyStore.Sign(ctx, signingBytes)
			if err != nil {
				return candidate, fmt.Errorf("sign sync payload: %w", err)
			}
			candidate.DeviceSignature = signature
			return candidate, nil
		}

		signedPayload, signErr := sign(signedPayload)
		if signErr != nil {
			return signErr
		}
		syncReceipt, syncErr := runtime.api.Sync(ctx, freshCredentials.Token, signedPayload)
		if isNoLinkedProjectError(syncErr) {
			signedPayload.OrgHash, signedPayload.RepoHash = signedPayload.RepoHash, signedPayload.OrgHash
			signedPayload, signErr = sign(signedPayload)
			if signErr != nil {
				return signErr
			}
			syncReceipt, syncErr = runtime.api.Sync(ctx, freshCredentials.Token, signedPayload)
		}
		if syncErr == nil {
			receipt = syncReceipt
			transmitted = signedPayload
		}
		return syncErr
	}
	err := withSpinner(out, "Transmitting proof…", triggerSource == "manual", func() error {
		if fromAgent {
			return retryAfterAuthForAgent(ctx, out, runtime, transmit)
		}
		return retryAfterAuth(ctx, out, "project synchronization", transmit)
	})
	return receipt, transmitted, err
}

// reportSyncOutcome prints the final user-facing line(s) for a completed
// transmit, branching on the backend's receipt status. Shared by the normal
// sync flow and `sync --resync` so both surface identical wording for
// "deduped"/"regenerating" responses.
func reportSyncOutcome(out io.Writer, receipt model.SyncReceipt, payload model.SyncPayload, metadataOnly bool) error {
	switch receipt.Status {
	case "deduped", "duplicate":
		if _, err := fmt.Fprintln(out, "No new commits since your last sync (nothing changed on the server)."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "To regenerate your milestone summaries without new commits, run: proofboard sync --resync"); err != nil {
			return err
		}
	case "regenerating":
		if _, err := fmt.Fprintf(out, "%s %s %s\n",
			style.Success(out, "✓"),
			style.Brand(out, "Proofboard"),
			style.Heading(out, "— Regenerate requested for your milestone summaries. Refresh your dashboard shortly to see updated text.")); err != nil {
			return err
		}
	default:
		// Print one live line per detected cluster as they're found, so
		// every category synced this run is visible in real time. This is
		// informational only: the backend's clustering/AI-summary pass is
		// still async at this point, so there's nothing to review yet.
		// The actionable "ready to review" prompt surfaces later, once
		// that finishes, via the sync-complete notification (see
		// `proofboard notices`, wired into shell startup).
		for _, cluster := range payload.MilestoneClusters {
			fmt.Fprintln(out, style.ClusterLine(out, cluster.Category, cluster.ImpactType, cluster.ImpactScale, cluster.CommitCount))
		}
		var err error
		if len(payload.SHAs) == 0 && metadataOnly {
			_, err = fmt.Fprintf(out, "%s %s %s\n",
				style.Success(out, "✓"), style.Brand(out, "Proofboard"), style.Heading(out, "— Repository metadata synchronized."))
		} else {
			_, err = fmt.Fprintf(out, "%s %s %s\n",
				style.Success(out, "✓"), style.Brand(out, "Proofboard"),
				style.Heading(out, fmt.Sprintf("— Synced %d commits. Clusters detected: %d.", len(payload.SHAs), len(payload.MilestoneClusters))))
		}
		if err != nil {
			return err
		}
		if len(payload.MilestoneClusters) > 0 {
			if _, err := fmt.Fprintln(out, style.Muted(out, "Finishing analysis — check your dashboard shortly to review and publish.")); err != nil {
				return err
			}
		}
	}
	return nil
}

// formatRetryDuration renders a Retry-After duration as a whole-minute,
// human-readable hint ("15m"), rounding up so the hint never undersells how
// long the throttle window actually is.
func formatRetryDuration(d time.Duration) string {
	minutes := int((d + time.Minute - 1) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%dm", minutes)
}

func isNoLinkedProjectError(err error) bool {
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return strings.Contains(strings.ToLower(apiErr.Message), "no linked project")
	}
	return false
}

func deferExpiredAgentSession(ctx context.Context, runtime runtimeContext, out io.Writer) (bool, error) {
	if current, err := runtime.state.Load(ctx); err == nil && current.AuthLoggedOut {
		return true, nil
	}
	credentials, err := runtime.credentials.Load(ctx)
	if err != nil {
		return false, nil
	}
	if credentialsNeedRefresh(credentials) && credentials.RefreshToken != "" {
		service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
		if _, refreshErr := service.Refresh(ctx, credentials); refreshErr == nil {
			return false, nil
		} else if !isAuthFailure(refreshErr) {
			return false, refreshErr
		}
		if err := promptAgentReconnect(ctx, out, runtime); err != nil {
			return false, err
		}
		return true, nil
	}
	if !credentialsCompletelyExpired(credentials) {
		return false, nil
	}
	if err := promptAgentReconnect(ctx, out, runtime); err != nil {
		return false, err
	}
	return true, nil
}

func isDocFile(filePath string) bool {
	lower := strings.ToLower(filepath.Base(filePath))
	if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".rst") {
		return true
	}
	if strings.HasPrefix(lower, "readme") || strings.HasPrefix(lower, "changelog") || strings.HasPrefix(lower, "license") {
		return true
	}
	return false
}

func isRevertSubject(subject []byte) bool {
	trimmed := bytes.TrimSpace(subject)
	prefix := [...]byte{'r', 'e', 'v', 'e', 'r', 't', ':'}
	return len(trimmed) >= len(prefix) && bytes.EqualFold(trimmed[:len(prefix)], prefix[:])
}

func abortSync(homeDir, repoHash string) error {
	return abortSyncWithTrigger(homeDir, repoHash, "manual")
}

func abortSyncWithTrigger(homeDir, repoHash, triggerSource string) error {
	return logging.WriteSyncLog(homeDir, repoHash, triggerSource, "pre-classification filter", "aborted", "trivial merge skipped")
}
