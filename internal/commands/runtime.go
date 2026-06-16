package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/config"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/state"
)

type runtimeContext struct {
	config      config.Config
	homeDir     string
	workingDir  string
	credentials pbauth.CredentialStore
	state       state.Store
	api         api.Client
}

func loadRuntime(ctx context.Context) (runtimeContext, error) {
	cfg, err := config.Load(ctx)
	if err != nil {
		return runtimeContext{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return runtimeContext{}, fmt.Errorf("detect home directory: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return runtimeContext{}, fmt.Errorf("detect working directory: %w", err)
	}
	return runtimeContext{
		config:      cfg,
		homeDir:     home,
		workingDir:  wd,
		credentials: pbauth.NewCredentialStore(home),
		state:       state.NewStore(home),
		api:         api.NewClient(cfg.APIBaseURL, cfg.LinkPath, cfg.SyncPath),
	}, nil
}

func logPath(home string) string {
	return filepath.Join(home, ".proofboard", "sync.log")
}

func authEmailHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "config", "user.email")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read git config user.email: %w", err)
	}
	email := strings.TrimSpace(string(out))
	if email == "" {
		return "", fmt.Errorf("git config user.email is empty")
	}
	return crypto.NormalizedSHA256(email), nil
}

func triggerMonthlyCareerSummary(ctx context.Context, out io.Writer, runtime runtimeContext) error {
	return triggerMonthlyCareerSummaryWithTime(ctx, out, runtime, time.Now())
}

func triggerMonthlyCareerSummaryWithTime(ctx context.Context, out io.Writer, runtime runtimeContext, now time.Time) error {
	current, err := runtime.state.Load(ctx)
	if err != nil {
		return err
	}
	key, monthName := getReadyCareerSummaryMonth(now)
	if current.MonthlyCareerSummaryShown == nil {
		current.MonthlyCareerSummaryShown = make(map[string]bool)
	}
	if !current.MonthlyCareerSummaryShown[key] {
		_, err := fmt.Fprintf(out, "Proofboard: Your %s career summary is ready. proofboard.io/career-summary\n", monthName)
		if err != nil {
			return err
		}
		current.MonthlyCareerSummaryShown[key] = true
		if err := runtime.state.Save(ctx, current); err != nil {
			return err
		}
	}
	return nil
}

func getReadyCareerSummaryMonth(now time.Time) (string, string) {
	lastFriday := lastFridayOfMonth(now.Year(), now.Month(), now.Location())
	var targetTime time.Time
	if now.After(lastFriday) {
		targetTime = now
	} else {
		targetTime = now.AddDate(0, -1, 0)
	}
	key := targetTime.Format("2006-01")
	monthName := targetTime.Month().String()
	return key, monthName
}

func lastFridayOfMonth(year int, month time.Month, loc *time.Location) time.Time {
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)
	for t.Weekday() != time.Friday {
		t = t.AddDate(0, 0, -1)
	}
	return t
}
