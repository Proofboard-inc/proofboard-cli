package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBinaries creates the four Homebrew platform binaries in dir with
// distinct contents and returns their expected sha256 by asset name.
func writeBinaries(t *testing.T, dir string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	for _, name := range homebrewAssets {
		body := []byte("binary for " + name)
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		sums[name] = hex.EncodeToString(sum[:])
	}
	return sums
}

func TestRenderPinsVersionURLsAndChecksums(t *testing.T) {
	dir := t.TempDir()
	sums := writeBinaries(t, dir)

	formula, err := render("v1.16.11", dir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(formula, `version "1.16.11"`) {
		t.Errorf("formula does not pin version 1.16.11 (tag prefix should be dropped):\n%s", formula)
	}
	for _, name := range homebrewAssets {
		url := "https://github.com/Proofboard-inc/proofboard-cli/releases/download/v1.16.11/" + name
		if !strings.Contains(formula, `url "`+url+`"`) {
			t.Errorf("formula is missing url for %s", name)
		}
		if !strings.Contains(formula, `sha256 "`+sums[name]+`"`) {
			t.Errorf("formula is missing the sha256 of %s", name)
		}
	}
	// Each url must be followed by its own checksum, never another platform's.
	for _, name := range homebrewAssets {
		i := strings.Index(formula, "/v1.16.11/"+name+`"`)
		if i < 0 {
			continue // already reported as a missing url above
		}
		rest := formula[i:]
		next := strings.Index(rest, "sha256 \"")
		if next < 0 || !strings.HasPrefix(rest[next+len("sha256 \""):], sums[name]) {
			t.Errorf("the checksum after %s's url is not %s's", name, name)
		}
	}
	// What makes it a working formula, kept from the hand-written one.
	for _, want := range []string{"class Proofboard < Formula", `bin.install Dir["Proofboard-Career-Agent-*"].first => "proofboard"`, "brew upgrade proofboard"} {
		if !strings.Contains(formula, want) {
			t.Errorf("formula lost %q", want)
		}
	}
}

func TestRenderRefusesAMissingBinary(t *testing.T) {
	dir := t.TempDir()
	writeBinaries(t, dir)
	if err := os.Remove(filepath.Join(dir, "Proofboard-Career-Agent-darwin-arm64")); err != nil {
		t.Fatal(err)
	}
	if _, err := render("v1.16.11", dir); err == nil || !strings.Contains(err.Error(), "Proofboard-Career-Agent-darwin-arm64") {
		t.Fatalf("expected an error naming the missing darwin-arm64 binary, got %v", err)
	}
}

func TestRenderRefusesAMalformedTag(t *testing.T) {
	dir := t.TempDir()
	writeBinaries(t, dir)
	for _, tag := range []string{"", "1.16.11", "vnext", "v1.16"} {
		if _, err := render(tag, dir); err == nil {
			t.Errorf("tag %q should be refused", tag)
		}
	}
}
