package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/spf13/cobra"
)

func newUpdateDictionaryCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "update-dictionary",
		Short: "Check for a newer category dictionary",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("update-dictionary: %w", err)
			}
			local, err := dictionary.LoadDefault(ctx)
			if err != nil {
				return err
			}
			releases := api.NewReleaseClient(runtime.config.ReleaseBaseURL)
			url := fmt.Sprintf("%s%s", runtime.config.APIBaseURL, runtime.config.DictionaryPath)
			latest, err := releases.Latest(ctx, url)
			if err != nil {
				return err
			}
			comparison, err := compareDictionaryVersions(latest.Version, local.Version)
			if err != nil {
				return fmt.Errorf("compare dictionary versions: %w", err)
			}
			if latest.Version == "" || comparison <= 0 {
				_, err := fmt.Fprintf(out, "Dictionary is up to date (%s).\n", local.Version)
				return err
			}
			if strings.TrimSpace(latest.URL) == "" {
				return fmt.Errorf(
					"dictionary %s is available but the API did not provide a download URL",
					latest.Version,
				)
			}

			// New version is available!
			dir := filepath.Join(runtime.homeDir, ".proofboard")
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}

			tempPath := filepath.Join(dir, "dictionary.json.tmp")
			tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return fmt.Errorf("create temp dictionary file: %w", err)
			}

			// Download dictionary
			err = releases.Download(ctx, latest.URL, tempFile)
			tempFile.Close() // close before reading/renaming
			if err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("download dictionary: %w", err)
			}

			// Validate
			validFile, err := os.Open(tempPath)
			if err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("open temp dictionary file for validation: %w", err)
			}
			downloadedDict, err := dictionary.Load(ctx, validFile)
			validFile.Close()
			if err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("load downloaded dictionary: %w", err)
			}

			if err := dictionary.Validate(downloadedDict); err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("validate downloaded dictionary: %w", err)
			}

			// Atomically rename
			targetPath := filepath.Join(dir, "dictionary.json")
			if err := os.Rename(tempPath, targetPath); err != nil {
				return fmt.Errorf("replace dictionary file: %w", err)
			}

			// Update state
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}
			current.DictionaryVersion = latest.Version
			if err := runtime.state.Save(ctx, current); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			_, err = fmt.Fprintf(out, "Dictionary updated successfully to version %s.\n", latest.Version)
			return err
		},
	}
}

func compareDictionaryVersions(left, right string) (int, error) {
	parse := func(value string) ([]int, error) {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		if value == "" {
			return nil, nil
		}
		parts := strings.Split(value, ".")
		numbers := make([]int, len(parts))
		for index, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil || number < 0 {
				return nil, fmt.Errorf("invalid version %q", value)
			}
			numbers[index] = number
		}
		return numbers, nil
	}
	leftParts, err := parse(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parse(right)
	if err != nil {
		return 0, err
	}
	width := len(leftParts)
	if len(rightParts) > width {
		width = len(rightParts)
	}
	for index := 0; index < width; index++ {
		var leftNumber, rightNumber int
		if index < len(leftParts) {
			leftNumber = leftParts[index]
		}
		if index < len(rightParts) {
			rightNumber = rightParts[index]
		}
		if leftNumber < rightNumber {
			return -1, nil
		}
		if leftNumber > rightNumber {
			return 1, nil
		}
	}
	return 0, nil
}
