package commands

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func newRepoWithRemote(t *testing.T, remote string) string {
	t.Helper()
	// Deliberately not t.TempDir(). These tests bound a git call against an
	// unreachable remote by killing it, and killing git leaves its transport
	// helper alive for a moment longer. Unix does not care — a directory can
	// be unlinked while a process still holds it — but Windows refuses, so
	// t.TempDir's automatic cleanup failed the test with "The process cannot
	// access the file because it is being used by another process" after the
	// assertion it was making had already passed.
	//
	// Cleanup here is best effort for the same reason: a leftover temp
	// directory on a CI runner is not worth failing a test that proved what it
	// set out to prove.
	dir, err := os.MkdirTemp("", "proofboard-branch-timeout-")
	if err != nil {
		t.Fatalf("create repository directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	for _, args := range [][]string{
		{"init", "-q", "--initial-branch=main"},
		{"remote", "add", "origin", remote},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

// refs/remotes/origin/HEAD is written by `git clone` and NOT by `git init`
// plus `git remote add`, so this is the ordinary state of many working
// repositories — and the state that used to send sync to the network.
func TestLocalDefaultBranchNeverTouchesTheNetwork(t *testing.T) {
	// A remote that would hang rather than refuse: a routable-but-black-holed
	// address. If localDefaultBranch consulted it, this test would sit here
	// until the go test timeout instead of returning.
	dir := newRepoWithRemote(t, "https://10.255.255.1/proofboard/does-not-exist.git")

	done := make(chan string, 1)
	go func() { done <- localDefaultBranch(context.Background(), dir) }()

	select {
	case branch := <-done:
		if branch != "" {
			t.Fatalf("expected no local default branch in a repo without refs/remotes/origin/HEAD, got %q", branch)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("localDefaultBranch blocked on an unreachable remote: it must only read the local ref")
	}
}

// The interactive path may consult the remote, but must come back rather than
// wait forever on one that will not answer.
func TestDetectDefaultBranchIsBoundedAgainstAnUnreachableRemote(t *testing.T) {
	dir := newRepoWithRemote(t, "https://10.255.255.1/proofboard/does-not-exist.git")

	started := time.Now()
	done := make(chan struct{})
	go func() { detectDefaultBranch(context.Background(), dir); close(done) }()

	select {
	case <-done:
		if elapsed := time.Since(started); elapsed > remoteDefaultBranchTimeout+5*time.Second {
			t.Fatalf("returned after %v, beyond its own %v bound", elapsed, remoteDefaultBranchTimeout)
		}
	case <-time.After(remoteDefaultBranchTimeout + 15*time.Second):
		t.Fatalf("detectDefaultBranch never returned: the %v bound is not being applied", remoteDefaultBranchTimeout)
	}
}
