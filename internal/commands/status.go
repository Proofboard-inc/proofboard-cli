package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/spf13/cobra"
)

func newStatusCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Proofboard Career Agent status",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			running, _ := agentRunning(runtime.homeDir)
			runningLabel := "Stopped"
			if running {
				runningLabel = "Running locally"
			}
			lastSync := time.Time{}
			for _, repoState := range current.LinkedRepos {
				if repoState.LastSyncAt.After(lastSync) {
					lastSync = repoState.LastSyncAt
				}
			}
			lastSyncLabel := "Never"
			if !lastSync.IsZero() {
				lastSyncLabel = relativeSyncTime(time.Now(), lastSync)
			}
			authLabel := "Not connected"
			if credentials, loadErr := runtime.credentials.Load(ctx); loadErr == nil && credentials.Token != "" {
				authLabel = "Connected"
				if !credentials.ExpiresAt.IsZero() && time.Now().After(credentials.ExpiresAt) && credentials.RefreshToken == "" {
					authLabel = "Reconnect required"
				}
			}
			if jsonOutput {
				status := struct {
					Product              string `json:"product"`
					Active               bool   `json:"active"`
					RunningLocally       bool   `json:"runningLocally"`
					LastSync             string `json:"lastSync,omitempty"`
					RepositoriesTracked  int    `json:"repositoriesTracked"`
					AuthenticationStatus string `json:"authenticationStatus"`
				}{
					Product:              "Proofboard Career Agent",
					Active:               running,
					RunningLocally:       running,
					RepositoriesTracked:  len(current.LinkedRepos),
					AuthenticationStatus: authLabel,
				}
				if !lastSync.IsZero() {
					status.LastSync = lastSync.UTC().Format(time.RFC3339)
				}
				return json.NewEncoder(out).Encode(status)
			}
			fmt.Fprintf(out, "Proofboard Career Agent\n%s\nLast sync: %s\nTracking %d repositories\nAuthentication: %s\n",
				runningLabel, lastSyncLabel, len(current.LinkedRepos), authLabel)
			if len(current.LinkedRepos) == 0 {
				return nil
			}

			// Detect current directory repo information
			var currentRepoHash string
			var localHeadSHA string
			var localMetadataHash string
			var checkErr error

			repo, err := pbgit.Discover(ctx, runtime.workingDir)
			if err == nil {
				remoteURL, err := pbgit.OriginURL(ctx, repo)
				if err == nil {
					identity, err := pbgit.ParseRemote(remoteURL)
					if err == nil {
						currentRepoHash = identity.RepoHash
						localHeadSHA, checkErr = pbgit.Head(ctx, repo)
						if checkErr == nil {
							localMetadataHash, checkErr = pbgit.MetadataFingerprint(ctx, repo)
						}
					} else {
						checkErr = err
					}
				} else {
					checkErr = err
				}
			} else {
				checkErr = err
			}

			hashes := make([]string, 0, len(current.LinkedRepos))
			for repoHash := range current.LinkedRepos {
				hashes = append(hashes, repoHash)
			}
			sort.Strings(hashes)
			for _, repoHash := range hashes {
				repoState := current.LinkedRepos[repoHash]
				pending := "unknown"
				if checkErr == nil && currentRepoHash != "" && repoHash == currentRepoHash {
					if localHeadSHA != repoState.LastHeadSHA || repoState.MetadataHash == "" || localMetadataHash != repoState.MetadataHash {
						pending = "yes"
					} else {
						pending = "no"
					}
				}
				fmt.Fprintf(out, "%s projectID=%s lastSync=%s lastHead=%s pending=%s\n",
					repoHash,
					repoState.ProjectID,
					repoState.LastSyncAt.Format("2006-01-02T15:04:05Z07:00"),
					repoState.LastHeadSHA,
					pending,
				)
			}
			if err := triggerMonthlyCareerSummary(ctx, out, runtime); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-readable Career Agent status")
	return cmd
}

func relativeSyncTime(now, syncedAt time.Time) string {
	age := now.Sub(syncedAt)
	if age < 0 {
		return syncedAt.UTC().Format(time.RFC3339)
	}
	if age < time.Minute {
		return "just now"
	}
	if age < time.Hour {
		minutes := int(age / time.Minute)
		return relativeCount(minutes, "minute")
	}
	if age < 24*time.Hour {
		hours := int(age / time.Hour)
		return relativeCount(hours, "hour")
	}
	days := int(age / (24 * time.Hour))
	return relativeCount(days, "day")
}

func relativeCount(count int, unit string) string {
	if count != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s ago", count, unit)
}
