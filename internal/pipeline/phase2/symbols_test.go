package phase2

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestExtractSymbols(t *testing.T) {
	lower := func(s string) []byte { return []byte(strings.ToLower(s)) }

	cases := []struct {
		name      string
		body      string
		filePaths []string
		want      []string
	}{
		{
			name:      "go function declaration",
			body:      "adds chargeCard() and imports stripe",
			filePaths: []string{"internal/payments/charge.go"},
			want:      []string{"chargecard"},
		},
		{
			name:      "python function and class",
			body:      "def process_refund(): pass\nclass RefundHandler: pass",
			filePaths: []string{"billing/refund.py"},
			want:      []string{"process_refund", "refundhandler"},
		},
		{
			name:      "js/ts function, const, import",
			body:      "function handleWebhook() {}\nconst stripeClient = init()\nimport stripe from 'stripe'",
			filePaths: []string{"src/webhooks/handler.ts"},
			want:      []string{"handlewebhook", "stripeclient", "stripe", "init"},
		},
		{
			name:      "universal identifier() pattern applies regardless of extension",
			body:      "def process_refund(): pass",
			filePaths: []string{"README.md"},
			want:      []string{"process_refund"},
		},
		{
			name:      "no parens and no matching extension yields no symbols",
			body:      "improves docs clarity",
			filePaths: []string{"README.md"},
			want:      nil,
		},
		{
			name:      "prose-only body yields no false positives",
			body:      "just a small copy tweak, nothing structural here",
			filePaths: []string{"internal/payments/charge.go"},
			want:      nil,
		},
		{
			name:      "empty body yields no symbols",
			body:      "",
			filePaths: []string{"internal/payments/charge.go"},
			want:      nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractSymbols(lower(tc.body), tc.filePaths)
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ExtractSymbols() = %v, want %v", got, want)
			}
		})
	}
}
