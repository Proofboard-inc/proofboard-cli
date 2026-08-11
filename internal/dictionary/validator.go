package dictionary

import "fmt"

func Validate(dictionary Dictionary) error {
	if dictionary.Version == "" {
		return fmt.Errorf("dictionary version is required")
	}
	if len(dictionary.Categories) == 0 {
		return fmt.Errorf("dictionary must define at least one category")
	}
	for category, signals := range dictionary.Categories {
		if category == "" {
			return fmt.Errorf("dictionary category name is required")
		}
		if signals.Impact == "" {
			return fmt.Errorf("dictionary category %q impact is required", category)
		}
	}
	// A real backend response always includes stack signals (see
	// CLI_DICTIONARY_RESPONSE) — rejecting one that doesn't means a
	// truncated/malformed/regressed response can never get installed and
	// silently degrade tech-stack detection down to the tiny built-in
	// fallback table with no visible error anywhere in the chain.
	if len(dictionary.StackSignals) == 0 {
		return fmt.Errorf("dictionary must define stack signals")
	}
	return nil
}
