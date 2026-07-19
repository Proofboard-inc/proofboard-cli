package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/detection"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

func newDetectCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var workspace string
	var editor string

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Inspect an opened workspace and surface link or sync actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("detect: %w", err)
			}
			if workspace == "" {
				workspace = runtime.workingDir
			}
			result, err := detection.Inspect(ctx, runtime.homeDir, workspace, editor)
			if err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, "", "detect", "failure", "inspect workspace", err.Error())
				return nil
			}

			if result.Action == detection.ActionLink {
				if err := autoLinkWorkspace(ctx, runtime, result); err != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, result.RepoHash, "detect", "failure", "auto-link", err.Error())
					return nil
				}
			}

			reason := result.Reason
			if reason == "" {
				reason = string(result.Action)
			}
			if err := logging.WriteSyncLog(runtime.homeDir, result.RepoHash, "detect", string(result.Action), reason, ""); err != nil {
				return nil
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace root to inspect")
	cmd.Flags().StringVar(&editor, "editor", "", "editor or IDE name")
	return cmd
}

func autoLinkWorkspace(ctx context.Context, runtime runtimeContext, result detection.Result) error {
	credentials, err := runtime.credentials.Load(ctx)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	repo := pbgit.Repo{Path: result.RepoPath}
	req := api.LinkRequest{
		OrgHash:   result.OrgHash,
		RepoHash:  result.RepoHash,
		Provider:  result.Provider,
		CreateNew: true,
		Handshake: &api.LinkHandshake{SSHTest: true},
	}
	response, err := runtime.api.Link(ctx, credentials.Token, req)
	if err != nil {
		return fmt.Errorf("link workspace: %w", err)
	}
	if response.ProjectID == "" {
		return fmt.Errorf("link workspace returned empty project id")
	}

	current, err := runtime.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	head, err := pbgit.Head(ctx, repo)
	if err != nil {
		return fmt.Errorf("read head: %w", err)
	}
	branch := detectDefaultBranch(ctx, repo.Path)
	if branch == "" {
		branch = "main"
	}

	current = statestore.AddLinkedRepo(current, model.LinkedRepoState{
		RepoHash:           result.RepoHash,
		OrgHash:            result.OrgHash,
		PathHash:           crypto.SHA256(repo.Path),
		Provider:           result.Provider,
		LastHeadSHA:        head,
		LastSyncAt:         time.Time{},
		ProjectID:          response.ProjectID,
		PublicKey:          response.PublicKey,
		DictionaryVersion:  response.DictionaryVersion,
		ProductionBranches: []string{branch},
	})
	if err := runtime.state.Save(ctx, current); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	if err := hooks.Install(ctx, repo); err != nil {
		return fmt.Errorf("install hooks: %w", err)
	}
	return nil
}
