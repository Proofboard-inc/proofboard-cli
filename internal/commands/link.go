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
	var nonInteractive bool
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Connect the current repository for advanced workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("link: %w", err)
			}
			_, err = loadOrAuthCredentials(ctx, out, runtime)
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
			// Check if already linked locally and on backend
			current, err := runtime.state.Load(ctx)
			if err == nil {
				if linkedRepo, ok := current.LinkedRepos[identity.RepoHash]; ok && linkedRepo.EmailHashKey != "" {
					// Verify with backend
					var checkRes api.CheckResponse
					checkErr := retryAfterAuth(ctx, out, "project setup", func() error {
						freshCredentials, loadErr := runtime.credentials.Load(ctx)
						if loadErr != nil {
							return fmt.Errorf("reload credentials: %w", loadErr)
						}
						if freshCredentials.Token == "" {
							return fmt.Errorf("missing authentication token")
						}
						var checkErr error
						checkRes, checkErr = runtime.api.Check(ctx, freshCredentials.Token, identity.OrgHash)
						return checkErr
					})
					if checkErr == nil && checkRes.IsLinked {
						fmt.Fprintln(out, "Repository is already linked to Proofboard.")
						return nil
					}
					// If backend check failed or returned not linked, proceed with link
				}
			}

			// Call 1
			req := api.LinkRequest{
				OrgHash:  identity.OrgHash,
				RepoHash: identity.RepoHash,
				Provider: identity.Provider,
				Handshake: &api.LinkHandshake{
					SSHTest: true,
				},
			}
			var response api.LinkResponse
			err = retryAfterAuth(ctx, out, "project setup", func() error {
				freshCredentials, err := runtime.credentials.Load(ctx)
				if err != nil {
					return fmt.Errorf("reload credentials: %w", err)
				}
				if freshCredentials.Token == "" {
					return fmt.Errorf("missing authentication token")
				}
				var linkErr error
				response, linkErr = runtime.api.Link(ctx, freshCredentials.Token, req)
				return linkErr
			})
			if err != nil {
				return fmt.Errorf("register linked repository: %w", err)
			}

			if response.IsNewProject && len(response.ExistingProjectOptions) > 0 {
				if nonInteractive {
					existingProjectID := current.LinkedRepos[identity.RepoHash].ProjectID
					for _, option := range response.ExistingProjectOptions {
						if existingProjectID != "" && option.ID == existingProjectID {
							req.ExistingProjectID = existingProjectID
							break
						}
					}
					if req.ExistingProjectID == "" {
						req.CreateNew = true
					}
				} else {
					fmt.Fprintf(out, "Detected organisation: %s\n", identity.Org)
					existingID, createNew := promptForProject(os.Stdin, out, response.ExistingProjectOptions)
					req.ExistingProjectID = existingID
					req.CreateNew = createNew
				}
				err = retryAfterAuth(ctx, out, "project setup", func() error {
					freshCredentials, err := runtime.credentials.Load(ctx)
					if err != nil {
						return fmt.Errorf("reload credentials: %w", err)
					}
					if freshCredentials.Token == "" {
						return fmt.Errorf("missing authentication token")
					}
					var linkErr error
					response, linkErr = runtime.api.Link(ctx, freshCredentials.Token, req)
					return linkErr
				})
				if err != nil {
					return fmt.Errorf("register linked repository selection: %w", err)
				}
			} else if response.IsNewProject && response.ProjectID == "" {
				req.CreateNew = true
				err = retryAfterAuth(ctx, out, "project setup", func() error {
					freshCredentials, err := runtime.credentials.Load(ctx)
					if err != nil {
						return fmt.Errorf("reload credentials: %w", err)
					}
					if freshCredentials.Token == "" {
						return fmt.Errorf("missing authentication token")
					}
					var linkErr error
					response, linkErr = runtime.api.Link(ctx, freshCredentials.Token, req)
					return linkErr
				})
				if err != nil {
					return fmt.Errorf("register linked repository (new): %w", err)
				}
			}

			current, err = runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			branch := detectDefaultBranch(ctx, repo.Path)
			if branch == "" {
				if nonInteractive {
					branch = "main"
				} else {
					branch = promptForBranch(os.Stdin, out)
				}
			}
			if !nonInteractive {
				fmt.Fprintf(out, "Tracking branch: %s. Add others with: proofboard config add-branch <name>\n", branch)
			}

			existingRepoState := current.LinkedRepos[identity.RepoHash]
			linkedRepoState := model.LinkedRepoState{
				RepoHash:           identity.RepoHash,
				OrgHash:            identity.OrgHash,
				PathHash:           crypto.SHA256(repo.Path),
				Provider:           identity.Provider,
				ProjectID:          response.ProjectID,
				PublicKey:          response.PublicKey,
				EmailHashKey:       response.EmailHashKey,
				DictionaryVersion:  response.DictionaryVersion,
				ProductionBranches: []string{branch},
				LastHeadSHA:        existingRepoState.LastHeadSHA,
				LastSyncAt:         existingRepoState.LastSyncAt,
				LastHandshake:      existingRepoState.LastHandshake,
				MetadataHash:       existingRepoState.MetadataHash,
			}
			if linkedRepoState.ProjectID == "" {
				linkedRepoState.ProjectID = existingRepoState.ProjectID
			}
			if linkedRepoState.PublicKey == "" {
				linkedRepoState.PublicKey = existingRepoState.PublicKey
			}
			if linkedRepoState.EmailHashKey == "" {
				linkedRepoState.EmailHashKey = existingRepoState.EmailHashKey
			}
			if linkedRepoState.DictionaryVersion == "" {
				linkedRepoState.DictionaryVersion = existingRepoState.DictionaryVersion
			}
			current = statestore.AddLinkedRepo(current, linkedRepoState)
			metadataHash, metadataErr := pbgit.MetadataFingerprint(ctx, repo)
			if metadataErr == nil {
				repoState := current.LinkedRepos[identity.RepoHash]
				repoState.MetadataHash = metadataHash
				current.LinkedRepos[identity.RepoHash] = repoState
			}
			if err := hooks.Install(ctx, repo); err != nil {
				return err
			}
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "Project connected. Proofboard Career Agent is tracking it automatically.\n")
			return err
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "connect the project without terminal prompts")
	_ = cmd.Flags().MarkHidden("non-interactive")
	return cmd
}
