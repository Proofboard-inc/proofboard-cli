package commands

import (
	"bytes"
	"strings"
	"testing"
)

// The three milestone subcommands each take a bundle identifier, and this is
// the only place the CLI can tell anyone what those identifiers are. If the
// identifier stops being printed, the commands still exist but become
// impossible to invoke.
func TestReadyMilestonesPrintTheirRunnableCommands(t *testing.T) {
	var out bytes.Buffer
	printMilestonesReady(&out, []struct{ title, bundleID string }{
		{title: "Checkout rework", bundleID: "bundle-abc123"},
	})

	got := out.String()
	for _, want := range []string{
		"Checkout rework",
		"proofboard milestone review bundle-abc123",
		"proofboard milestone publish bundle-abc123",
		"proofboard milestone skip bundle-abc123",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

// Without an identifier there is nothing to run, so the entry is named but no
// uninvokable command is offered.
func TestReadyMilestoneWithoutBundleIDOffersNoCommand(t *testing.T) {
	var out bytes.Buffer
	printMilestonesReady(&out, []struct{ title, bundleID string }{{title: "Unnamed work"}})

	got := out.String()
	if !strings.Contains(got, "Unnamed work") {
		t.Fatalf("expected the milestone to still be named:\n%s", got)
	}
	if strings.Contains(got, "proofboard milestone") {
		t.Fatalf("offered a command with no identifier to pass it:\n%s", got)
	}
}

func TestNoMilestonesPrintsNothing(t *testing.T) {
	var out bytes.Buffer
	printMilestonesReady(&out, nil)
	if out.Len() != 0 {
		t.Fatalf("expected silence when nothing is ready, got %q", out.String())
	}
}
