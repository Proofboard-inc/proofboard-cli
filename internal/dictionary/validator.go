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
	return nil
}
