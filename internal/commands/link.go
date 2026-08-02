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
	"github.com/proofboard/proofboard/internal/detection"
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

// frontendStackSignals / backendStackSignals / mobileLanguageSignals back
// inferRoleTitle's heuristic: it only reads labels detection.DetectStack
// already produced (framework/tool names, language extension counts) — never
// file contents — consistent with the same "local detection surfaces a
// value, human confirms" rule the org/company prompt follows.
var frontendStackSignals = map[string]bool{
	"React": true, "Next.js": true, "Vue": true, "Angular": true,
	"Svelte": true, "Tailwind CSS": true,
}
var backendStackSignals = map[string]bool{
	"Express": true, "NestJS": true, "Fastify": true, "Gin": true,
	"Echo": true, "Fiber": true, "Cobra": true, "Django": true, "Flask": true,
	"FastAPI": true, "Laravel": true, "Symfony": true, "Ruby on Rails": true,
	"Sinatra": true, "Actix Web": true, "Rocket": true, "Axum": true,
}
var mobileLanguageSignals = map[string]bool{
	"Swift": true, "Kotlin": true, "Objective-C": true,
}

// inferRoleTitle suggests a role from the locally-detected tech stack — never
// asserted outright, only offered as an editable default the user confirms
// or overrides in promptForCompanyAndRole.
func inferRoleTitle(stack *model.StackReport) string {
	if stack == nil {
		return ""
	}
	frontend, backend := false, false
	for _, t := range stack.TechStack {
		if frontendStackSignals[t] {
			frontend = true
		}
		if backendStackSignals[t] {
			backend = true
		}
	}
	mobile := false
	for lang := range stack.Languages {
		if mobileLanguageSignals[lang] {
			mobile = true
		}
	}
	switch {
	case frontend && backend:
		return "Full-Stack Engineer"
	case frontend:
		return "Frontend Engineer"
	case backend:
		return "Backend Engineer"
	case mobile:
		return "Mobile Engineer"
	default:
		return ""
	}
}

// promptForCompanyAndRole is the interactive, human-confirmed company/role
// autofill: it never guesses from repo/org *content* (commit messages, file
// structure) — only the org name already computed locally by
// pbgit.ParseRemote, and the tech-stack labels already computed locally by
// detection.DetectStack, are offered back to the user to confirm or edit.
// Nothing is sent unless the user confirms. Callers must skip this entirely
// in non-interactive/agent runs.
//
// Declining the detected organisation does not skip the prompt outright —
// the user still gets to type their own company name and confirm/edit a
// role title, so there's always a chance to fill both in rather than leave
// them at the backend's placeholder.
func promptForCompanyAndRole(in io.Reader, out io.Writer, org string, stack *model.StackReport) (companyName string, roleTitle string) {
	reader := bufio.NewReader(in)

	if strings.TrimSpace(org) != "" {
		fmt.Fprintf(out, "Detected organisation: %s\n", org)
		fmt.Fprintf(out, "Is this your employer/client for this project? [Y/n]: ")
		line, _ := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer == "n" || answer == "no" {
			fmt.Fprintf(out, "Company name (optional, press enter to skip): ")
			companyLine, _ := reader.ReadString('\n')
			companyName = strings.TrimSpace(companyLine)
		} else {
			companyName = org
		}
	} else {
		fmt.Fprintf(out, "Company name (optional, press enter to skip): ")
		companyLine, _ := reader.ReadString('\n')
		companyName = strings.TrimSpace(companyLine)
	}

	suggestedRole := inferRoleTitle(stack)
	if suggestedRole != "" {
		fmt.Fprintf(out, "Role title [%s] (press enter to accept, or type your own): ", suggestedRole)
	} else {
		fmt.Fprintf(out, "Role title (optional, press enter to skip): ")
	}
	roleLine, _ := reader.ReadString('\n')
	roleTitle = strings.TrimSpace(roleLine)
	if roleTitle == "" {
		roleTitle = suggestedRole
	}
	return companyName, roleTitle
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
					checkErr := withSpinner(out, "Checking project status…", !nonInteractive, func() error {
						return retryAfterAuth(ctx, out, "project setup", func() error {
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
					})
					if checkErr == nil && checkRes.IsLinked {
						fmt.Fprintln(out, "Repository is already linked to Proofboard.")
						return nil
					}
					// If backend check failed or returned not linked, proceed with link
				}
			}

			// Best-effort local stack detection — never block or fail
			// link on a detection error, the repo may simply not be a
			// perfectly clean git checkout yet.
			var stack *model.StackReport
			if report, detectErr := detection.DetectStack(repo.Path); detectErr == nil {
				stack = &report
			}

			// Human-confirmed company/role autofill. Only offered
			// interactively — never in --non-interactive/agent-triggered
			// runs — and only applied by the backend if this request ends up
			// creating a brand new project (never overwrites an existing
			// one's values).
			var companyName, roleTitle string
			if !nonInteractive {
				companyName, roleTitle = promptForCompanyAndRole(os.Stdin, out, identity.Org, stack)
			}

			// Call 1
			req := api.LinkRequest{
				OrgHash:  identity.OrgHash,
				RepoHash: identity.RepoHash,
				Provider: identity.Provider,
				Handshake: &api.LinkHandshake{
					SSHTest: true,
				},
				Stack:       stack,
				CompanyName: companyName,
				RoleTitle:   roleTitle,
			}
			var response api.LinkResponse
			err = withSpinner(out, "Registering project…", !nonInteractive, func() error {
				return retryAfterAuth(ctx, out, "project setup", func() error {
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
					existingID, createNew := promptForProject(os.Stdin, out, response.ExistingProjectOptions)
					req.ExistingProjectID = existingID
					req.CreateNew = createNew
				}
				err = withSpinner(out, "Registering project…", !nonInteractive, func() error {
					return retryAfterAuth(ctx, out, "project setup", func() error {
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
				})
				if err != nil {
					return fmt.Errorf("register linked repository selection: %w", err)
				}
			} else if response.IsNewProject && response.ProjectID == "" {
				req.CreateNew = true
				err = withSpinner(out, "Registering project…", !nonInteractive, func() error {
					return retryAfterAuth(ctx, out, "project setup", func() error {
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
