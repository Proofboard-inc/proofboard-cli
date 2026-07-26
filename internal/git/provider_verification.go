package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

const providerVerificationTimeout = 45 * time.Second

type ProviderVerification struct {
	Login      string
	Repository string
	Private    bool
	Admin      bool
	CommitSHAs map[string]struct{}
}

func VerifyProviderContribution(
	ctx context.Context,
	identity model.RemoteIdentity,
	proofboardUsername string,
) (ProviderVerification, error) {
	if identity.Provider != "github" {
		return ProviderVerification{}, fmt.Errorf(
			"provider verification is not available for %s repositories; refusing to link without verified ownership or contributions",
			identity.Provider,
		)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return ProviderVerification{}, errors.New(
			"GitHub verification requires GitHub CLI; install gh and run gh auth login",
		)
	}

	verifyCtx, cancel := context.WithTimeout(ctx, providerVerificationTimeout)
	defer cancel()

	var viewer struct {
		Login string `json:"login"`
	}
	if err := ghAPIJSON(verifyCtx, []string{"user"}, &viewer); err != nil {
		return ProviderVerification{}, err
	}
	if viewer.Login == "" {
		return ProviderVerification{}, errors.New("GitHub verification returned no authenticated account")
	}
	if proofboardUsername == "" || !strings.EqualFold(viewer.Login, proofboardUsername) {
		return ProviderVerification{}, fmt.Errorf(
			"GitHub account %q does not match the authenticated Proofboard account; use the same account in gh auth login and proofboard auth",
			viewer.Login,
		)
	}

	repositoryPath := url.PathEscape(identity.Org) + "/" + url.PathEscape(identity.Repo)
	var repository struct {
		FullName    string `json:"full_name"`
		Private     bool   `json:"private"`
		Permissions struct {
			Admin bool `json:"admin"`
		} `json:"permissions"`
	}
	if err := ghAPIJSON(verifyCtx, []string{"repos/" + repositoryPath}, &repository); err != nil {
		return ProviderVerification{}, fmt.Errorf(
			"verify current GitHub repository access: %w",
			err,
		)
	}

	commitOutput, err := ghAPI(
		verifyCtx,
		"api",
		"--paginate",
		"--method",
		"GET",
		"repos/"+repositoryPath+"/commits",
		"-f",
		"author="+viewer.Login,
		"-f",
		"per_page=100",
		"--jq",
		".[].sha",
	)
	if err != nil {
		return ProviderVerification{}, fmt.Errorf("verify GitHub contributions: %w", err)
	}
	commitSHAs := make(map[string]struct{})
	for _, line := range strings.Split(string(commitOutput), "\n") {
		sha := strings.TrimSpace(line)
		if sha != "" {
			commitSHAs[sha] = struct{}{}
		}
	}
	if !repository.Permissions.Admin && len(commitSHAs) == 0 {
		return ProviderVerification{}, errors.New(
			"repository rejected: the authenticated GitHub account is not an administrator and GitHub attributes no commits in its default history to that account",
		)
	}

	return ProviderVerification{
		Login:      viewer.Login,
		Repository: repository.FullName,
		Private:    repository.Private,
		Admin:      repository.Permissions.Admin,
		CommitSHAs: commitSHAs,
	}, nil
}

func FilterProviderVerifiedCommits(
	commits []model.RawCommit,
	verified ProviderVerification,
) []model.RawCommit {
	filtered := make([]model.RawCommit, 0, len(commits))
	for _, commit := range commits {
		if _, ok := verified.CommitSHAs[commit.SHA]; ok {
			filtered = append(filtered, commit)
		}
	}
	return filtered
}

func ghAPIJSON(ctx context.Context, route []string, target any) error {
	args := append([]string{"api"}, route...)
	output, err := ghAPI(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("decode GitHub verification response: %w", err)
	}
	return nil
}

func ghAPI(ctx context.Context, args ...string) ([]byte, error) {
	run := func(environment []string) ([]byte, error) {
		command := exec.CommandContext(ctx, "gh", args...)
		command.Env = environment
		output, err := command.Output()
		if err != nil {
			return nil, err
		}
		return output, nil
	}

	output, err := run(os.Environ())
	if err == nil {
		return output, nil
	}
	if os.Getenv("GITHUB_TOKEN") != "" || os.Getenv("GH_TOKEN") != "" {
		filtered := make([]string, 0, len(os.Environ()))
		for _, entry := range os.Environ() {
			if strings.HasPrefix(entry, "GITHUB_TOKEN=") || strings.HasPrefix(entry, "GH_TOKEN=") {
				continue
			}
			filtered = append(filtered, entry)
		}
		if fallbackOutput, fallbackErr := run(filtered); fallbackErr == nil {
			return fallbackOutput, nil
		}
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("GitHub verification timed out: %w", ctx.Err())
	}
	return nil, errors.New("GitHub verification failed; run gh auth login with access to this repository")
}
