//go:build darwin

package notifications

import "testing"

func TestResolveTerminalNotifierActivationMapsClickedActionToItsKey(t *testing.T) {
	labels := activationLabels{
		primary: "Review", primaryKey: "review",
		secondary: "Publish", secondaryKey: "publish",
		tertiary: "Skip", tertiaryKey: "ignore",
	}

	cases := []struct {
		name     string
		result   terminalNotifierResult
		wantKind string
		wantOK   bool
	}{
		{
			name:     "primary action clicked",
			result:   terminalNotifierResult{ActivationType: "actionClicked", ActivationValue: "Review"},
			wantKind: "review",
			wantOK:   true,
		},
		{
			name:     "secondary action clicked",
			result:   terminalNotifierResult{ActivationType: "actionClicked", ActivationValue: "Publish"},
			wantKind: "publish",
			wantOK:   true,
		},
		{
			name:     "tertiary action clicked",
			result:   terminalNotifierResult{ActivationType: "actionClicked", ActivationValue: "Skip"},
			wantKind: "ignore",
			wantOK:   true,
		},
		{
			name:   "notification dismissed — no activation",
			result: terminalNotifierResult{ActivationType: "closed"},
			wantOK: false,
		},
		{
			name:   "notification timed out — no activation",
			result: terminalNotifierResult{ActivationType: "timeout"},
			wantOK: false,
		},
		{
			name:   "body clicked without picking a dropdown action",
			result: terminalNotifierResult{ActivationType: "contentsClicked"},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, ok := resolveTerminalNotifierActivation(tc.result, labels)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && kind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tc.wantKind)
			}
		})
	}
}

func TestAppleScriptSafeTextStripsUnsafeCharacters(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text is untouched",
			in:   "Sync Project",
			want: "Sync Project",
		},
		{
			name: "allowed punctuation is kept",
			in:   "Milestone (v1.2) - ready, review_it now.",
			want: "Milestone (v1.2) - ready, review_it now.",
		},
		{
			name: "quotes and backslashes that could break out of the AppleScript string are dropped",
			in:   `evil" & (do shell script "rm -rf ~") & "`,
			want: "evil  (do shell script rm -rf )  ",
		},
		{
			name: "newlines and control characters are dropped",
			in:   "line one\nline two\ttabbed",
			want: "line oneline twotabbed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := appleScriptSafeText(tc.in)
			if got != tc.want {
				t.Fatalf("appleScriptSafeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveTerminalNotifierActivationRespectsDismissKey(t *testing.T) {
	// A "dismiss" key (used by the two-choice link/sync prompts, which have
	// no third action) must never be activated even if somehow matched.
	labels := activationLabels{
		primary: "Sync Project", primaryKey: "sync",
		secondary: "Not Now", secondaryKey: "dismiss",
	}

	kind, ok := resolveTerminalNotifierActivation(
		terminalNotifierResult{ActivationType: "actionClicked", ActivationValue: "Not Now"},
		labels,
	)
	if ok {
		t.Fatalf("expected dismiss key to never activate, got kind=%q ok=%v", kind, ok)
	}
}
