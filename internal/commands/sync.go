package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/proofboard/proofboard/internal/dictionary"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/pipeline"
	"github.com/proofboard/proofboard/internal/pipeline/phase1"
	"github.com/spf13/cobra"
)

func newSyncCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var incremental bool
	var skipHandshake bool
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
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			repoState, linked := current.LinkedRepos[identity.RepoHash]
			if !linked {
				return fmt.Errorf("repository is not linked; run proofboard link first")
			}
			if fromHook {
				branch, err := pbgit.CurrentBranch(ctx, repo)
				if err != nil {
					return err
				}
				if !pbgit.IsProductionBranch(branch, repoState.ProductionBranches) {
					return nil
				}
				changed, err := hooks.PostRewrite(ctx, repo, repoState.LastHeadSHA)
				if err != nil {
					return err
				}
				if !changed {
					return nil
				}
			}
			lastSHA := ""
			if incremental {
				lastSHA = repoState.LastHeadSHA
			}
			dict, err := dictionary.LoadDefault(ctx)
			if err != nil {
				return err
			}
			if err := dictionary.Validate(dict); err != nil {
				return err
			}
			if verbose {
				fmt.Fprintln(out, "Phase 1: ingest")
			}
			raw, err := phase1.Ingest(ctx, repo, lastSHA)
			if err != nil {
				return err
			}
			if len(raw) == 0 {
				_, err := fmt.Fprintln(out, "No commits to sync.")
				return err
			}
			handshakeStatus := "success"
			if verbose {
				fmt.Fprintln(out, "Phase 6: handshake")
			}
			if err := pbgit.LSRemoteHandshake(ctx, repo, 10*time.Second); err != nil {
				if !skipHandshake {
					return fmt.Errorf("Remote handshake timed out. If you are on a corporate network with VPN or SSH proxy restrictions, retry with: proofboard sync --skip-handshake")
				}
				if repoState.LastHandshake.IsZero() {
					return fmt.Errorf("No prior handshake recorded for this repository. A successful handshake must occur at least once during active employment to qualify for SHA Proof. Connect to your organisation network and retry without --skip-handshake")
				}
				handshakeStatus = "skipped"
			}
			if verbose {
				fmt.Fprintln(out, "Phases 2-5: classify, score, cluster, shred")
			}
			payload, err := pipeline.New(dict).Run(ctx, pipeline.RunInput{
				Raw:             raw,
				OrgHash:         identity.OrgHash,
				RepoHash:        identity.RepoHash,
				EmailHash:       credentials.EmailHash,
				HandshakeStatus: handshakeStatus,
				ExpectedOrgHash: repoState.OrgHash,
			})
			if err != nil {
				return err
			}
			if verbose {
				fmt.Fprintln(out, "Phase 8: transmit")
			}
			receipt, err := runtime.api.Sync(ctx, credentials.Token, payload)
			if err != nil {
				return fmt.Errorf("transmit sync payload: %w", err)
			}
			head, err := pbgit.Head(ctx, repo)
			if err != nil {
				return err
			}
			repoState.LastHeadSHA = head
			repoState.LastSyncAt = time.Now().UTC()
			repoState.DictionaryVersion = dict.Version
			if handshakeStatus == "success" {
				repoState.LastHandshake = repoState.LastSyncAt
				repoState.Tier = "Tier2"
			} else {
				repoState.Tier = "Tier2-skipped"
			}
			current.LinkedRepos[identity.RepoHash] = repoState
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			tier := receipt.Tier
			if tier == "" {
				tier = repoState.Tier
			}
			_, err = fmt.Fprintf(out, "Synced %d commits. Categories detected: %d. Tier achieved: %s.\n", len(payload.SHAs), len(payload.ImpactScores), tier)
			return err
		},
	}
	cmd.Flags().BoolVar(&incremental, "incremental", false, "sync only commits since the last recorded HEAD")
	cmd.Flags().BoolVar(&skipHandshake, "skip-handshake", false, "continue only if this repo has a prior successful handshake")
	cmd.Flags().BoolVar(&fromHook, "from-hook", false, "run silent hook gating before syncing")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print pipeline steps")
	return cmd
}
