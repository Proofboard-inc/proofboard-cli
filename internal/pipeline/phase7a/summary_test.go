package phase7a

import (
	"strings"
	"testing"
)

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name              string
		primaryCategory   string
		secondaryCategory string
		impactType        string
		scale             string
		commitCount       int
		durationDays      int
		expectedContains  []string
	}{
		{
			name:              "Large scale with secondary category, weeks duration",
			primaryCategory:   "Payments and Billing",
			secondaryCategory: "Authentication and Security",
			impactType:        "feature",
			scale:             "large",
			commitCount:       67,
			durationDays:      98,
			expectedContains: []string{
				"Built and delivered a large-scale Payments and Billing feature with Authentication and Security integration over 14 weeks across 67 commits.",
			},
		},
		{
			name:              "Medium scale, no secondary category, 1 week duration",
			primaryCategory:   "API Design",
			secondaryCategory: "",
			impactType:        "refactor",
			scale:             "medium",
			commitCount:       15,
			durationDays:      7,
			expectedContains: []string{
				"Built and delivered a medium-scale API Design refactor over 1 week across 15 commits.",
			},
		},
		{
			name:              "Small scale, days duration (< 7 days)",
			primaryCategory:   "Frontend Layout",
			secondaryCategory: "",
			impactType:        "bugfix",
			scale:             "small",
			commitCount:       3,
			durationDays:      5,
			expectedContains: []string{
				"Built and delivered a small-scale Frontend Layout bugfix over 5 days across 3 commits.",
			},
		},
		{
			name:              "Zero days duration",
			primaryCategory:   "CI/CD Pipeline",
			secondaryCategory: "",
			impactType:        "chore",
			scale:             "small",
			commitCount:       1,
			durationDays:      0,
			expectedContains: []string{
				"Built and delivered a small-scale CI/CD Pipeline chore over 1 day across 1 commits.",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GenerateSummary(tc.primaryCategory, tc.secondaryCategory, tc.impactType, tc.scale, tc.commitCount, tc.durationDays)
			for _, exp := range tc.expectedContains {
				if !strings.Contains(got, exp) {
					t.Errorf("GenerateSummary(...) = %q; expected to contain %q", got, exp)
				}
			}
		})
	}
}
