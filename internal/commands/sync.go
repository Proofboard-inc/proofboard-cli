package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/dictionary"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/notifications"
	"github.com/proofboard/proofboard/internal/pipeline"
	"github.com/proofboard/proofboard/internal/pipeline/phase1"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

func newSyncCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var incremental bool
	var fromHook bool
	var fromAgent bool
	var verbose bool
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
				notifications.Dispatch(out, notifications.NewProjectDetected(identity.Repo))
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
					return fmt.Errorf("link current repository before syncing")
				}
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
				if !pbgit.IsProductionBranch(branch, current.WatchedBranches) {
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
			if incremental {
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
			if len(raw) == 0 && !metadataChanged {
				_, err := fmt.Fprintln(out, "No commits to sync.")
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
						sub := strings.ToLower(strings.TrimSpace(string(commit.Subject)))
						if !strings.HasPrefix(sub, "revert:") {
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

			if verbose {
				fmt.Fprintln(out, "Phases 2-5: classify, score, cluster, shred")
			}
			payload, err := pipeline.New(dict).Run(ctx, pipeline.RunInput{
				Raw:             raw,
				OrgHash:         identity.OrgHash,
				RepoHash:        identity.RepoHash,
				EmailHash:       credentials.EmailHash,
				Provider:        identity.Provider,
				ExpectedOrgHash: repoState.OrgHash,
				MergeTimestamps: mergeTimestamps,
				PreviousHead:    previousHead,
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
			transmit := func() error {
				freshCredentials, err := runtime.credentials.Load(ctx)
				if err != nil {
					return fmt.Errorf("reload credentials: %w", err)
				}
				if freshCredentials.Token == "" {
					return fmt.Errorf("missing authentication token")
				}
				signedPayload := payload
				keyStore := pbauth.NewDeviceKeyStore(runtime.homeDir)
				deviceKey, err := keyStore.Ensure(ctx, runtime.api, freshCredentials.Token, false)
				if err != nil {
					return fmt.Errorf("ensure device key: %w", err)
				}
				freshCredentials.DeviceKeyID = deviceKey.DeviceKeyID
				if err := runtime.credentials.Save(ctx, freshCredentials); err != nil {
					return fmt.Errorf("persist device key id: %w", err)
				}
				signedPayload.DeviceKeyID = deviceKey.DeviceKeyID
				signedPayload.DeviceSignature = ""
				signingBytes, err := json.Marshal(signedPayload)
				if err != nil {
					return fmt.Errorf("marshal sync payload for signing: %w", err)
				}
				signature, err := keyStore.Sign(ctx, signingBytes)
				if err != nil {
					return fmt.Errorf("sign sync payload: %w", err)
				}
				signedPayload.DeviceSignature = signature
				_, syncErr := runtime.api.Sync(ctx, freshCredentials.Token, signedPayload)
				if syncErr != nil && strings.Contains(syncErr.Error(), "No linked project found") {
					signedPayload.OrgHash, signedPayload.RepoHash = signedPayload.RepoHash, signedPayload.OrgHash
					signedPayload.DeviceSignature = ""
					signingBytes, err = json.Marshal(signedPayload)
					if err != nil {
						return fmt.Errorf("marshal swapped sync payload for signing: %w", err)
					}
					signature, err = keyStore.Sign(ctx, signingBytes)
					if err != nil {
						return fmt.Errorf("sign swapped sync payload: %w", err)
					}
					signedPayload.DeviceSignature = signature
					_, syncErr = runtime.api.Sync(ctx, freshCredentials.Token, signedPayload)
				}
				return syncErr
			}
			if fromAgent {
				err = retryAfterAuthForAgent(ctx, out, runtime, transmit)
			} else {
				err = retryAfterAuth(ctx, out, "proofboard sync", transmit)
			}
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
			if len(payload.MilestoneClusters) > 0 {
				bundleID, title := resolveMilestoneNotification(ctx, runtime, repoState.ProjectID, payload.MilestoneClusters[0].Category)
				notifications.PrintEvent(out, notifications.MilestoneDetected(title))
				_ = launchTargetActionNotification(ctx, "milestone", bundleID, title)
			}
			if len(payload.SHAs) == 0 && metadataChanged {
				_, err = fmt.Fprintln(out, "Repository metadata synchronized.")
			} else {
				_, err = fmt.Fprintf(out, "Synced %d commits. Categories detected: %d.\n", len(payload.SHAs), countImpactCategories(payload.ImpactScores))
			}
			if err != nil {
				return err
			}
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "complete", "success", "")
			if err := triggerMonthlyCareerSummary(ctx, out, runtime); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&incremental, "incremental", false, "sync only commits since the last recorded HEAD")
	cmd.Flags().BoolVar(&fromHook, "from-hook", false, "run silent hook gating before syncing")
	cmd.Flags().BoolVar(&fromAgent, "agent", false, "run as a non-interactive Career Agent sync")
	_ = cmd.Flags().MarkHidden("agent")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print pipeline steps")
	return cmd
}

func deferExpiredAgentSession(ctx context.Context, runtime runtimeContext, out io.Writer) (bool, error) {
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

func resolveMilestoneNotification(ctx context.Context, runtime runtimeContext, projectID, category string) (string, string) {
	title := strings.TrimSpace(category)
	if title == "" {
		title = "Engineering milestone"
	} else {
		title += " Completed"
	}
	credentials, err := runtime.credentials.Load(ctx)
	if err != nil || credentials.Token == "" {
		return "", title
	}
	bundles, err := runtime.api.GetPendingMilestoneBundles(ctx, credentials.Token, projectID, 5)
	if err != nil || len(bundles) == 0 {
		return "", title
	}
	bundle := bundles[0]
	if strings.TrimSpace(bundle.Title) != "" {
		title = strings.TrimSpace(bundle.Title)
	}
	return bundle.ID, title
}

func countImpactCategories(scores model.ImpactScores) int {
	count := 0
	for _, value := range []float64{scores.Feature, scores.Bugfix, scores.Refactor, scores.Ship, scores.Maintenance} {
		if value > 0 {
			count++
		}
	}
	return count
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

func abortSync(homeDir, repoHash string) error {
	return abortSyncWithTrigger(homeDir, repoHash, "manual")
}

func abortSyncWithTrigger(homeDir, repoHash, triggerSource string) error {
	return logging.WriteSyncLog(homeDir, repoHash, triggerSource, "pre-classification filter", "aborted", "trivial merge skipped")
}
