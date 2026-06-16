package phase7a

import (
	"fmt"
)

// GenerateSummary generates a one-sentence professional summary for a cluster.
func GenerateSummary(primaryCategory, secondaryCategory, impactType, scale string, commitCount, durationDays int) string {
	var durationStr string
	weeks := durationDays / 7
	if weeks >= 1 {
		if weeks == 1 {
			durationStr = "1 week"
		} else {
			durationStr = fmt.Sprintf("%d weeks", weeks)
		}
	} else {
		if durationDays <= 0 {
			durationStr = "1 day"
		} else if durationDays == 1 {
			durationStr = "1 day"
		} else {
			durationStr = fmt.Sprintf("%d days", durationDays)
		}
	}

	if secondaryCategory != "" {
		return fmt.Sprintf("Built and delivered a %s-scale %s %s with %s integration over %s across %d commits.",
			scale, primaryCategory, impactType, secondaryCategory, durationStr, commitCount)
	}
	return fmt.Sprintf("Built and delivered a %s-scale %s %s over %s across %d commits.",
		scale, primaryCategory, impactType, durationStr, commitCount)
}
