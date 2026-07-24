package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

// MetadataFingerprint returns a one-way fingerprint of repository metadata that
// is useful for change detection. Raw remotes and ref names never leave this
// function and are never persisted.
func MetadataFingerprint(ctx context.Context, repo Repo) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("fingerprint repository metadata: %w", err)
	}
	remoteURL, err := OriginURL(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("fingerprint origin: %w", err)
	}
	identity, err := ParseRemote(remoteURL)
	if err != nil {
		return "", fmt.Errorf("fingerprint remote identity: %w", err)
	}

	refsCmd := exec.CommandContext(ctx, "git", "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/remotes")
	refsCmd.Dir = repo.Path
	refs, err := refsCmd.Output()
	if err != nil {
		return "", fmt.Errorf("fingerprint remote refs: %w", err)
	}

	defaultBranch := ""
	branchCmd := exec.CommandContext(ctx, "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	branchCmd.Dir = repo.Path
	if output, branchErr := branchCmd.Output(); branchErr == nil {
		defaultBranch = strings.TrimSpace(string(output))
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte(identity.Provider))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(identity.OrgHash))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(identity.RepoHash))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(refs)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(defaultBranch))
	return hex.EncodeToString(hash.Sum(nil)), nil
}
