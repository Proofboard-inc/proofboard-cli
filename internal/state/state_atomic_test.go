package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

// stateOfSize returns a state whose encoded length grows with n, so writers
// racing each other produce documents of different lengths — the condition
// that left a stray byte at the end of a real state.json.
func stateOfSize(n int) model.State {
	st := Default()
	for i := 0; i < n; i++ {
		st.SuppressedWorkspaces = append(st.SuppressedWorkspaces, crypto.SHA256(fmt.Sprint(i)))
	}
	return st
}

// A developer's state.json was found holding a complete, valid document
// followed by one extra "}", and from then on every command — sync, unlink,
// even a fresh login followed by sync — failed with
//
//	decode state: invalid character '}' after top-level value
//
// os.WriteFile truncates, so one writer cannot do that. Two can: both open
// with O_TRUNC, the longer document is written, then the shorter one is
// written over it from offset zero, and the longer one's final byte survives.
// state.json has many writers that run at the same time — the shell cd hook,
// the git hook sync, the background agent and the startup checks — and none of
// them coordinate.
func TestSaveNeverLeavesAPartialDocumentUnderConcurrentWriters(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx := context.Background()
	if err := store.Save(ctx, stateOfSize(40)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Readers run WHILE writers write. Checking only the file left at the end
	// proves nothing — the first version of this test did that and passed 20
	// times out of 20 against the broken code — because the damage happens in
	// the window between a writer truncating the file and filling it again,
	// and any command that reads in that window sees a partial document.
	stop := make(chan struct{})
	var bad sync.Map
	var readers sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				data, err := os.ReadFile(store.Path())
				if err == nil && !json.Valid(data) {
					tail := data
					if len(tail) > 24 {
						tail = tail[len(tail)-24:]
					}
					bad.Store(len(data), string(tail))
				}
			}
		}()
	}

	var writers sync.WaitGroup
	for writer := 0; writer < 8; writer++ {
		writers.Add(1)
		go func(writer int) {
			defer writers.Done()
			for i := 0; i < 150; i++ {
				// Sizes differ by writer and iteration, so truncating writes
				// of unequal length keep landing on top of each other.
				_ = store.Save(ctx, stateOfSize(20+(writer*7+i)%31))
			}
		}(writer)
	}
	writers.Wait()
	close(stop)
	readers.Wait()

	var observed []string
	bad.Range(func(size, tail any) bool {
		observed = append(observed, fmt.Sprintf("%d bytes ending %q", size, tail))
		return len(observed) < 3
	})
	if len(observed) > 0 {
		t.Fatalf("a reader saw a state.json that was not one complete JSON document "+
			"while saves were in progress: %v", observed)
	}

	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("concurrent saves left state.json that is not a single JSON document")
	}
}

// Machines already carrying a corrupted file have to recover without anyone
// editing JSON by hand. The document before the stray bytes is complete and
// was the last full write, so it is the state to keep.
func TestLoadRecoversAStateFileWithTrailingBytes(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx := context.Background()
	want := stateOfSize(3)
	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("seed: %v", err)
	}
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Exactly what was found on disk: a valid document and one extra brace.
	if err := os.WriteFile(store.Path(), append(data, '}'), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load must recover a document with trailing bytes, got: %v", err)
	}
	if len(got.SuppressedWorkspaces) != len(want.SuppressedWorkspaces) {
		t.Fatalf("recovered state lost data: %d suppressed workspaces, want %d",
			len(got.SuppressedWorkspaces), len(want.SuppressedWorkspaces))
	}

	// Recovering in memory only would leave every other command reading the
	// broken file, so the repair has to reach the disk.
	repaired, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read repaired: %v", err)
	}
	if !json.Valid(repaired) {
		t.Fatalf("state.json was not repaired on disk; it still ends %q", repaired[len(repaired)-5:])
	}
}

// A file that holds no decodable document at all must not brick the CLI
// either. It is set aside rather than deleted, so whatever it held can still
// be inspected, and the CLI carries on from defaults.
func TestLoadSetsAsideAnUndecodableStateFile(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	garbage := []byte("{\"linkedRepos\": {\"unterminated")
	if err := os.WriteFile(store.Path(), garbage, 0o600); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load must not fail on an undecodable file, got: %v", err)
	}
	if got.LinkedRepos == nil {
		t.Fatal("expected default state after setting the file aside")
	}

	entries, err := os.ReadDir(filepath.Dir(store.Path()))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var preserved []byte
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "state.json.corrupt-") {
			preserved, _ = os.ReadFile(filepath.Join(filepath.Dir(store.Path()), entry.Name()))
		}
	}
	if !bytes.Equal(preserved, garbage) {
		t.Fatalf("the undecodable file was not preserved as state.json.corrupt-*; found %q", preserved)
	}
}
