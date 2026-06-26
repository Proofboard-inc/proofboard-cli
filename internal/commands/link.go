package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/crypto"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

func detectDefaultBranch(ctx context.Context, repoPath string) string {
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err == nil {
		str := strings.TrimSpace(string(out))
		if strings.HasPrefix(str, "refs/remotes/origin/") {
			return strings.TrimPrefix(str, "refs/remotes/origin/")
		}
	}
	cmd = exec.CommandContext(ctx, "git", "remote", "show", "origin")
	cmd.Dir = repoPath
	out, err = cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "HEAD branch:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
			}
		}
	}
	return ""
}

func promptForBranch(in io.Reader, out io.Writer) string {
	reader := bufio.NewReader(in)
	for {
		fmt.Fprintf(out, "Could not detect default branch from remote.\nEnter the branch name you ship from (e.g. main, develop): ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return "main"
		}
		branch := strings.TrimSpace(line)
		if branch != "" {
			return branch
		}
	}
}

func promptForProject(in io.Reader, out io.Writer, options []api.ExistingProjectOption) (string, bool) {
	fmt.Fprintf(out, "This repo is not linked yet. Found existing projects:\n")
	for i, opt := range options {
		fmt.Fprintf(out, "  %d  %-15s %s\n", i+1, opt.Name, opt.Role)
	}
	fmt.Fprintf(out, "  n  Create a new project\n")
	
	reader := bufio.NewReader(in)
	for {
		fmt.Fprintf(out, "Choose [1-%d/n]: ", len(options))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", true
		}
		choice := strings.TrimSpace(line)
		if choice == "n" || choice == "N" {
			return "", true
		}
		idx, err := strconv.Atoi(choice)
		if err == nil && idx >= 1 && idx <= len(options) {
			return options[idx-1].ID, false
		}
	}
}

func newLinkCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "link",
		Short: "Link the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("link: %w", err)
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
			
			// Call 1
			req := api.LinkRequest{
				OrgHash:  identity.OrgHash,
				RepoHash: identity.RepoHash,
				Provider: identity.Provider,
			}
			response, err := runtime.api.Link(ctx, credentials.Token, req)
			if err != nil {
				return fmt.Errorf("register linked repository: %w", err)
			}

			if response.IsNewProject && len(response.ExistingProjectOptions) > 0 {
				existingID, createNew := promptForProject(os.Stdin, out, response.ExistingProjectOptions)
				req.ExistingProjectID = existingID
				req.CreateNew = createNew
				response, err = runtime.api.Link(ctx, credentials.Token, req)
				if err != nil {
					return fmt.Errorf("register linked repository selection: %w", err)
				}
			} else if response.IsNewProject {
				req.CreateNew = true
				response, err = runtime.api.Link(ctx, credentials.Token, req)
				if err != nil {
					return fmt.Errorf("register linked repository (new): %w", err)
				}
			}

			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			head, _ := pbgit.Head(ctx, repo)
			
			branch := detectDefaultBranch(ctx, repo.Path)
			if branch == "" {
				branch = promptForBranch(os.Stdin, out)
			}
			fmt.Fprintf(out, "Tracking branch: %s. Add others with: proofboard config add-branch <name>\n", branch)

			current = statestore.AddLinkedRepo(current, model.LinkedRepoState{
				RepoHash:           identity.RepoHash,
				OrgHash:            identity.OrgHash,
				PathHash:           crypto.SHA256(repo.Path),
				Provider:           identity.Provider,
				LastHeadSHA:        head,
				LastSyncAt:         time.Time{},
				ProjectID:          response.ProjectID,
				PublicKey:          response.PublicKey,
				DictionaryVersion:  response.DictionaryVersion,
				ProductionBranches: []string{branch},
			})
			if err := hooks.Install(ctx, repo); err != nil {
				return err
			}
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "Linked repository successfully. Hooks installed.\n")
			return err
		},
	}
}
