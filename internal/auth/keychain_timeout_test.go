package auth

import (
	"context"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

// blockingSecretStore stands in for an OS keychain that never answers.
type blockingSecretStore struct{ release chan struct{} }

func (b *blockingSecretStore) Get(service, account string) (string, error) {
	<-b.release
	return "", nil
}

func (b *blockingSecretStore) Set(service, account, value string) error {
	<-b.release
	return nil
}

func (b *blockingSecretStore) Delete(service, account string) error {
	<-b.release
	return nil
}

// The OS keychain is a foreign call that can block indefinitely, and on macOS
// it does: a runner with no unlocked login keychain leaves the Security
// framework waiting inside the syscall, which is where the auth test suite hung
// until the package timeout killed it (go-keyring/keyring_darwin.go:99).
//
// On a developer's Mac the same thing means `proofboard auth` freezes after
// signing in — the credentials are in hand and the CLI never comes back. There
// is a perfectly good file fallback a few lines below; nothing reaches it while
// the keychain call is still waiting.
func TestSaveFallsBackWhenTheKeychainDoesNotAnswer(t *testing.T) {
	blocking := &blockingSecretStore{release: make(chan struct{})}
	t.Cleanup(func() { close(blocking.release) })

	store := CredentialStore{homeDir: t.TempDir(), secretStore: blocking}

	original := keychainCallTimeout
	keychainCallTimeout = 200 * time.Millisecond
	t.Cleanup(func() { keychainCallTimeout = original })

	done := make(chan error, 1)
	go func() {
		done <- store.Save(context.Background(), model.Credentials{Token: "access-token"})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Save() error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Save blocked on an unresponsive keychain: the OS call has no " +
			"bound, so the file fallback below it is never reached")
	}

	// The point of falling back is that the credentials are actually stored.
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Token != "access-token" {
		t.Fatalf("credentials were not persisted: %+v", loaded)
	}
}
