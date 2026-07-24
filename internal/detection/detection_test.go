package detection

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
)

func createTempRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "init")
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git init: %v", err)
		}
	}
	cmds := [][]string{
		{"git", "-C", repoDir, "config", "user.email", "dev@example.com"},
		{"git", "-C", repoDir, "config", "user.name", "Dev User"},
		{"git", "-C", repoDir, "remote", "add", "origin", "git@github.com:Proofboard-inc/proofboard-cli.git"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("command %v: %v", args, err)
		}
	}
	filePath := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "main.go").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "feat: initial commit").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	_ = out
	return repoDir
}

func TestInspectDetectsLinkSyncAndNone(t *testing.T) {
	repoDir := createTempRepo(t)
	ctx := context.Background()
	homeDir := t.TempDir()
	head, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}

	linkedRepo, err := Inspect(ctx, homeDir, repoDir, "vscode")
	if err != nil {
		t.Fatalf("inspect linked repo: %v", err)
	}

	if linkedRepo.Action != ActionLink {
		t.Fatalf("expected link action for unlinked repo, got %q", linkedRepo.Action)
	}

	store := statestore.NewStore(homeDir)
	stateData, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	metadataHash, err := pbgit.MetadataFingerprint(ctx, pbgit.Repo{Path: repoDir})
	if err != nil {
		t.Fatalf("metadata fingerprint: %v", err)
	}
	stateData.LinkedRepos[linkedRepo.RepoHash] = model.LinkedRepoState{
		RepoHash:     linkedRepo.RepoHash,
		OrgHash:      linkedRepo.OrgHash,
		PathHash:     "",
		LastHeadSHA:  strings.TrimSpace(string(head)),
		MetadataHash: metadataHash,
	}
	if err := store.Save(ctx, stateData); err != nil {
		t.Fatalf("save state: %v", err)
	}

	synced, err := Inspect(ctx, homeDir, repoDir, "vscode")
	if err != nil {
		t.Fatalf("inspect synced repo: %v", err)
	}
	if synced.Action != ActionNone {
		t.Fatalf("expected none action for synced repo, got %q", synced.Action)
	}

	if err := exec.Command("git", "-C", repoDir, "update-ref", "refs/remotes/origin/main", strings.TrimSpace(string(head))).Run(); err != nil {
		t.Fatalf("update remote ref: %v", err)
	}
	metadataChanged, err := Inspect(ctx, homeDir, repoDir, "vscode")
	if err != nil {
		t.Fatalf("inspect metadata change: %v", err)
	}
	if metadataChanged.Action != ActionSync || !metadataChanged.MetadataChanged {
		t.Fatalf("expected metadata sync action, got %+v", metadataChanged)
	}
	metadataHash, err = pbgit.MetadataFingerprint(ctx, pbgit.Repo{Path: repoDir})
	if err != nil {
		t.Fatalf("updated metadata fingerprint: %v", err)
	}
	repoState := stateData.LinkedRepos[linkedRepo.RepoHash]
	repoState.MetadataHash = metadataHash
	stateData.LinkedRepos[linkedRepo.RepoHash] = repoState
	if err := store.Save(ctx, stateData); err != nil {
		t.Fatalf("save updated metadata state: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "feature.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "feature.go").Run(); err != nil {
		t.Fatalf("git add feature: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "feat: second commit").Run(); err != nil {
		t.Fatalf("git commit second: %v", err)
	}

	outOfDate, err := Inspect(ctx, homeDir, repoDir, "vscode")
	if err != nil {
		t.Fatalf("inspect out-of-date repo: %v", err)
	}
	if outOfDate.Action != ActionSync {
		t.Fatalf("expected sync action for out-of-date repo, got %q", outOfDate.Action)
	}
}
