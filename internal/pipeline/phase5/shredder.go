package phase5

import (
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

func Shred(commits []model.RawCommit, signals []model.CommitSignal) []model.SafeCommit {
	for i := range commits {
		crypto.ZeroBytes(commits[i].Subject)
		commits[i].Subject = nil
		commits[i].FilePaths = crypto.DropStrings(commits[i].FilePaths)
		commits[i].AuthorEmail = ""
		commits[i].Repository = ""
		commits[i].Organization = ""
	}

	safe := make([]model.SafeCommit, 0, len(signals))
	for _, signal := range signals {
		safe = append(safe, model.SafeCommit{
			SHA:             signal.SHA,
			TimestampUnix:   signal.Timestamp.Unix(),
			Additions:       signal.Additions,
			Deletions:       signal.Deletions,
			FilesChanged:    signal.FilesChanged,
			Category:        signal.PrimaryCategory,
			ImpactType:      signal.ImpactType,
			NoiseScore:      signal.NoiseScore,
			AuthorEmailHash: signal.AuthorEmailHash,
		})
	}
	return safe
}
