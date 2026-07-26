package auth

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

func TestCredentialStoreRepairsExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not report POSIX permission bits")
	}
	homeDir := t.TempDir()
	store := NewCredentialStore(homeDir)
	directory := filepath.Dir(store.Path())
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create permissive directory: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte(`{"token":"old"}`), 0o644); err != nil {
		t.Fatalf("create permissive credentials file: %v", err)
	}

	if err := store.Save(context.Background(), model.Credentials{
		Token:        "access-token",
		RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat credentials directory: %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("credentials directory mode = %v, want 0700", directoryInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("credentials file mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}

func TestCredentialStoreDeleteRemovesCredentialsAndIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewCredentialStore(t.TempDir())
	if err := store.Save(ctx, model.Credentials{Token: "access-token"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("credential file still exists after Delete: %v", err)
	}
	if err := store.Delete(ctx); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}
