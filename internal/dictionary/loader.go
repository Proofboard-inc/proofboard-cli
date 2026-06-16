package dictionary

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed dictionary.json
var embeddedDictionary embed.FS

func Load(ctx context.Context, reader io.Reader) (Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return Dictionary{}, fmt.Errorf("load dictionary: %w", err)
	}
	var dictionary Dictionary
	if err := json.NewDecoder(reader).Decode(&dictionary); err != nil {
		return Dictionary{}, fmt.Errorf("decode dictionary: %w", err)
	}
	return dictionary, nil
}

func LoadDefault(ctx context.Context) (Dictionary, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		dictPath := filepath.Join(home, ".proofboard", "dictionary.json")
		if file, err := os.Open(dictPath); err == nil {
			defer file.Close()
			if dict, err := Load(ctx, file); err == nil {
				return dict, nil
			}
		}
	}

	file, err := embeddedDictionary.Open("dictionary.json")
	if err != nil {
		return Dictionary{}, fmt.Errorf("open embedded dictionary: %w", err)
	}
	defer file.Close()
	return Load(ctx, file)
}
