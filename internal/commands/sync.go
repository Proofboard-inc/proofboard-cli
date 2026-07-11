package commands

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/dictionary"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/pipeline"
	"github.com/proofboard/proofboard/internal/pipeline/phase1"
	"github.com/proofboard/proofboard/internal/pipeline/phase2"
	"github.com/spf13/cobra"
)

func newSyncCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var incremental bool
	var fromHook bool
	var verbose bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run local ingest, anonymization, and sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			credentials, err := runtime.credentials.Load(ctx)
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
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
			if fromHook {
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
				isSuppressed := false
				for _, path := range current.SuppressedWorkspaces {
					if path == repoPath {
						isSuppressed = true
						break
					}
				}
				if isSuppressed {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "workspace suppressed")
					return nil
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

				raw, err := phase1.Ingest(ctx, repo, "")
				if err != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phase 1: Ingest", "failure", err.Error())
					return err
				}

				classified := phase2.Classify(raw, dict)
				totals := make(map[string]int)
				for _, commit := range classified {
					for category, score := range commit.CategoryScores {
						totals[category] += score
					}
				}
				type rankedCategory struct {
					name  string
					score int
				}
				var ranked []rankedCategory
				for category, score := range totals {
					if score > 0 {
						ranked = append(ranked, rankedCategory{name: category, score: score})
					}
				}
				sort.Slice(ranked, func(i, j int) bool {
					if ranked[i].score == ranked[j].score {
						return ranked[i].name < ranked[j].name
					}
					return ranked[i].score > ranked[j].score
				})
				limit := 3
				if len(ranked) < limit {
					limit = len(ranked)
				}
				var topCategories []string
				for i := 0; i < limit; i++ {
					topCategories = append(topCategories, ranked[i].name)
				}

				if fromHook {
					_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "unlinked repo in hook")
					return nil
				}
				
				fmt.Fprintln(cmd.OutOrStdout(), "Proofboard — unlinked repository detected.")
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintf(cmd.OutOrStdout(), "Project: %s\n", filepath.Base(repo.Path))
				fmt.Fprintln(cmd.OutOrStdout(), "This repository is not linked to Proofboard.")
				fmt.Fprintln(cmd.OutOrStdout(), "Please run `proofboard link` to link it to an existing project or create a new one.")
				return nil
			}
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
			if len(raw) == 0 {
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
				if !shouldAbort {
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
			})
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phases 2-5: Pipeline", "failure", err.Error())
				return err
			}
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "Phases 2-5: Pipeline", "success", "")

			// c. High boilerplate noise: Average aiNoiseScore (AINoiseScore) across all commits in the range > 0.85
			if fromHook && payload.AntiFraudSignals.AINoiseScore > 0.85 {
				return abortSyncWithTrigger(runtime.homeDir, identity.RepoHash, triggerSource)
			}

			if verbose {
				fmt.Fprintln(out, "Phase 6: transmit")
			}
			_, err = runtime.api.Sync(ctx, credentials.Token, payload)
			if err != nil && strings.Contains(err.Error(), "No linked project found") {
				payload.OrgHash, payload.RepoHash = payload.RepoHash, payload.OrgHash
				_, err = runtime.api.Sync(ctx, credentials.Token, payload)
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
			_, err = fmt.Fprintf(out, "Synced %d commits. Categories detected: %d.\n", len(payload.SHAs), len(payload.ImpactScores))
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
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print pipeline steps")
	return cmd
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



