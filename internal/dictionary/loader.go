package dictionary

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
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
	file, err := embeddedDictionary.Open("dictionary.json")
	if err != nil {
		return Dictionary{}, fmt.Errorf("open embedded dictionary: %w", err)
	}
	defer file.Close()
	return Load(ctx, file)
}
